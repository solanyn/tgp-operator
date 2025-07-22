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
	name string
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
	return &providers.GPUInstance{
		ID:        "integration-test-instance",
		Status:    providers.InstanceStatePending,
		PublicIP:  "192.168.1.100",
		PrivateIP: "10.0.0.100",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockProvider) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
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

// setupTestEnvironment sets up the test environment and returns the k8s client
func setupTestEnvironment(t *testing.T) (client.Client, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())

	// Setup test environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{"../../config/crd/bases"},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("Failed to start test environment: %v", err)
	}

	// Setup scheme
	scheme := runtime.NewScheme()
	if schemeErr := tgpv1.AddToScheme(scheme); schemeErr != nil {
		t.Fatalf("Failed to add scheme: %v", schemeErr)
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
			"runpod": &mockProvider{name: "runpod"},
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

	cleanup := func() {
		cancel()
		if stopErr := testEnv.Stop(); stopErr != nil {
			t.Errorf("Failed to stop test environment: %v", stopErr)
		}
	}

	return mgr.GetClient(), ctx, cleanup
}

// createTestGPURequest creates a test GPURequest with default values
func createTestGPURequest(name string) *tgpv1.GPURequest {
	return &tgpv1.GPURequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: tgpv1.GPURequestSpec{
			Provider: "runpod",
			GPUType:  "RTX3090",
			TalosConfig: tgpv1.TalosConfig{
				Image: "factory.talos.dev/installer/test:v1.8.2",
				TailscaleConfig: tgpv1.TailscaleConfig{
					Hostname: "test-gpu-node",
					Tags:     []string{"tag:k8s"},
				},
			},
		},
	}
}

func TestE2E(t *testing.T) {
	k8sClient, ctx, cleanup := setupTestEnvironment(t)
	defer cleanup()

	t.Run("GPURequest lifecycle", func(t *testing.T) {
		gpuRequest := createTestGPURequest("e2e-test-gpu-request")

		if err := k8sClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		// Wait for finalizer to be added
		if !waitForCondition(t, k8sClient, gpuRequest, 10*time.Second, func(obj *tgpv1.GPURequest) bool {
			return len(obj.Finalizers) > 0
		}) {
			t.Fatal("Finalizer was not added")
		}

		// Wait for status to be updated (any phase is fine for K8s API testing)
		if !waitForCondition(t, k8sClient, gpuRequest, 10*time.Second, func(obj *tgpv1.GPURequest) bool {
			return obj.Status.Phase != ""
		}) {
			t.Fatal("Status was not updated")
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

func waitForCondition(
	_ *testing.T,
	client client.Client,
	obj *tgpv1.GPURequest,
	timeout time.Duration,
	condition func(*tgpv1.GPURequest) bool,
) bool {
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

func waitForDeletion(_ *testing.T, client client.Client, obj *tgpv1.GPURequest, timeout time.Duration) bool {
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
