package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/pricing"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

const (
	FinalizerName = "tgp.io/finalizer"
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

	// Fetch the GPURequest instance
	var gpuRequest tgpv1.GPURequest
	if err := r.Client.Get(ctx, req.NamespacedName, &gpuRequest); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch GPURequest")
		return ctrl.Result{}, err
	}

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
	case tgpv1.GPURequestPhaseReady:
		return r.handleRunning(ctx, &gpuRequest, log)
	case tgpv1.GPURequestPhaseFailed:
		return r.handleFailed(ctx, &gpuRequest, log)
	default:
		log.Info("unknown phase", "phase", gpuRequest.Status.Phase)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
}

func (r *GPURequestReconciler) handlePending(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling pending GPURequest")

	// Update status to provisioning
	gpuRequest.Status.Phase = tgpv1.GPURequestPhaseProvisioning
	gpuRequest.Status.Message = "Selecting provider and provisioning GPU instance"

	if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
		log.Error(err, "failed to update status to provisioning")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *GPURequestReconciler) handleProvisioning(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling provisioning GPURequest")

	var provider providers.ProviderClient
	var selectedProvider string

	// Use cached provider selection if available
	if gpuRequest.Status.SelectedProvider != "" {
		var exists bool
		provider, exists = r.Providers[gpuRequest.Status.SelectedProvider]
		if exists {
			selectedProvider = gpuRequest.Status.SelectedProvider
			log.Info("using cached provider selection", "provider", selectedProvider)
		} else {
			log.Info("cached provider no longer available, reselecting", "cached", gpuRequest.Status.SelectedProvider)
			gpuRequest.Status.SelectedProvider = ""
		}
	}

	// Select provider if not cached or cache invalid
	if selectedProvider == "" {
		if gpuRequest.Spec.Provider != "" {
			var exists bool
			provider, exists = r.Providers[gpuRequest.Spec.Provider]
			if !exists {
				log.Error(fmt.Errorf("unknown provider"), "provider not supported", "provider", gpuRequest.Spec.Provider)
				gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
				gpuRequest.Status.Message = fmt.Sprintf("Provider %s not supported", gpuRequest.Spec.Provider)
				if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
					log.Error(err, "failed to update status to failed")
				}
				return ctrl.Result{}, nil
			}
			selectedProvider = gpuRequest.Spec.Provider
		} else {
			if r.PricingCache != nil {
				log.Info("selecting best price provider", "gpuType", gpuRequest.Spec.GPUType, "region", gpuRequest.Spec.Region)
				bestPrice, err := r.PricingCache.GetBestPrice(ctx, r.Providers, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region)
				if err != nil {
					log.Error(err, "failed to get best price, using first available provider")
					for name, p := range r.Providers {
						provider = p
						selectedProvider = name
						break
					}
				} else {
					for name, p := range r.Providers {
						pricing, _ := p.GetNormalizedPricing(ctx, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region)
						if pricing != nil && pricing.PricePerHour == bestPrice.PricePerHour {
							provider = p
							selectedProvider = name
							break
						}
					}
					log.Info("selected provider based on pricing", "provider", selectedProvider, "price", bestPrice.PricePerHour)
				}
			} else {
				for name, p := range r.Providers {
					provider = p
					selectedProvider = name
					break
				}
			}
		}

		// Cache the provider selection
		gpuRequest.Status.SelectedProvider = selectedProvider
	}

	if gpuRequest.Status.InstanceID == "" {
		log.Info("launching instance", "provider", selectedProvider)

		// Convert spec to launch request
		maxPrice := 0.0
		if gpuRequest.Spec.MaxHourlyPrice != nil {
			if price, err := gpuRequest.Spec.GetMaxHourlyPriceFloat(); err == nil {
				maxPrice = price
			}
		}

		launchReq := &providers.LaunchRequest{
			GPUType:      gpuRequest.Spec.GPUType,
			Region:       gpuRequest.Spec.Region,
			Image:        gpuRequest.Spec.TalosConfig.Image,
			SpotInstance: gpuRequest.Spec.Spot,
			MaxPrice:     maxPrice,
			TalosConfig:  &gpuRequest.Spec.TalosConfig,
		}

		instance, err := provider.LaunchInstance(ctx, launchReq)
		if err != nil {
			log.Error(err, "failed to launch instance")
			gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
			gpuRequest.Status.Message = fmt.Sprintf("Failed to launch instance: %v", err)
			if updateErr := r.updateStatusWithRetry(ctx, gpuRequest, log); updateErr != nil {
				log.Error(updateErr, "failed to update status to failed")
			}
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		gpuRequest.Status.InstanceID = instance.ID
		gpuRequest.Status.PublicIP = instance.PublicIP
		gpuRequest.Status.PrivateIP = instance.PrivateIP
		gpuRequest.Status.Message = "Instance launched, waiting for ready state"

		// Get and store pricing information
		if pricing, err := provider.GetNormalizedPricing(ctx, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region); err == nil {
			gpuRequest.Status.SetHourlyPriceFloat(pricing.PricePerHour)
		}

		// Update termination time if maxLifetime is set
		gpuRequest.UpdateTerminationTime()

		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status with instance ID")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
	}

	// Check if instance should be terminated due to maxLifetime
	if gpuRequest.IsTerminationDue() {
		log.Info("instance has reached maxLifetime, terminating", "instanceId", gpuRequest.Status.InstanceID)
		if err := provider.TerminateInstance(ctx, gpuRequest.Status.InstanceID); err != nil {
			log.Error(err, "failed to terminate instance due to maxLifetime")
		}
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseTerminating
		gpuRequest.Status.Message = "Instance terminated due to maxLifetime"
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status to terminating")
		}
		return ctrl.Result{RequeueAfter: TerminatingRequeue}, nil
	}

	status, err := provider.GetInstanceStatus(ctx, gpuRequest.Status.InstanceID)
	if err != nil {
		log.Error(err, "failed to get instance status")
		return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
	}

	switch status.State {
	case providers.InstanceStateRunning:
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseReady
		gpuRequest.Status.Message = "GPU instance is ready"
		gpuRequest.Status.NodeName = fmt.Sprintf("gpu-node-%s", gpuRequest.Name)
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status to ready")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: RunningRequeue}, nil
	case providers.InstanceStateFailed:
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
		gpuRequest.Status.Message = fmt.Sprintf("Instance failed: %s", status.Message)
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status to failed")
		}
		return ctrl.Result{RequeueAfter: FailedRequeue}, nil
	default:
		log.Info("waiting for instance to be ready", "state", status.State)
		gpuRequest.Status.Message = fmt.Sprintf("Instance state: %s", status.State)
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status message")
		}
		return ctrl.Result{RequeueAfter: ProvisioningRequeue}, nil
	}
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
		gpuRequest.Status.Message = "Instance terminated due to maxLifetime"
		if err := r.updateStatusWithRetry(ctx, gpuRequest, log); err != nil {
			log.Error(err, "failed to update status to terminating")
		}
		return ctrl.Result{RequeueAfter: TerminatingRequeue}, nil
	}

	// TODO: Check for idle timeout by monitoring pod scheduling

	// Update heartbeat
	gpuRequest.Status.LastHeartbeat = &metav1.Time{Time: time.Now()}
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

// SetupWithManager sets up the controller with the Manager.
func (r *GPURequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tgpv1.GPURequest{}).
		Complete(r)
}
