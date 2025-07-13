package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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

	if err := r.Status().Update(ctx, gpuRequest); err != nil {
		log.Error(err, "failed to update status to provisioning")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *GPURequestReconciler) handleProvisioning(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling provisioning GPURequest")

	var provider providers.ProviderClient
	var selectedProvider string

	if gpuRequest.Spec.Provider != "" {
		var exists bool
		provider, exists = r.Providers[gpuRequest.Spec.Provider]
		if !exists {
			log.Error(fmt.Errorf("unknown provider"), "provider not supported", "provider", gpuRequest.Spec.Provider)
			gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
			gpuRequest.Status.Message = fmt.Sprintf("Provider %s not supported", gpuRequest.Spec.Provider)
			if err := r.Status().Update(ctx, gpuRequest); err != nil {
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
					pricing, _ := p.GetPricing(ctx, gpuRequest.Spec.GPUType, gpuRequest.Spec.Region)
					if pricing != nil && pricing.HourlyPrice == bestPrice.HourlyPrice {
						provider = p
						selectedProvider = name
						break
					}
				}
				log.Info("selected provider based on pricing", "provider", selectedProvider, "price", bestPrice.HourlyPrice)
			}
		} else {
			for name, p := range r.Providers {
				provider = p
				selectedProvider = name
				break
			}
		}
	}

	if gpuRequest.Status.InstanceID == "" {
		log.Info("launching instance", "provider", selectedProvider)
		instance, err := provider.LaunchInstance(ctx, gpuRequest.Spec)
		if err != nil {
			log.Error(err, "failed to launch instance")
			gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
			gpuRequest.Status.Message = fmt.Sprintf("Failed to launch instance: %v", err)
			if updateErr := r.Status().Update(ctx, gpuRequest); updateErr != nil {
				log.Error(updateErr, "failed to update status to failed")
			}
			return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
		}

		gpuRequest.Status.InstanceID = instance.ID
		gpuRequest.Status.Message = "Instance launched, waiting for ready state"
		if err := r.Status().Update(ctx, gpuRequest); err != nil {
			log.Error(err, "failed to update status with instance ID")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	status, err := provider.GetInstanceStatus(ctx, gpuRequest.Status.InstanceID)
	if err != nil {
		log.Error(err, "failed to get instance status")
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	switch status.State {
	case providers.InstanceStateRunning:
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseReady
		gpuRequest.Status.Message = "GPU instance is ready"
		gpuRequest.Status.NodeName = fmt.Sprintf("gpu-node-%s", gpuRequest.Name)
		if err := r.Status().Update(ctx, gpuRequest); err != nil {
			log.Error(err, "failed to update status to ready")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	case providers.InstanceStateFailed:
		gpuRequest.Status.Phase = tgpv1.GPURequestPhaseFailed
		gpuRequest.Status.Message = fmt.Sprintf("Instance failed: %s", status.Message)
		if err := r.Status().Update(ctx, gpuRequest); err != nil {
			log.Error(err, "failed to update status to failed")
		}
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	default:
		log.Info("waiting for instance to be ready", "state", status.State)
		gpuRequest.Status.Message = fmt.Sprintf("Instance state: %s", status.State)
		if err := r.Status().Update(ctx, gpuRequest); err != nil {
			log.Error(err, "failed to update status message")
		}
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}
}

func (r *GPURequestReconciler) handleRunning(ctx context.Context, gpuRequest *tgpv1.GPURequest, log logr.Logger) (ctrl.Result, error) {
	log.Info("handling running GPURequest")

	// TODO: Monitor instance health and handle TTL
	// For now, just requeue for monitoring
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
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
