// Package controllers implements Kubernetes controllers for the TGP operator
package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
	"github.com/solanyn/tgp-operator/pkg/providers"
	"github.com/solanyn/tgp-operator/pkg/providers/lambdalabs"
	"github.com/solanyn/tgp-operator/pkg/providers/paperspace"
	"github.com/solanyn/tgp-operator/pkg/providers/runpod"
)

const (
	GPUNodeClassFinalizerName = "tgp.io/gpunodeclass-finalizer"
)

// GPUNodeClassReconciler reconciles a GPUNodeClass object
type GPUNodeClassReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Config *config.OperatorConfig
}

// +kubebuilder:rbac:groups=tgp.io,resources=gpunodeclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodeclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodeclasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodepools,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles GPUNodeClass reconciliation
func (r *GPUNodeClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gpunodeclass", req.NamespacedName)

	// Fetch the GPUNodeClass instance
	var nodeClass tgpv1.GPUNodeClass
	if err := r.Get(ctx, req.NamespacedName, &nodeClass); err != nil {
		if errors.IsNotFound(err) {
			log.Info("GPUNodeClass resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GPUNodeClass")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if nodeClass.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &nodeClass, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&nodeClass, GPUNodeClassFinalizerName) {
		controllerutil.AddFinalizer(&nodeClass, GPUNodeClassFinalizerName)
		if err := r.Update(ctx, &nodeClass); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Validate provider configurations
	if err := r.validateProviders(ctx, &nodeClass, log); err != nil {
		log.Error(err, "Provider validation failed")
		r.updateCondition(&nodeClass, "ProviderValidation", metav1.ConditionFalse, "ValidationFailed", err.Error())
		if updateErr := r.Status().Update(ctx, &nodeClass); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Update ready condition
	r.updateCondition(&nodeClass, "Ready", metav1.ConditionTrue, "ValidationPassed", "GPUNodeClass is ready")
	if err := r.Status().Update(ctx, &nodeClass); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Update GPU availability status
	if err := r.updateGPUAvailability(ctx, &nodeClass, log); err != nil {
		log.Error(err, "Failed to update GPU availability")
		// Don't fail the reconcile if GPU discovery fails
	}

	log.Info("GPUNodeClass reconciled successfully")
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// handleDeletion handles GPUNodeClass deletion
func (r *GPUNodeClassReconciler) handleDeletion(ctx context.Context, nodeClass *tgpv1.GPUNodeClass, log logr.Logger) (ctrl.Result, error) {
	log.Info("Handling GPUNodeClass deletion")

	// Check for any active GPUNodePools using this class
	activeNodePools, err := r.getActiveNodePools(ctx, nodeClass, log)
	if err != nil {
		log.Error(err, "Failed to check for active GPUNodePools")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	if len(activeNodePools) > 0 {
		log.Info("Cannot delete GPUNodeClass with active GPUNodePools", "activeCount", len(activeNodePools))
		// Update status condition to indicate blocking
		r.updateCondition(nodeClass, "DeletionBlocked", metav1.ConditionTrue, "ActiveNodePools",
			fmt.Sprintf("Cannot delete: %d active GPUNodePools still reference this class", len(activeNodePools)))
		if updateErr := r.Status().Update(ctx, nodeClass); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	controllerutil.RemoveFinalizer(nodeClass, GPUNodeClassFinalizerName)
	if err := r.Update(ctx, nodeClass); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	log.Info("GPUNodeClass deleted successfully")
	return ctrl.Result{}, nil
}

// validateProviders validates that all configured providers have valid credentials
func (r *GPUNodeClassReconciler) validateProviders(ctx context.Context, nodeClass *tgpv1.GPUNodeClass, log logr.Logger) error {
	for _, providerConfig := range nodeClass.Spec.Providers {
		if providerConfig.Enabled != nil && !*providerConfig.Enabled {
			continue
		}

		// Validate credentials exist - use the namespace from the credentials reference
		namespace := providerConfig.CredentialsRef.Namespace
		if namespace == "" {
			namespace = nodeClass.Namespace
		}
		credentials, err := r.Config.GetProviderCredentials(ctx, r.Client, providerConfig.Name, namespace)
		if err != nil {
			return fmt.Errorf("failed to get credentials for provider %s: %w", providerConfig.Name, err)
		}

		// Test credentials by creating a client (basic validation)
		if credentials == "" {
			return fmt.Errorf("empty credentials for provider %s", providerConfig.Name)
		}

		// Validate provider credentials by creating a client and testing basic functionality
		if err := r.validateProviderClient(ctx, providerConfig.Name, credentials, log); err != nil {
			return fmt.Errorf("provider client validation failed for %s: %w", providerConfig.Name, err)
		}

		log.Info("Provider credentials validated", "provider", providerConfig.Name)
	}

	return nil
}

// validateProviderClient creates a provider client and tests basic functionality
func (r *GPUNodeClassReconciler) validateProviderClient(ctx context.Context, providerName, credentials string, log logr.Logger) error {
	// Create provider client based on provider name
	var providerClient providers.ProviderClient
	switch providerName {
	case "runpod":
		providerClient = runpod.NewClient(credentials)
	case "lambdalabs":
		providerClient = lambdalabs.NewClient(credentials)
	case "paperspace":
		providerClient = paperspace.NewClient(credentials)
	default:
		return fmt.Errorf("unsupported provider: %s", providerName)
	}

	// Test basic functionality - get provider info (this is usually lightweight)
	providerInfo := providerClient.GetProviderInfo()
	if providerInfo == nil {
		return fmt.Errorf("failed to get provider info")
	}

	log.V(1).Info("Provider client created successfully",
		"provider", providerName,
		"providerInfo", providerInfo.Name)

	// TODO: Consider adding a lightweight API call test here
	// For now, successful client creation and GetProviderInfo is sufficient
	// Future enhancement could test ListAvailableGPUs with a timeout

	return nil
}

// updateCondition updates a condition in the GPUNodeClass status
func (r *GPUNodeClassReconciler) updateCondition(nodeClass *tgpv1.GPUNodeClass, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Find and update existing condition or append new one
	for i, existingCondition := range nodeClass.Status.Conditions {
		if existingCondition.Type == conditionType {
			if existingCondition.Status != status {
				condition.LastTransitionTime = metav1.Now()
			} else {
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			nodeClass.Status.Conditions[i] = condition
			return
		}
	}

	nodeClass.Status.Conditions = append(nodeClass.Status.Conditions, condition)
}

// getActiveNodePools finds all GPUNodePools that reference this GPUNodeClass
func (r *GPUNodeClassReconciler) getActiveNodePools(ctx context.Context, nodeClass *tgpv1.GPUNodeClass, log logr.Logger) ([]tgpv1.GPUNodePool, error) {
	var nodePools tgpv1.GPUNodePoolList
	if err := r.List(ctx, &nodePools); err != nil {
		return nil, fmt.Errorf("failed to list GPUNodePools: %w", err)
	}

	var activeNodePools []tgpv1.GPUNodePool
	for _, nodePool := range nodePools.Items {
		// Skip node pools that are being deleted
		if nodePool.DeletionTimestamp != nil {
			continue
		}

		// Check if this node pool references our GPUNodeClass
		if nodePool.Spec.NodeClassRef.Name == nodeClass.Name {
			activeNodePools = append(activeNodePools, nodePool)
			log.V(1).Info("Found active GPUNodePool referencing this class",
				"nodePool", nodePool.Name,
				"namespace", nodePool.Namespace)
		}
	}

	return activeNodePools, nil
}

// updateGPUAvailability queries providers and updates the GPU availability status
func (r *GPUNodeClassReconciler) updateGPUAvailability(ctx context.Context, nodeClass *tgpv1.GPUNodeClass, log logr.Logger) error {
	availableGPUs := make(map[string][]tgpv1.GPUAvailability)
	
	for _, providerConfig := range nodeClass.Spec.Providers {
		if providerConfig.Enabled != nil && !*providerConfig.Enabled {
			continue
		}

		// Get credentials for this provider
		namespace := providerConfig.CredentialsRef.Namespace
		if namespace == "" {
			namespace = nodeClass.Namespace
		}
		credentials, err := r.Config.GetProviderCredentials(ctx, r.Client, providerConfig.Name, namespace)
		if err != nil {
			log.Error(err, "Failed to get credentials for provider", "provider", providerConfig.Name)
			continue
		}

		// Create provider client
		providerClient, err := r.createProviderClient(providerConfig.Name, credentials)
		if err != nil {
			log.Error(err, "Failed to create provider client", "provider", providerConfig.Name)
			continue
		}

		// Query available GPUs
		offers, err := providerClient.ListAvailableGPUs(ctx, &providers.GPUFilters{})
		if err != nil {
			log.Error(err, "Failed to query GPU availability", "provider", providerConfig.Name)
			continue
		}

		// Convert offers to GPU availability format
		var gpuAvailability []tgpv1.GPUAvailability
		gpuTypeMap := make(map[string]*tgpv1.GPUAvailability)
		
		for _, offer := range offers {
			key := offer.GPUType
			if existing, exists := gpuTypeMap[key]; exists {
				// Merge regions for same GPU type
				existing.Regions = mergeRegions(existing.Regions, []string{offer.Region})
			} else {
				spotPrice := ""
				if offer.IsSpot && offer.SpotPrice > 0 {
					spotPrice = fmt.Sprintf("%.2f", offer.SpotPrice)
				}
				
				gpu := &tgpv1.GPUAvailability{
					GPUType:      offer.GPUType,
					Regions:      []string{offer.Region},
					PricePerHour: fmt.Sprintf("%.2f", offer.HourlyPrice),
					Memory:       offer.Memory,
					Available:    offer.Available,
					SpotPrice:    &spotPrice,
					LastUpdated:  metav1.Now(),
				}
				gpuTypeMap[key] = gpu
				gpuAvailability = append(gpuAvailability, *gpu)
			}
		}

		if len(gpuAvailability) > 0 {
			availableGPUs[providerConfig.Name] = gpuAvailability
			log.V(1).Info("Updated GPU availability", "provider", providerConfig.Name, "gpuTypes", len(gpuAvailability))
		}
	}

	// Update status
	nodeClass.Status.AvailableGPUs = availableGPUs
	now := metav1.Now()
	nodeClass.Status.LastInventoryUpdate = &now

	if err := r.Status().Update(ctx, nodeClass); err != nil {
		return fmt.Errorf("failed to update GPU availability status: %w", err)
	}

	return nil
}

// createProviderClient creates a provider client based on provider name (duplicate of gpunodepool_controller)
func (r *GPUNodeClassReconciler) createProviderClient(providerName, credentials string) (providers.ProviderClient, error) {
	switch providerName {
	case "runpod":
		return runpod.NewClient(credentials), nil
	case "lambdalabs":
		return lambdalabs.NewClient(credentials), nil
	case "paperspace":
		return paperspace.NewClient(credentials), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// mergeRegions combines two region slices, removing duplicates
func mergeRegions(existing, new []string) []string {
	regionMap := make(map[string]bool)
	for _, region := range existing {
		regionMap[region] = true
	}
	for _, region := range new {
		regionMap[region] = true
	}
	
	var result []string
	for region := range regionMap {
		result = append(result, region)
	}
	return result
}

// SetupWithManager sets up the controller with the Manager
func (r *GPUNodeClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tgpv1.GPUNodeClass{}).
		Complete(r)
}
