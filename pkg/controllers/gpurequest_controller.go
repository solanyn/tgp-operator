// Package controllers implements Kubernetes controllers for the TGP operator
package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/metrics"
	"github.com/solanyn/tgp-operator/pkg/pricing"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

const (
	FinalizerName                 = "tgp.io/finalizer"
	maxLifetimeTerminationMessage = "Instance terminated due to maxLifetime"
)

// Requeue intervals for different scenarios
const (
	ProvisioningRequeue = 15 * time.Second // Quick requeue for new instances
	RunningRequeue      = 2 * time.Minute  // Slower requeue for stable instances
	FailedRequeue       = 5 * time.Minute  // Backoff for failures
	TerminatingRequeue  = 10 * time.Second // Quick requeue for termination
)

// GPURequestReconciler reconciles a GPURequest object
type GPURequestReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Providers    map[string]providers.ProviderClient
	PricingCache *pricing.Cache
	Metrics      *metrics.Metrics
}

// +kubebuilder:rbac:groups=tgp.io,resources=gpurequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tgp.io,resources=gpurequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tgp.io,resources=gpurequests/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GPURequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gpurequest", req.NamespacedName)
	start := time.Now()

	// Fetch the GPURequest instance
	var gpuRequest tgpv1.GPURequest
	if err := r.Client.Get(ctx, req.NamespacedName, &gpuRequest); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch GPURequest")
		return ctrl.Result{}, err
	}

	// Record metrics for this reconciliation
	provider := gpuRequest.Status.SelectedProvider
	if provider == "" {
		provider = gpuRequest.Spec.Provider
	}
	if provider == "" {
		provider = "unknown"
	}

	defer func() {
		if r.Metrics != nil {
			duration := time.Since(start).Seconds()
			r.Metrics.RecordGPURequestDuration(provider, gpuRequest.Spec.GPUType, string(gpuRequest.Status.Phase), duration)
		}
	}()

	// Handle deletion
	if gpuRequest.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &gpuRequest, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&gpuRequest, FinalizerName) {
		controllerutil.AddFinalizer(&gpuRequest, FinalizerName)
		if err := r.Update(ctx, &gpuRequest); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle reconciliation based on current phase
	switch gpuRequest.Status.Phase {
	case "":
		return r.handlePending(ctx, &gpuRequest, log)
	case tgpv1.GPURequestPhasePending:
		return r.handlePending(ctx, &gpuRequest, log)
	case tgpv1.GPURequestPhaseProvisioning:
		return r.handleProvisioning(ctx, &gpuRequest, log)
	case tgpv1.GPURequestPhaseBooting, tgpv1.GPURequestPhaseJoining:
		return r.handleProvisioning(ctx, &gpuRequest, log)
	case tgpv1.GPURequestPhaseReady:
		return r.handleRunning(ctx, &gpuRequest, log)
	case tgpv1.GPURequestPhaseFailed:
		return r.handleFailed(ctx, &gpuRequest, log)
	case tgpv1.GPURequestPhaseTerminating:
		return ctrl.Result{RequeueAfter: TerminatingRequeue}, nil
	default:
		log.Info("unknown phase", "phase", gpuRequest.Status.Phase)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
}

func (r *GPURequestReconciler) handlePending(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling pending GPURequest")

	// Record GPU request metric
	if r.Metrics != nil {
		provider := gpuRequest.Spec.Provider
		if provider == "" {
			provider = "auto-select"
		}
		r.Metrics.RecordGPURequest(provider, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region, "pending")
	}

	// Update status to provisioning
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseProvisioning
	gpuRequest.Status.Message = "Selecting provider and provisioning GPU instance"
	r.updateCondition(gpuRequest, "ProvisioningStarted", metav1.ConditionTrue, "ProvisioningInitiated", "GPU instance provisioning initiated")

	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status to provisioning")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *GPURequestReconciler) handleProvisioning(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling provisioning GPURequest")

	// Select provider
	provider, selectedProvider, err := r.selectProvider(ctx, gpuRequest, log)
	if err != nil {
		return r.handleProviderSelectionError(ctx, gpuRequest, err, log)
	}

	// Cache the provider selection
	gpuRequest.Status.SelectedProvider = selectedProvider

	// Launch instance if not already launched
	if gpuRequest.Status.InstanceID == "" {
		return r.launchInstance(ctx, gpuRequest, provider, selectedProvider, log)
	}

	// Handle existing instance - check for termination or status
	return r.handleExistingInstance(ctx, gpuRequest, provider, selectedProvider, log)
}

