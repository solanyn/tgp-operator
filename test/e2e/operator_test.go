package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/controllers"
	"github.com/solanyn/tgp-operator/pkg/pricing"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

// mockProvider implements ProviderClient for E2E testing
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
		InstanceID: "mock-instance-123",
		PublicIP:   "192.168.1.100",
		PrivateIP:  "10.0.0.100",
		Status:     "provisioning",
		GPUType:    req.GPUType,
		Region:     req.Region,
		Provider:   m.name,
	}, nil
}

func (m *mockProvider) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	m.statusCalls++
	// Simulate provisioning time - first few calls return "pending", then "running"
	if m.statusCalls < 3 {
		return &providers.InstanceStatus{
			InstanceID: instanceID,
			Status:     "pending",
			PublicIP:   "192.168.1.100",
			PrivateIP:  "10.0.0.100",
		}, nil
	}
	return &providers.InstanceStatus{
		InstanceID: instanceID,
		Status:     "running",
		PublicIP:   "192.168.1.100",
		PrivateIP:  "10.0.0.100",
	}, nil
}

func (m *mockProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (m *mockProvider) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	return []providers.GPUOffer{
		{
			GPUType:      "RTX3090",
			PricePerHour: 0.50,
			Region:       "us-east-1",
			Provider:     m.name,
			Available:    true,
		},
	}, nil
}

func TestE2E(t *testing.T) {
	if os.Getenv("USE_EXISTING_CLUSTER") != "true" {
		t.Skip("Skipping e2e tests, set USE_EXISTING_CLUSTER=true to run")
	}

	t.Log("🚀 Starting e2e tests against Talos cluster...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup scheme
	testScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("Failed to add core scheme: %v", err)
	}
	if err := tgpv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("Failed to add tgp scheme: %v", err)
	}

	// Get kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}

	// Create config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Create manager
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                  testScheme,
		HealthProbeBindAddress:  "0",
		LeaderElection:          false,
		LeaderElectionNamespace: "",
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Setup controller with mock provider
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

	k8sClient := mgr.GetClient()

	t.Run("GPURequest_lifecycle", func(t *testing.T) {
		t.Log("Testing GPURequest lifecycle on Talos cluster...")

		// Use default namespace for simplicity
		namespace := "default"

		// Create GPURequest
		gpuRequest := &tgpv1.GPURequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "e2e-gpu-request",
				Namespace: namespace,
			},
			Spec: tgpv1.GPURequestSpec{
				Provider: "runpod",
				GPUType:  "RTX3090",
				Region:   "us-east-1",
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

		t.Log("Creating GPURequest...")
		if err := k8sClient.Create(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to create GPURequest: %v", err)
		}

		// Test finalizer addition
		t.Log("Waiting for finalizer to be added...")
		if !waitForCondition(t, k8sClient, gpuRequest, 30*time.Second, func(obj *tgpv1.GPURequest) bool {
			return len(obj.Finalizers) > 0
		}) {
			t.Fatal("Finalizer was not added within timeout")
		}
		t.Log("✓ Finalizer added successfully")

		// Test status progression: Pending -> Provisioning
		t.Log("Waiting for status to progress to Provisioning...")
		if !waitForCondition(t, k8sClient, gpuRequest, 30*time.Second, func(obj *tgpv1.GPURequest) bool {
			return obj.Status.Phase == tgpv1.GPURequestPhaseProvisioning
		}) {
			t.Fatal("Status did not progress to Provisioning within timeout")
		}
		t.Log("✓ Status progressed to Provisioning")

		// Test status progression: Provisioning -> Ready
		t.Log("Waiting for status to progress to Ready...")
		if !waitForCondition(t, k8sClient, gpuRequest, 60*time.Second, func(obj *tgpv1.GPURequest) bool {
			return obj.Status.Phase == tgpv1.GPURequestPhaseReady
		}) {
			t.Fatal("Status did not progress to Ready within timeout")
		}
		t.Log("✓ Status progressed to Ready")

		// Verify instance details
		t.Log("Verifying instance details...")
		objKey := types.NamespacedName{Name: gpuRequest.Name, Namespace: gpuRequest.Namespace}
		if err := k8sClient.Get(ctx, objKey, gpuRequest); err != nil {
			t.Fatalf("Failed to get updated GPURequest: %v", err)
		}

		if gpuRequest.Status.InstanceID == "" {
			t.Error("Expected InstanceID to be set")
		} else {
			t.Logf("✓ InstanceID set: %s", gpuRequest.Status.InstanceID)
		}

		if gpuRequest.Status.NodeName == "" {
			t.Error("Expected NodeName to be set")
		} else {
			t.Logf("✓ NodeName set: %s", gpuRequest.Status.NodeName)
		}

		if gpuRequest.Status.Message == "" {
			t.Error("Expected status message to be set")
		} else {
			t.Logf("✓ Status message: %s", gpuRequest.Status.Message)
		}

		// Test deletion workflow
		t.Log("Testing deletion workflow...")
		if err := k8sClient.Delete(ctx, gpuRequest); err != nil {
			t.Fatalf("Failed to delete GPURequest: %v", err)
		}

		// Wait for cleanup and removal
		t.Log("Waiting for resource cleanup...")
		if !waitForDeletion(t, k8sClient, gpuRequest, 60*time.Second) {
			t.Fatal("Resource was not deleted within timeout")
		}
		t.Log("✓ Resource deleted successfully")

		t.Log("🎉 E2E test completed successfully!")
	})
}

func waitForCondition(
	t *testing.T,
	client client.Client,
	obj *tgpv1.GPURequest,
	timeout time.Duration,
	condition func(*tgpv1.GPURequest) bool,
) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Get final state for debugging
			objKey := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
			if err := client.Get(context.Background(), objKey, obj); err == nil {
				t.Logf("Final state - Phase: %s, Message: %s, Finalizers: %v",
					obj.Status.Phase, obj.Status.Message, obj.Finalizers)
			}
			return false
		case <-ticker.C:
			objKey := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
			if err := client.Get(ctx, objKey, obj); err != nil {
				t.Logf("Error getting object: %v", err)
				continue
			}
			if condition(obj) {
				return true
			}
			t.Logf("Waiting... Current phase: %s, message: %s", obj.Status.Phase, obj.Status.Message)
		}
	}
}

func waitForDeletion(t *testing.T, client client.Client, obj *tgpv1.GPURequest, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			objKey := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}
			err := client.Get(ctx, objKey, obj)
			if err != nil && errors.IsNotFound(err) {
				return true
			}
			if err == nil {
				t.Logf("Still exists... Phase: %s, Finalizers: %v", obj.Status.Phase, obj.Finalizers)
			}
		}
	}
}
