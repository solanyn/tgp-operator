package runpod

import (
	"context"
	"os"
	"testing"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

func TestClient_LaunchInstance(t *testing.T) {
	// Check if we have real RunPod credentials
	apiKey := "fake-api-key"
	if realKey := os.Getenv("RUNPOD_API_KEY"); realKey != "" {
		apiKey = realKey
	}

	client := NewClient(apiKey)
	ctx := context.Background()

	t.Run("should launch instance or fail gracefully with invalid credentials", func(t *testing.T) {
		req := &providers.LaunchRequest{
			GPUType: "RTX3090",
			Region:  "us-east-1",
			Image:   "factory.talos.dev/installer/test:v1.8.2",
			TalosConfig: &tgpv1.TalosConfig{
				Image: "factory.talos.dev/installer/test:v1.8.2",
			},
		}

		instance, err := client.LaunchInstance(ctx, req)

		if apiKey == "fake-api-key" {
			// With fake credentials, we expect an error
			if err == nil {
				t.Error("Expected error with fake credentials, got none")
			}
			// Skip further checks since we expect failure
			return
		}

		// With real credentials, we expect success
		if err != nil {
			t.Errorf("Expected no error with real credentials, got: %v", err)
			return
		}

		if instance == nil {
			t.Error("Expected instance to not be nil")
			return
		}

		if instance.Status != providers.InstanceStatePending && instance.Status != providers.InstanceStateRunning {
			t.Errorf("Expected status to be pending or running, got: %s", instance.Status)
		}
		if instance.ID == "" {
			t.Error("Expected instance ID to not be empty")
		}
	})
}

func TestClient_GetInstanceStatus(t *testing.T) {
	// Check if we have real RunPod credentials
	apiKey := "fake-api-key"
	if realKey := os.Getenv("RUNPOD_API_KEY"); realKey != "" {
		apiKey = realKey
	}

	client := NewClient(apiKey)
	ctx := context.Background()

	t.Run("should get instance status or fail gracefully with invalid credentials", func(t *testing.T) {
		status, err := client.GetInstanceStatus(ctx, "test-instance-id")

		if apiKey == "fake-api-key" {
			// With fake credentials, GetInstanceStatus should return a status with Failed state
			// because our implementation handles API errors by returning a Failed status
			if err != nil {
				t.Errorf("Expected no error (handled internally), got: %v", err)
				return
			}
			if status.State != providers.InstanceStateFailed {
				t.Errorf("Expected status to be Failed with fake credentials, got: %s", status.State)
			}
		} else {
			// With real credentials but fake instance ID, we expect a Failed state
			if err != nil {
				t.Errorf("Expected no error (handled internally), got: %v", err)
				return
			}
			// The status should indicate failure for non-existent instance
			t.Logf("Status for non-existent instance: %s - %s", status.State, status.Message)
		}
	})
}

func TestClient_TerminateInstance(t *testing.T) {
	// Check if we have real RunPod credentials
	apiKey := "fake-api-key"
	if realKey := os.Getenv("RUNPOD_API_KEY"); realKey != "" {
		apiKey = realKey
	}

	client := NewClient(apiKey)
	ctx := context.Background()

	t.Run("should terminate instance or fail gracefully with invalid credentials", func(t *testing.T) {
		err := client.TerminateInstance(ctx, "test-instance-id")

		if apiKey == "fake-api-key" {
			// With fake credentials, we expect an error
			if err == nil {
				t.Error("Expected error with fake credentials, got none")
			}
		} else {
			// With real credentials and a fake instance ID, we might get an error
			// This is expected - terminating a non-existent instance should fail
			t.Logf("Termination result: %v", err)
		}
	})
}

func TestClient_GetNormalizedPricing(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should return normalized pricing info", func(t *testing.T) {
		pricing, err := client.GetNormalizedPricing(ctx, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if pricing.PricePerHour <= 0 {
			t.Errorf("Expected price to be > 0, got: %f", pricing.PricePerHour)
		}
	})
}