func (r *GPURequestReconciler) handleRunning(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling running GPURequest")

	// Check if instance should be terminated due to maxLifetime
	if gpuRequest.IsTerminationDue() {
		log.Info("instance has reached maxLifetime, terminating", "instanceId", gpuRequest.Status.InstanceID)
		provider, exists := r.Providers[gpuRequest.Status.SelectedProvider]
		if exists {
			if err := provider.TerminateInstance(ctx, gpuRequest.Status.InstanceID); err != nil {
				log.Error(err, "failed to terminate instance due to maxLifetime")
			}
		}
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseTerminating
		gpuRequest.Status.Message = maxLifetimeTerminationMessage
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status to terminating")
		}
		return ctrl.Result{RequeueAfter: TerminatingRequeue}, nil
	}

	// Perform health check on the running instance
	if err := r.performHealthCheck(ctx, gpuRequest, log); err != nil {
		log.Error(err, "health check failed")
		r.updateCondition(gpuRequest, "InstanceHealthy", metav1.ConditionFalse, "HealthCheckFailed", err.Error())
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update health check status")
		}
		return ctrl.Result{RequeueAfter: RunningRequeue}, nil
	}

	// Check for idle timeout by monitoring pod activity
	if gpuRequest.Spec.IdleTimeout != nil {
		if idle, reason, err := r.checkIdleTimeout(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to check idle timeout")
		} else if idle {
			log.Info("instance is idle beyond timeout, terminating", "reason", reason)
			r.updateCondition(gpuRequest, "IdleTimeout", metav1.ConditionTrue, "IdleTimeoutReached", reason)

			// Record idle timeout metric
			if r.Metrics != nil {
				r.Metrics.RecordIdleTimeout(gpuRequest.Status.SelectedProvider, gpuRequest.Spec.GPUType)
			}

			provider, exists := r.Providers[gpuRequest.Status.SelectedProvider]
			if exists {
				if err := provider.TerminateInstance(ctx, gpuRequest.Status.InstanceID); err != nil {
					log.Error(err, "failed to terminate idle instance")
				}
			}
			gpuRequest.Status.Phase = tgpv1.GPURequestPhaseTerminating
			gpuRequest.Status.Message = fmt.Sprintf("Instance terminated due to idle timeout: %s", reason)
			if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
				log.Error(err, "failed to update status to terminating due to idle")
			}
			return ctrl.Result{RequeueAfter: TerminatingRequeue}, nil
		}
	}

	// Update heartbeat and healthy condition
	now := metav1.Time{Time: time.Now()}
	gpuRequest.Status.LastHeartbeat = &now
	r.updateCondition(gpuRequest, "InstanceHealthy", metav1.ConditionTrue, "HealthCheckPassed", "Instance is healthy and responsive")

	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update heartbeat")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: RunningRequeue}, nil
}

// updateStatusWithRetry updates the GPURequest status with retry logic to handle resource conflicts
func (r *GPURequestReconciler) updateStatusWithRetry(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the latest version of the resource
		latest := &tgpv1.GPURequest{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(gpuRequest), latest); err != nil {
			return err
		}

		// Update the status fields on the latest version
		latest.Status = gpuRequest.Status

		// Attempt to update
		return r.Status().Update(ctx, latest)
	})
}

