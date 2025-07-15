package integration

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/controllers"
	"github.com/solanyn/tgp-operator/pkg/pricing"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

// mockProvider implements ProviderClient for integration testing
type mockProvider struct {
	name         string
	statusCalls  int
	instanceTime time.Time
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
	return &providers.NormalizedPricing{
		PricePerHour:   0.50,
		PricePerSecond: 0.50 / 3600,
		Currency:       "USD",
		BillingModel:   providers.BillingPerHour,
		LastUpdated:    time.Now(),
	}, nil
}

func (m *mockProvider) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	m.instanceTime = time.Now()
	m.statusCalls = 0
	return &providers.GPUInstance{
		ID:        "integration-test-instance",
		Status:    providers.InstanceStatePending,
		PublicIP:  "192.168.1.100",
		PrivateIP: "10.0.0.100",
		CreatedAt: m.instanceTime,
	}, nil
}

func (m *mockProvider) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	m.statusCalls++
	
	// Simulate provisioning time - first few calls return pending, then running
	if m.statusCalls <= 2 || time.Since(m.instanceTime) < 2*time.Second {
		return &providers.InstanceStatus{
			State:     providers.InstanceStatePending,
			Message:   "Instance is starting up",
			UpdatedAt: time.Now(),
		}, nil
	}
	
	return &providers.InstanceStatus{
		State:     providers.InstanceStateRunning,
		Message:   "Instance is running",
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (m *mockProvider) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	return nil, nil
}

func TestE2E(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup test environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{"../../config/crd/bases"},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("Failed to start test environment: %v", err)
	}
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("Failed to stop test environment: %v", err)
		}
	}()

	// Setup scheme
	scheme := runtime.NewScheme()
	if err := tgpv1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	// Create manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme,
		HealthProbeBindAddress:  "0",
		LeaderElection:          false,
		LeaderElectionNamespace: "",
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Setup controller with mock providers
	if err := (&controllers.GPURequestReconciler{
		Client: mgr.GetClient(),
		Log:    zap.New(zap.UseDevMode(true)),
		Scheme: mgr.GetScheme(),
		Providers: map[string]providers.ProviderClient{
			"vast.ai": &mockProvider{name: "vast.ai"},
		},
		PricingCache: pricing.NewCache(time.Minute * 5),
	}).SetupWithManager(mgr); err != nil {
		t.Fatalf("Failed to setup controller: %v", err)
	}

	// Start manager in background
	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Errorf("Failed to start manager: %v", err)
		}
	}()

	// Wait for manager to be ready
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		t.Fatal("Failed to sync cache")
	}

	k8sClient := mgr.GetClient()

	t.Run("GPURequest lifecycle", func(t *testing.T) {
		// Create GPURequest
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "e2e-test-gpu-request",
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

		if err := k8sClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		// Wait for finalizer to be added
		if !waitForCondition(t, k8sClient, gpuRequest, 10*time.Second, func(obj *tgpv1.GPURequest) bool {
			return len(obj.Finalizers) > 0
		}) {
			t.Fatal("Finalizer was not added")
		}

		// Wait for status to be updated to Provisioning
		if !waitForCondition(t, k8sClient, gpuRequest, 10*time.Second, func(obj *tgpv1.GPURequest) bool {
			return obj.Status.Phase == tgpv1.GPURequestPhaseProvisioning
		}) {
			t.Fatal("Status was not updated to Provisioning")
		}

		// Wait for status to be updated to Ready
		if !waitForCondition(t, k8sClient, gpuRequest, 15*time.Second, func(obj *tgpv1.GPURequest) bool {
			return obj.Status.Phase == tgpv1.GPURequestPhaseReady
		}) {
			t.Fatal("Status was not updated to Ready")
		}

		// Verify instance details are set
		objKey := types.NamespacedName{Name: gpuRequest.Name, Namespace: gpuRequest.Namespace}
		if err := k8sClient.Get(ctx, objKey, gpuRequest); err != nil {
			t.Fatalf("Failed to get updated GPURequest: %v", err)
		}

		if gpuRequest.Status.InstanceID == "" {
			t.Error("Expected InstanceID to be set")
		}
		if gpuRequest.Status.NodeName == "" {
			t.Error("Expected NodeName to be set")
		}

		// Test deletion
		if err := k8sClient.Delete(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to delete GPURequest: %v", err)
		}

		// Wait for resource to be fully deleted
		if !waitForDeletion(t, k8sClient, gpuRequest, 10*time.Second) {
			t.Fatal("Resource was not deleted")
		}
	})
}

func waitForCondition(t *testing.T, client client.Client, obj *tgpv1.GPURequest, timeout time.Duration, condition func(*tgpv1.GPURequest) bool) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(250 * time.Millisecond):
			objKey := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
			if err := client.Get(ctx, objKey, obj); err != nil {
				continue
			}
			if condition(obj) {
				return true
			}
		}
	}
}

func waitForDeletion(t *testing.T, client client.Client, obj *tgpv1.GPURequest, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(250 * time.Millisecond):
			objKey := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
			err := client.Get(ctx, objKey, obj)
			if err != nil && errors.IsNotFound(err) {
				return true
			}
		}
	}
}
