// Package controllers implements Kubernetes controllers for the TGP operator
package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
	"github.com/solanyn/tgp-operator/pkg/pricing"
)

const (
	GPUNodePoolFinalizerName = "tgp.io/gpunodepool-finalizer"
)

// GPUNodePoolReconciler reconciles a GPUNodePool object
type GPUNodePoolReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Config       *config.OperatorConfig
	PricingCache *pricing.Cache
}

// +kubebuilder:rbac:groups=tgp.io,resources=gpunodepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodepools/finalizers,verbs=update
// +kubebuilder:rbac:groups=tgp.io,resources=gpunodeclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles GPUNodePool reconciliation
func (r *GPUNodePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gpunodepool", req.NamespacedName)

	// Fetch the GPUNodePool instance
	var nodePool tgpv1.GPUNodePool
	if err := r.Get(ctx, req.NamespacedName, &nodePool); err != nil {
		if errors.IsNotFound(err) {
			log.Info("GPUNodePool resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GPUNodePool")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if nodePool.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &nodePool, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&nodePool, GPUNodePoolFinalizerName) {
		controllerutil.AddFinalizer(&nodePool, GPUNodePoolFinalizerName)
		if err := r.Update(ctx, &nodePool); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Get referenced GPUNodeClass
	nodeClass, err := r.getNodeClass(ctx, &nodePool)
	if err != nil {
		log.Error(err, "Failed to get referenced GPUNodeClass")
		r.updateCondition(&nodePool, "NodeClassReady", metav1.ConditionFalse, "NodeClassNotFound", err.Error())
		if updateErr := r.Status().Update(ctx, &nodePool); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Update NodeClass ready condition
	r.updateCondition(&nodePool, "NodeClassReady", metav1.ConditionTrue, "NodeClassFound", "Referenced GPUNodeClass is available")

	// TODO: Implement pod-driven provisioning logic
	// This would:
	// 1. Watch for unschedulable pods with GPU requirements
	// 2. Match pod requirements against this pool's capabilities
	// 3. Provision nodes when needed
	// 4. Handle node lifecycle and cleanup

	// For now, just update the status
	r.updateCondition(&nodePool, "Ready", metav1.ConditionTrue, "Initialized", "GPUNodePool is ready for provisioning")
	if err := r.Status().Update(ctx, &nodePool); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("GPUNodePool reconciled successfully", "nodeClass", nodeClass.Name)
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// handleDeletion handles GPUNodePool deletion
func (r *GPUNodePoolReconciler) handleDeletion(ctx context.Context, nodePool *tgpv1.GPUNodePool, log logr.Logger) (ctrl.Result, error) {
	log.Info("Handling GPUNodePool deletion")

	// TODO: Implement cleanup logic
	// This would:
	// 1. Drain and terminate all nodes created by this pool
	// 2. Wait for graceful termination
	// 3. Clean up any related resources

	controllerutil.RemoveFinalizer(nodePool, GPUNodePoolFinalizerName)
	if err := r.Update(ctx, nodePool); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	log.Info("GPUNodePool deleted successfully")
	return ctrl.Result{}, nil
}

// getNodeClass retrieves the GPUNodeClass referenced by the pool
func (r *GPUNodePoolReconciler) getNodeClass(ctx context.Context, nodePool *tgpv1.GPUNodePool) (*tgpv1.GPUNodeClass, error) {
	var nodeClass tgpv1.GPUNodeClass
	namespacedName := types.NamespacedName{
		Name: nodePool.Spec.NodeClassRef.Name,
		// GPUNodeClass is cluster-scoped, so no namespace
	}

	if err := r.Get(ctx, namespacedName, &nodeClass); err != nil {
		return nil, fmt.Errorf("failed to get GPUNodeClass %s: %w", nodePool.Spec.NodeClassRef.Name, err)
	}

	return &nodeClass, nil
}

// updateCondition updates a condition in the GPUNodePool status
func (r *GPUNodePoolReconciler) updateCondition(nodePool *tgpv1.GPUNodePool, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Find and update existing condition or append new one
	for i, existingCondition := range nodePool.Status.Conditions {
		if existingCondition.Type == conditionType {
			if existingCondition.Status != status {
				condition.LastTransitionTime = metav1.Now()
			} else {
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			nodePool.Status.Conditions[i] = condition
			return
		}
	}

	nodePool.Status.Conditions = append(nodePool.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager
func (r *GPUNodePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tgpv1.GPUNodePool{}).
		Owns(&corev1.Node{}). // Watch nodes created by this controller
		Complete(r)
}