func (r *GPURequestReconciler) handleFailed(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling failed GPURequest")

	// TODO: Implement retry logic or cleanup
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

// performHealthCheck verifies the instance is healthy by checking provider status
func (r *GPURequestReconciler) performHealthCheck(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) error {
	if gpuRequest.Status.InstanceID == "" || gpuRequest.Status.SelectedProvider == "" {
		return fmt.Errorf("missing instance information for health check")
	}

	provider, exists := r.Providers[gpuRequest.Status.SelectedProvider]
	if !exists {
		return fmt.Errorf("provider %s not available", gpuRequest.Status.SelectedProvider)
	}

	status, err := provider.GetInstanceStatus(ctx, gpuRequest.Status.InstanceID)

	// Record health check metrics
	if r.Metrics != nil {
		if err != nil {
			r.Metrics.RecordHealthCheck(gpuRequest.Status.SelectedProvider, "error")
		} else {
			r.Metrics.RecordHealthCheck(gpuRequest.Status.SelectedProvider, "success")
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get instance status: %w", err)
	}

	// Update instance details from provider status
	if status.PublicIP != "" && status.PublicIP != gpuRequest.Status.PublicIP {
		gpuRequest.Status.PublicIP = status.PublicIP
	}
	if status.PrivateIP != "" && status.PrivateIP != gpuRequest.Status.PrivateIP {
		gpuRequest.Status.PrivateIP = status.PrivateIP
	}

	// Check if instance is still running
	switch status.State {
	case providers.InstanceStateRunning:
		// Instance is healthy
		return nil
	case providers.InstanceStateFailed, providers.InstanceStateTerminated:
		return fmt.Errorf("instance is in failed state: %s - %s", status.State, status.Message)
	case providers.InstanceStateTerminating:
		return fmt.Errorf("instance is terminating: %s", status.Message)
	case providers.InstanceStatePending:
		return fmt.Errorf("instance is still starting: %s", status.State)
	case providers.InstanceStateUnknown:
		return fmt.Errorf("instance in unknown state: %s", status.State)
	default:
		return fmt.Errorf("instance in unexpected state: %s", status.State)
	}
}

// updateCondition updates or adds a condition to the GPURequest status
func (r *GPURequestReconciler) updateCondition(gpuRequest *tgpv1.GPURequest, conditionType string,
	status metav1.ConditionStatus, reason, message string,
) {
	now := metav1.Time{Time: time.Now()}

	// Find existing condition
	for i, condition := range gpuRequest.Status.Conditions {
		if condition.Type == conditionType {
			// Update existing condition if status changed
			if condition.Status != status {
				gpuRequest.Status.Conditions[i].Status = status
				gpuRequest.Status.Conditions[i].LastTransitionTime = now
			}
			gpuRequest.Status.Conditions[i].Reason = reason
			gpuRequest.Status.Conditions[i].Message = message
			gpuRequest.Status.Conditions[i].ObservedGeneration = gpuRequest.Generation
			return
		}
	}

	// Add new condition
	newCondition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: gpuRequest.Generation,
	}
	gpuRequest.Status.Conditions = append(gpuRequest.Status.Conditions, newCondition)
}

// checkIdleTimeout determines if an instance has been idle beyond the configured timeout
func (r *GPURequestReconciler) checkIdleTimeout(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (bool, string, error) {
	idleTimeout := gpuRequest.Spec.IdleTimeout.Duration

	// If node hasn't been provisioned yet, not considered idle
	if gpuRequest.Status.ProvisionedAt == nil {
		return false, "", nil
	}

	// Check if enough time has passed since provisioning for idle timeout to be relevant
	timeSinceProvisioned := time.Since(gpuRequest.Status.ProvisionedAt.Time)
	if timeSinceProvisioned < idleTimeout {
		return false, "", nil
	}

	// Check for active pods on this node
	if gpuRequest.Status.NodeName != "" {
		activePods, err := r.getActivePodsOnNode(ctx, gpuRequest.Status.NodeName)
		if err != nil {
			return false, "", fmt.Errorf("failed to get pods on node: %w", err)
		}

		// If there are active pods, not idle
		if activePods > 0 {
			log.V(1).Info("node has active pods, not idle", "activePods", activePods, "nodeName", gpuRequest.Status.NodeName)
			return false, "", nil
		}
	}

	// Determine how long the instance has been idle
	var idleSince time.Time

	// If we have a last heartbeat with activity, use that
	if gpuRequest.Status.LastHeartbeat != nil {
		idleSince = gpuRequest.Status.LastHeartbeat.Time
	} else {
		// Fall back to provisioned time
		idleSince = gpuRequest.Status.ProvisionedAt.Time
	}

	idleDuration := time.Since(idleSince)
	if idleDuration >= idleTimeout {
		reason := fmt.Sprintf("No pod activity for %v (timeout: %v)", idleDuration.Round(time.Minute), idleTimeout)
		return true, reason, nil
	}

	return false, "", nil
}

// getActivePodsOnNode returns the number of non-terminal pods scheduled on the given node
func (r *GPURequestReconciler) getActivePodsOnNode(ctx context.Context, nodeName string) (int, error) {
	podList := &corev1.PodList{}

	// List pods on the specific node
	err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": nodeName})
	if err != nil {
		return 0, err
	}

	activePods := 0
	for _, pod := range podList.Items {
		// Count pods that are not in terminal states
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			activePods++
		}
	}

	return activePods, nil
}

func (r *GPURequestReconciler) handleDeletion(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling GPURequest deletion")

	if gpuRequest.Status.InstanceID != "" {
		provider, exists := r.Providers[gpuRequest.Spec.Provider]
		if exists {
			log.Info("terminating instance", "instanceID", gpuRequest.Status.InstanceID)
			if err := provider.TerminateInstance(ctx, gpuRequest.Status.InstanceID); err != nil {
				log.Error(err, "failed to terminate instance")
				return ctrl.Result{RequeueAfter: time.Second * 30}, nil
			}
			log.Info("instance terminated successfully")
		} else {
			log.Info("provider not available for cleanup, removing finalizer anyway", "provider", gpuRequest.Spec.Provider)
		}
	}

	controllerutil.RemoveFinalizer(gpuRequest, FinalizerName)
	if err := r.Update(ctx, gpuRequest); err != nil {
		log.Error(err, "failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// selectProvider handles provider selection logic with caching and pricing optimization
func (r *GPURequestReconciler) selectProvider(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	log logr.Logger,
) (providers.ProviderClient, string, error) {
	// Use cached provider selection if available
	if gpuRequest.Status.SelectedProvider != "" {
		if provider, exists := r.Providers[gpuRequest.Status.SelectedProvider]; exists {
			log.Info("using cached provider selection", "provider", gpuRequest.Status.SelectedProvider)
			return provider, gpuRequest.Status.SelectedProvider, nil
		}
		log.Info("cached provider no longer available, reselecting", "cached", gpuRequest.Status.SelectedProvider)
		gpuRequest.Status.SelectedProvider = ""
	}

	// Select provider based on spec or pricing
	if gpuRequest.Spec.Provider != "" {
		return r.selectSpecificProvider(gpuRequest.Spec.Provider)
	}

	return r.selectBestPriceProvider(ctx, gpuRequest, log)
}

// selectSpecificProvider validates and returns a specific provider
func (r *GPURequestReconciler) selectSpecificProvider(providerName string) (providers.ProviderClient, string, error) {
	provider, exists := r.Providers[providerName]
	if !exists {
		return nil, "", fmt.Errorf("provider %s not supported", providerName)
	}
	return provider, providerName, nil
}

// selectBestPriceProvider selects provider based on pricing optimization
func (r *GPURequestReconciler) selectBestPriceProvider(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	log logr.Logger,
) (providers.ProviderClient, string, error) {
	if r.PricingCache == nil {
		return r.selectFirstAvailableProvider()
	}

	log.Info("selecting best price provider", "gpuType", gpuRequest.Spec.GPUType, "region", gpuRequest.Spec.Region)
	bestPrice, err := r.PricingCache.GetBestPrice(ctx, r.Providers, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region)
	if err != nil {
		log.Error(err, "failed to get best price, using first available provider")
		return r.selectFirstAvailableProvider()
	}

	// Find provider matching best price
	for name, p := range r.Providers {
		pricing, _ := p.GetNormalizedPricing(ctx, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region)
		if pricing != nil && pricing.PricePerHour == bestPrice.PricePerHour {
			log.Info("selected provider based on pricing", "provider", name, "price", bestPrice.PricePerHour)
			return p, name, nil
		}
	}

	// Fallback to first available
	return r.selectFirstAvailableProvider()
}

// selectFirstAvailableProvider returns the first available provider as fallback
func (r *GPURequestReconciler) selectFirstAvailableProvider() (providers.ProviderClient, string, error) {
	for name, provider := range r.Providers {
		return provider, name, nil
	}
	return nil, "", fmt.Errorf("no providers available")
}

// handleProviderSelectionError handles errors during provider selection
func (r *GPURequestReconciler) handleProviderSelectionError(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	err error,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Error(err, "provider selection failed")
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
	gpuRequest.Status.Message = fmt.Sprintf("Provider selection failed: %v", err)
	if updateErr := r.updateStatusWithRetry(ctx, gpuRequest, log); updateErr != nil {
		log.Error(updateErr, "failed to update status to failed")
	}
	return ctrl.Result{}, nil
}

// launchInstance handles the launch of a new GPU instance
func (r *GPURequestReconciler) launchInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	provider providers.ProviderClient,
	selectedProvider string,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Info("launching new instance", "provider", selectedProvider)

	// Check GPU availability
	if available, result, err := r.checkGPUAvailability(ctx, gpuRequest, provider, log); !available {
		return result, err
	}

	// Create and execute launch request
	instance, err := r.executeLaunchRequest(ctx, gpuRequest, provider, log)
	if err != nil {
		return r.handleLaunchFailure(ctx, gpuRequest, err, log)
	}

	// Update status with launch results
	if err := r.updateLaunchStatus(ctx, gpuRequest, instance, provider, selectedProvider, log); err != nil {
		log.Error(err, "failed to update status after launch")
		return ctrl.Result{}, err
	}

	log.Info("instance launched successfully", "instanceID", instance.ID)
	return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
}

// handleExistingInstance handles status checking and lifecycle management for existing instances
func (r *GPURequestReconciler) handleExistingInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	provider providers.ProviderClient,
	selectedProvider string,
	log logr.Logger,
) (ctrl.Result, error) {
	// Check if instance should be terminated due to maxLifetime
	if gpuRequest.IsTerminationDue() {
		log.Info("instance has reached maxLifetime, terminating", "instanceId", gpuRequest.Status.InstanceID)
		if err := provider.TerminateInstance(ctx, gpuRequest.Status.InstanceID); err != nil {
			log.Error(err, "failed to terminate instance due to maxLifetime")
		}
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseTerminating
		gpuRequest.Status.Message = maxLifetimeTerminationMessage
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status to terminating")
		}
		return ctrl.Result{RequeueAfter: TerminatingRequeue}, nil
	}

	// Get current instance status
	status, err := provider.GetInstanceStatus(ctx, gpuRequest.Status.InstanceID)
	if err != nil {
		log.Error(err, "failed to get instance status")
		return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
	}

	return r.processInstanceStatus(ctx, gpuRequest, selectedProvider, status, log)
}

// processInstanceStatus processes the instance status and updates the GPURequest accordingly
func (r *GPURequestReconciler) processInstanceStatus(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	selectedProvider string,
	status *providers.InstanceStatus,
	log logr.Logger,
) (ctrl.Result, error) {
	switch status.State {
	case providers.InstanceStateRunning:
		return r.handleRunningInstance(ctx, gpuRequest, selectedProvider, log)
	case providers.InstanceStateFailed:
		return r.handleFailedInstance(ctx, gpuRequest, status, log)
	case providers.InstanceStatePending:
		return r.handlePendingInstance(ctx, gpuRequest, status, log)
	case providers.InstanceStateTerminating, providers.InstanceStateTerminated:
		return r.handleTerminatedInstance(ctx, gpuRequest, status, log)
	case providers.InstanceStateUnknown:
		return r.handleUnknownInstance(ctx, gpuRequest, status, log)
	default:
		return r.handleDefaultInstance(ctx, gpuRequest, status, log)
	}
}

// handleRunningInstance updates status for running instances
func (r *GPURequestReconciler) handleRunningInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	selectedProvider string,
	log logr.Logger,
) (ctrl.Result, error) {
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseReady
	gpuRequest.Status.Message = "GPU instance is ready"
	gpuRequest.Status.NodeName = fmt.Sprintf("gpu-node-%s", gpuRequest.Name)

	// Record active instance metric
	if r.Metrics != nil {
		r.Metrics.SetInstanceActive(selectedProvider, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region, 1)
		r.Metrics.RecordGPURequest(selectedProvider, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region, "ready")
	}

	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status to ready")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: RunningRequeue}, nil
}

