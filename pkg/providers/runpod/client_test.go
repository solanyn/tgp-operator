package runpod

import (
	"context"
	"testing"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

func TestNewClient(t *testing.T) {
	t.Run("should create a new client with API key", func(t *testing.T) {
		client := NewClient("fake-api-key")
		if client == nil {
			t.Fatal("Expected client to not be nil")
		}
		if client.apiKey != "fake-api-key" {
			t.Errorf("Expected API key to be 'fake-api-key', got: %s", client.apiKey)
		}
	})
}

func TestClient_GetProviderInfo(t *testing.T) {
	client := NewClient("fake-api-key")

	t.Run("should return provider info", func(t *testing.T) {
		info := client.GetProviderInfo()
		if info.Name != "runpod" {
			t.Errorf("Expected provider name to be 'runpod', got: %s", info.Name)
		}
	})
}

func TestClient_LaunchInstance(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should launch a mock instance", func(t *testing.T) {
		req := &providers.LaunchRequest{
			GPUType: "RTX3090",
			Region:  "us-east-1",
			Image:   "factory.talos.dev/installer/test:v1.8.2",
			TalosConfig: &tgpv1.TalosConfig{
				Image: "factory.talos.dev/installer/test:v1.8.2",
			},
		}

		instance, err := client.LaunchInstance(ctx, req)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if instance.Status != providers.InstanceStatePending {
			t.Errorf("Expected status to be %s, got: %s", providers.InstanceStatePending, instance.Status)
		}
		if instance.ID == "" {
			t.Error("Expected instance ID to not be empty")
		}
	})
}

func TestClient_GetInstanceStatus(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should return mock status", func(t *testing.T) {
		status, err := client.GetInstanceStatus(ctx, "test-instance-id")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if status.State != providers.InstanceStateRunning {
			t.Errorf("Expected status to be %s, got: %s", providers.InstanceStateRunning, status.State)
		}
	})
}

func TestClient_TerminateInstance(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should terminate instance without error", func(t *testing.T) {
		err := client.TerminateInstance(ctx, "test-instance-id")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
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
