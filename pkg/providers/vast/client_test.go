package vast

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

func TestClient_GetProviderName(t *testing.T) {
	client := NewClient("fake-api-key")

	t.Run("should return provider name", func(t *testing.T) {
		name := client.GetProviderName()
		if name != "vast.ai" {
			t.Errorf("Expected provider name to be 'vast.ai', got: %s", name)
		}
	})
}

func TestClient_ListOffers(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should return mock offers for testing", func(t *testing.T) {
		offers, err := client.ListOffers(ctx, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(offers) != 1 {
			t.Errorf("Expected 1 offer, got: %d", len(offers))
		}

		offer := offers[0]
		if offer.Provider != "vast.ai" {
			t.Errorf("Expected provider to be 'vast.ai', got: %s", offer.Provider)
		}
		if offer.GPUType != "RTX3090" {
			t.Errorf("Expected GPU type to be 'RTX3090', got: %s", offer.GPUType)
		}
		if offer.Region != "us-east-1" {
			t.Errorf("Expected region to be 'us-east-1', got: %s", offer.Region)
		}
		if offer.HourlyPrice <= 0 {
			t.Errorf("Expected price to be > 0, got: %f", offer.HourlyPrice)
		}
	})
}

func TestClient_LaunchInstance(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should launch a mock instance", func(t *testing.T) {
		spec := tgpv1.GPURequestSpec{
			Provider: "vast.ai",
			GPUType:  "RTX3090",
			TalosConfig: tgpv1.TalosConfig{
				Image: "factory.talos.dev/installer/test:v1.8.2",
			},
		}

		instance, err := client.LaunchInstance(ctx, spec)
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

func TestClient_GetPricing(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should return pricing info", func(t *testing.T) {
		pricing, err := client.GetPricing(ctx, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if pricing.GPUType != "RTX3090" {
			t.Errorf("Expected GPU type to be 'RTX3090', got: %s", pricing.GPUType)
		}
		if pricing.HourlyPrice <= 0 {
			t.Errorf("Expected price to be > 0, got: %f", pricing.HourlyPrice)
		}
	})
}
