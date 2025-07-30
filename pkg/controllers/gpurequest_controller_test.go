package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/config"
	"github.com/solanyn/tgp-operator/pkg/pricing"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

// mockProvider implements ProviderClient for testing
type mockProvider struct {
	name           string
	shouldFail     bool
	instanceStatus *providers.InstanceStatus
	terminateError error
}

func (m *mockProvider) GetProviderInfo() *providers.ProviderInfo {
	return &providers.ProviderInfo{Name: m.name}
}

func (m *mockProvider) GetRateLimits() *providers.RateLimitInfo {
	return &providers.RateLimitInfo{RequestsPerSecond: 10}
}

func (m *mockProvider) TranslateGPUType(standard string) (string, error) {
	return standard, nil
}

func (m *mockProvider) TranslateRegion(standard string) (string, error) {
	return standard, nil
}

func (m *mockProvider) GetNormalizedPricing(ctx context.Context, gpuType, region string) (*providers.NormalizedPricing, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("pricing failed")
	}
	return &providers.NormalizedPricing{
		PricePerHour:   0.50,
		PricePerSecond: 0.50 / 3600,
		Currency:       "USD",
		BillingModel:   providers.BillingPerHour,
		LastUpdated:    time.Now(),
	}, nil
}

func (m *mockProvider) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("launch failed")
	}
	return &providers.GPUInstance{
		ID:        "test-instance-123",
		Status:    providers.InstanceStatePending,
		PublicIP:  "192.168.1.100",
		PrivateIP: "10.0.0.100",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockProvider) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("status check failed")
	}
	if m.instanceStatus != nil {
		return m.instanceStatus, nil
	}
	return &providers.InstanceStatus{
		State:     providers.InstanceStateRunning,
		Message:   "Instance is running",
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	if m.terminateError != nil {
		return m.terminateError
	}
	return nil
}

func (m *mockProvider) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	return []providers.GPUOffer{
		{
			ID:          "mock-offer-1",
			GPUType:     "RTX3090",
			Region:      "us-east",
			HourlyPrice: 0.50,
			Available:   true,
		},
	}, nil
}

// Helper function to set up test reconciler
func setupTestReconciler(t *testing.T) (*GPURequestReconciler, client.Client, context.Context) {
	scheme := runtime.NewScheme()
	if err := tgpv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 scheme: %v", err)
	}

	// Create mock Tailscale OAuth secret
	tailscaleSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tgp-operator-secret",
			Namespace: "test-operator-namespace",
		},
		Data: map[string][]byte{
			"client-id":     []byte("test-client-id"),
			"client-secret": []byte("test-client-secret"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&tgpv1.GPURequest{}).
		WithObjects(tailscaleSecret).
		Build()

	reconciler := &GPURequestReconciler{
		Client: fakeClient,
		Log:    zap.New(zap.UseDevMode(true)),
		Scheme: scheme,
		Providers: map[string]providers.ProviderClient{
			"runpod": &mockProvider{name: "runpod"},
		},
		PricingCache:      pricing.NewCache(time.Minute * 5),
		Config:            config.DefaultConfig(),
		OperatorNamespace: "test-operator-namespace",
	}

	return reconciler, fakeClient, context.Background()
}

func TestGPURequestController_Reconcile_NonExistent(t *testing.T) {
	reconciler, _, ctx := setupTestReconciler(t)

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
}

