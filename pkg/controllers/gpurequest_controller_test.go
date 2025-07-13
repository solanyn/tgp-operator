package controllers

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/pricing"
	"github.com/solanyn/tgp-operator/pkg/providers"
	"github.com/solanyn/tgp-operator/pkg/providers/vast"
)

func TestGPURequestController_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := tgpv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&tgpv1.GPURequest{}).
		Build()

	reconciler := &GPURequestReconciler{
		Client: fakeClient,
		Log:    zap.New(zap.UseDevMode(true)),
		Scheme: scheme,
		Providers: map[string]providers.ProviderClient{
			"vast.ai": vast.NewClient("test-api-key"),
		},
		PricingCache: pricing.NewCache(time.Minute * 5),
	}

	ctx := context.Background()

	t.Run("should return without error when GPURequest does not exist", func(t *testing.T) {
		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      "non-existent",
				Namespace: "default",
			},
		}

		result, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != (ctrl.Result{}) {
			t.Errorf("Expected empty result, got: %+v", result)
		}
	})

	t.Run("should add finalizer on first reconcile", func(t *testing.T) {
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-gpu-request",
				Namespace: "default",
			},
			Spec: tgpv1.GPURequestSpec{
				Provider: "vast.ai",
				GPUType:  "RTX3090",
				TalosConfig: tgpv1.TalosConfig{
					Image: "factory.talos.dev/installer/test:v1.8.2",
					WireGuardConfig: tgpv1.WireGuardConfig{
						PrivateKey:     "test-private-key",
						PublicKey:      "test-public-key",
						ServerEndpoint: "vpn.example.com:51820",
						AllowedIPs:     []string{"10.0.0.0/24"},
						Address:        "10.0.0.2/24",
					},
				},
			},
		}

		if err := fakeClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      gpuRequest.Name,
				Namespace: gpuRequest.Namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !result.Requeue {
			t.Error("Expected requeue to be true")
		}

		var updated tgpv1.GPURequest
		if err := fakeClient.Get(ctx, req.NamespacedName, &updated); err != nil {
			t.Fatalf("Failed to get updated GPURequest: %v", err)
		}

		found := false
		for _, finalizer := range updated.Finalizers {
			if finalizer == FinalizerName {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected finalizer to be added")
		}
	})

	t.Run("should handle pending phase", func(t *testing.T) {
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-pending",
				Namespace:  "default",
				Finalizers: []string{FinalizerName},
			},
			Spec: tgpv1.GPURequestSpec{
				Provider: "vast.ai",
				GPUType:  "RTX3090",
			},
		}

		if err := fakeClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      gpuRequest.Name,
				Namespace: gpuRequest.Namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !result.Requeue {
			t.Error("Expected requeue to be true")
		}

		var updated tgpv1.GPURequest
		if err := fakeClient.Get(ctx, req.NamespacedName, &updated); err != nil {
			t.Fatalf("Failed to get updated GPURequest: %v", err)
		}

		if updated.Status.Phase != tgpv1.GPURequestPhaseProvisioning {
			t.Errorf("Expected phase to be %s, got: %s", tgpv1.GPURequestPhaseProvisioning, updated.Status.Phase)
		}
	})

	t.Run("should handle deletion", func(t *testing.T) {
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-deletion",
				Namespace:  "default",
				Finalizers: []string{FinalizerName},
			},
			Spec: tgpv1.GPURequestSpec{
				Provider: "vast.ai",
				GPUType:  "RTX3090",
			},
			Status: tgpv1.GPURequestStatus{
				Phase:      tgpv1.GPURequestPhaseReady,
				InstanceID: "test-instance-id",
			},
		}

		if err := fakeClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		if err := fakeClient.Delete(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to delete GPURequest: %v", err)
		}

		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      gpuRequest.Name,
				Namespace: gpuRequest.Namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != (ctrl.Result{}) {
			t.Errorf("Expected empty result, got: %+v", result)
		}
	})
}

func TestGPURequestController_handlePending(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := tgpv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&tgpv1.GPURequest{}).
		Build()

	reconciler := &GPURequestReconciler{
		Client: fakeClient,
		Log:    zap.New(zap.UseDevMode(true)),
		Scheme: scheme,
		Providers: map[string]providers.ProviderClient{
			"vast.ai": vast.NewClient("test-api-key"),
		},
		PricingCache: pricing.NewCache(time.Minute * 5),
	}

	ctx := context.Background()

	t.Run("should update status to provisioning", func(t *testing.T) {
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-request",
				Namespace: "default",
			},
			Status: tgpv1.GPURequestStatus{
				Phase: tgpv1.GPURequestPhasePending,
			},
		}

		if err := fakeClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		result, err := reconciler.handlePending(ctx, gpuRequest, reconciler.Log)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !result.Requeue {
			t.Error("Expected requeue to be true")
		}
		if gpuRequest.Status.Phase != tgpv1.GPURequestPhaseProvisioning {
			t.Errorf("Expected phase to be %s, got: %s", tgpv1.GPURequestPhaseProvisioning, gpuRequest.Status.Phase)
		}
	})
}

func TestGPURequestController_handleProvisioning(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := tgpv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&tgpv1.GPURequest{}).
		Build()

	reconciler := &GPURequestReconciler{
		Client: fakeClient,
		Log:    zap.New(zap.UseDevMode(true)),
		Scheme: scheme,
		Providers: map[string]providers.ProviderClient{
			"vast.ai": vast.NewClient("test-api-key"),
		},
		PricingCache: pricing.NewCache(time.Minute * 5),
	}

	ctx := context.Background()

	t.Run("should simulate provisioning and update to ready", func(t *testing.T) {
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-request",
				Namespace: "default",
			},
			Spec: tgpv1.GPURequestSpec{
				Provider: "vast.ai",
				GPUType:  "RTX3090",
			},
			Status: tgpv1.GPURequestStatus{
				Phase: tgpv1.GPURequestPhaseProvisioning,
			},
		}

		if err := fakeClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		result, err := reconciler.handleProvisioning(ctx, gpuRequest, reconciler.Log)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result.RequeueAfter != time.Second*30 {
			t.Errorf("Expected requeue after 30 seconds, got: %v", result.RequeueAfter)
		}
		if gpuRequest.Status.Phase != tgpv1.GPURequestPhaseProvisioning {
			t.Errorf("Expected phase to be %s, got: %s", tgpv1.GPURequestPhaseProvisioning, gpuRequest.Status.Phase)
		}
		if gpuRequest.Status.InstanceID == "" {
			t.Error("Expected instance ID to be set")
		}
	})
}