// handleFailedInstance updates status for failed instances
func (r *GPURequestReconciler) handleFailedInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	status *providers.InstanceStatus,
	log logr.Logger,
) (ctrl.Result, error) {
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
	gpuRequest.Status.Message = fmt.Sprintf("Instance failed: %s", status.Message)
	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status to failed")
	}
	return ctrl.Result{RequeueAfter: FailedRequeue}, nil
}

// handlePendingInstance updates status for pending instances
func (r *GPURequestReconciler) handlePendingInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	status *providers.InstanceStatus,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Info("waiting for instance to be ready", "state", status.State)
	gpuRequest.Status.Message = fmt.Sprintf("Instance state: %s", status.State)
	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status message")
	}
	return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
}

// handleTerminatedInstance updates status for terminated instances
func (r *GPURequestReconciler) handleTerminatedInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	status *providers.InstanceStatus,
	log logr.Logger,
) (ctrl.Result, error) {
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
	gpuRequest.Status.Message = fmt.Sprintf("Instance terminated: %s", status.Message)
	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status to failed")
	}
	return ctrl.Result{RequeueAfter: FailedRequeue}, nil
}

// handleUnknownInstance updates status for instances in unknown state
func (r *GPURequestReconciler) handleUnknownInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	status *providers.InstanceStatus,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Info("instance in unknown state", "state", status.State)
	gpuRequest.Status.Message = fmt.Sprintf("Instance state: %s", status.State)
	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status message")
	}
	return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
}

