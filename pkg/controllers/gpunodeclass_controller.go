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

	log.Info("GPUNodeClass reconciled successfully")
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// handleDeletion handles GPUNodeClass deletion
func (r *GPUNodeClassReconciler) handleDeletion(ctx context.Context, nodeClass *tgpv1.GPUNodeClass, log logr.Logger) (ctrl.Result, error) {
	log.Info("Handling GPUNodeClass deletion")

	// TODO: Check for any active GPUNodePools using this class
	// For now, just remove the finalizer
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

		// Validate credentials exist - use default namespace for now
		credentials, err := r.Config.GetProviderCredentials(ctx, r.Client, providerConfig.Name, "default")
		if err != nil {
			return fmt.Errorf("failed to get credentials for provider %s: %w", providerConfig.Name, err)
		}

		// Test credentials by creating a client (basic validation)
		if credentials == "" {
			return fmt.Errorf("empty credentials for provider %s", providerConfig.Name)
		}

		// TODO: Add proper client validation once provider interfaces are updated

		log.Info("Provider credentials validated", "provider", providerConfig.Name)
	}

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

// SetupWithManager sets up the controller with the Manager
func (r *GPUNodeClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tgpv1.GPUNodeClass{}).
		Complete(r)
}