func TestGPURequestController_Reconcile_AddFinalizer(t *testing.T) {
	reconciler, fakeClient, ctx := setupTestReconciler(t)

	gpuRequest := &tgpv1.GPURequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gpu-request",
			Namespace: "default",
		},
		Spec: tgpv1.GPURequestSpec{
			Provider: "runpod",
			GPUType:  "RTX3090",
			TalosConfig: &tgpv1.TalosConfig{
				Image: "factory.talos.dev/installer/test:v1.8.2",
				TailscaleConfig: &tgpv1.TailscaleConfig{
					Hostname: "test-gpu-node",
					Tags:     []string{"tag:k8s"},
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
	if result.RequeueAfter == 0 && !result.Requeue {
		t.Error("Expected requeue to be requested")
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
}

func TestGPURequestController_Reconcile_PendingPhase(t *testing.T) {
	reconciler, fakeClient, ctx := setupTestReconciler(t)
	gpuRequest := &tgpv1.GPURequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-pending",
			Namespace:  "default",
			Finalizers: []string{FinalizerName},
		},
		Spec: tgpv1.GPURequestSpec{
			Provider: "runpod",
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
	if result.RequeueAfter == 0 && !result.Requeue {
		t.Error("Expected requeue to be requested")
	}

	var updated tgpv1.GPURequest
	if err := fakeClient.Get(ctx, req.NamespacedName, &updated); err != nil {
		t.Fatalf("Failed to get updated GPURequest: %v", err)
	}

	if updated.Status.Phase != tgpv1.GPURequestPhaseProvisioning {
		t.Errorf("Expected phase to be %s, got: %s", tgpv1.GPURequestPhaseProvisioning, updated.Status.Phase)
	}
}

func TestGPURequestController_Reconcile_Deletion(t *testing.T) {
	reconciler, fakeClient, ctx := setupTestReconciler(t)

	// Use the reconciler from setup (it already has no termination error)

	gpuRequest := &tgpv1.GPURequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-deletion",
			Namespace:  "default",
			Finalizers: []string{FinalizerName},
		},
		Spec: tgpv1.GPURequestSpec{
			Provider: "runpod",
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
			"runpod": &mockProvider{name: "runpod"},
		},
		PricingCache:      pricing.NewCache(time.Minute * 5),
		Config:            config.DefaultConfig(),
		OperatorNamespace: "test-namespace",
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
		if result.RequeueAfter == 0 && !result.Requeue {
			t.Error("Expected requeue to be requested")
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
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 scheme: %v", err)
	}

	// Create mock Tailscale OAuth secret
	tailscaleSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tgp-operator-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"client-id":     []byte("test-client-id"),
			"client-secret": []byte("test-client-secret"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tailscaleSecret).
		WithStatusSubresource(&tgpv1.GPURequest{}).
		Build()

	ctx := context.Background()

	t.Run("should simulate provisioning and update to ready", func(t *testing.T) {
		// Use a mock provider that returns Pending status initially
		mockProv := &mockProvider{
			name: "runpod",
			instanceStatus: &providers.InstanceStatus{
				State:     providers.InstanceStatePending,
				Message:   "Instance is starting",
				UpdatedAt: time.Now(),
			},
		}

		reconc := &GPURequestReconciler{
			Client: fakeClient,
			Log:    zap.New(zap.UseDevMode(true)),
			Scheme: scheme,
			Providers: map[string]providers.ProviderClient{
				"runpod": mockProv,
			},
			PricingCache:      pricing.NewCache(time.Minute * 5),
			Config:            config.DefaultConfig(),
			OperatorNamespace: "test-namespace",
		}

		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-request",
				Namespace: "default",
			},
			Spec: tgpv1.GPURequestSpec{
				Provider: "runpod",
				GPUType:  "RTX3090",
			},
			Status: tgpv1.GPURequestStatus{
				Phase: tgpv1.GPURequestPhaseProvisioning,
			},
		}

		if err := fakeClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		result, err := reconc.handleProvisioning(ctx, gpuRequest, reconc.Log)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result.RequeueAfter != ProvisioningRequeue {
			t.Errorf("Expected requeue after %v, got: %v", ProvisioningRequeue, result.RequeueAfter)
		}
		if gpuRequest.Status.Phase != tgpv1.GPURequestPhaseBooting {
			t.Errorf("Expected phase to be %s, got: %s", tgpv1.GPURequestPhaseBooting, gpuRequest.Status.Phase)
		}
		if gpuRequest.Status.InstanceID == "" {
			t.Error("Expected instance ID to be set")
		}
	})
}

func TestGPURequestController_SimplifiedSpec(t *testing.T) {
	t.Run("should accept GPURequest without TailscaleConfig", func(t *testing.T) {
		reconciler, fakeClient, ctx := setupTestReconciler(t)

		maxPrice := "0.50"
		// Create a simplified GPURequest that users should be able to submit
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-simple-request",
				Namespace: "default",
			},
			Spec: tgpv1.GPURequestSpec{
				Provider:       "runpod",
				GPUType:        "RTX3090",
				Region:         "US",
				MaxHourlyPrice: &maxPrice,
				Spot:           true,
				// Only image required - no TailscaleConfig
				TalosConfig: &tgpv1.TalosConfig{
					Image: "test-image",
					// TailscaleConfig should use operator defaults
				},
			},
		}

		err := fakeClient.Create(ctx, gpuRequest)
		if err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		// Reconcile should succeed
		req := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      gpuRequest.Name,
				Namespace: gpuRequest.Namespace,
			},
		}

		result, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Reconcile failed: %v", err)
		}
		// First reconcile adds finalizer and requeues
		if !result.Requeue {
			t.Error("Expected requeue after adding finalizer")
		}

		// Verify the GPURequest was processed (finalizer added)
		updatedGPURequest := &tgpv1.GPURequest{}
		err = fakeClient.Get(ctx, types.NamespacedName{
			Name:      gpuRequest.Name,
			Namespace: gpuRequest.Namespace,
		}, updatedGPURequest)
		if err != nil {
			t.Fatalf("Failed to get updated GPURequest: %v", err)
		}

		found := false
		for _, finalizer := range updatedGPURequest.Finalizers {
			if finalizer == FinalizerName {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected finalizer to be added")
		}
	})
}