// handleDefaultInstance updates status for instances in unhandled states
func (r *GPURequestReconciler) handleDefaultInstance(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	status *providers.InstanceStatus,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Info("waiting for instance to be ready", "state", status.State)
	gpuRequest.Status.Message = fmt.Sprintf("Instance state: %s", status.State)
	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status message")
	}
	return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
}

// checkGPUAvailability verifies GPU availability before launching
func (r *GPURequestReconciler) checkGPUAvailability(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	provider providers.ProviderClient,
	log logr.Logger,
) (bool, ctrl.Result, error) {
	filters := &providers.GPUFilters{
		GPUType: gpuRequest.Spec.GPUType,
		Region:  gpuRequest.Spec.Region,
	}

	if gpuRequest.Spec.MaxHourlyPrice != nil && *gpuRequest.Spec.MaxHourlyPrice != "" {
		if maxPrice, err := strconv.ParseFloat(*gpuRequest.Spec.MaxHourlyPrice, 64); err == nil {
			filters.MaxPrice = maxPrice
		}
	}

	gpus, err := provider.ListAvailableGPUs(ctx, filters)
	if err != nil {
		log.Error(err, "failed to list available GPUs")
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
		gpuRequest.Status.Message = fmt.Sprintf("Failed to list GPUs: %v", err)
		if updateErr := r.updateStatusWithRetry(ctx, gpuRequest, log); updateErr != nil {
			log.Error(updateErr, "failed to update status to failed")
		}
		return false, ctrl.Result{RequeueAfter: FailedRequeue}, nil
	}

	if len(gpus) == 0 {
		log.Info("no GPUs available, retrying later")
		gpuRequest.Status.Message = "No GPUs available, waiting for capacity"
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status message")
		}
		return false, ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
	}

	return true, ctrl.Result{}, nil
}

// executeLaunchRequest creates and executes the launch request
func (r *GPURequestReconciler) executeLaunchRequest(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	provider providers.ProviderClient,
	log logr.Logger,
) (*providers.GPUInstance, error) {
	// Resolve any secret references in WireGuardConfig
	resolvedWireGuardConfig, err := gpuRequest.Spec.TalosConfig.WireGuardConfig.Resolve(ctx, r.Client, gpuRequest.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve WireGuard config secrets: %w", err)
	}

	// Create resolved TalosConfig
	resolvedTalosConfig := gpuRequest.Spec.TalosConfig
	resolvedTalosConfig.WireGuardConfig = *resolvedWireGuardConfig

	launchReq := &providers.LaunchRequest{
		GPUType:      gpuRequest.Spec.GPUType,
		Region:       gpuRequest.Spec.Region,
		TalosConfig:  &resolvedTalosConfig,
		SpotInstance: gpuRequest.Spec.Spot,
	}

	if gpuRequest.Spec.MaxHourlyPrice != nil && *gpuRequest.Spec.MaxHourlyPrice != "" {
		if maxPrice, err := strconv.ParseFloat(*gpuRequest.Spec.MaxHourlyPrice, 64); err == nil {
			launchReq.MaxPrice = maxPrice
		}
	}

	return provider.LaunchInstance(ctx, launchReq)
}

// handleLaunchFailure handles instance launch failures
func (r *GPURequestReconciler) handleLaunchFailure(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	err error,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Error(err, "failed to launch instance")
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
	gpuRequest.Status.Message = fmt.Sprintf("Failed to launch instance: %v", err)
	if updateErr := r.updateStatusWithRetry(ctx, gpuRequest, log); updateErr != nil {
		log.Error(updateErr, "failed to update status to failed")
	}
	return ctrl.Result{RequeueAfter: FailedRequeue}, nil
}

// updateLaunchStatus updates the GPURequest status after successful launch
func (r *GPURequestReconciler) updateLaunchStatus(
	ctx context.Context,
	gpuRequest *tgpv1.GPURequest,
	instance *providers.GPUInstance,
	provider providers.ProviderClient,
	selectedProvider string,
	log logr.Logger,
) error {
	gpuRequest.Status.InstanceID = instance.ID
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseBooting
	gpuRequest.Status.Message = "Instance launched, waiting for boot"
	gpuRequest.Status.PublicIP = instance.PublicIP
	gpuRequest.Status.PrivateIP = instance.PrivateIP
	now := metav1.Time{Time: time.Now()}
	gpuRequest.Status.ProvisionedAt = &now

	// Try to get pricing info for cost tracking
	if r.Metrics != nil {
		pricing, priceErr := provider.GetNormalizedPricing(ctx, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region)
		if priceErr == nil && pricing != nil {
			r.Metrics.SetInstanceCost(selectedProvider, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region, pricing.PricePerHour)
			gpuRequest.Status.HourlyPrice = fmt.Sprintf("%.4f", pricing.PricePerHour)
		}
	}

	return r.updateStatusWithRetry(ctx, gpuRequest, log)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GPURequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tgpv1.GPURequest{}).
		Complete(r)
}
