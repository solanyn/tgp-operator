package lambdalabs

import (
	"context"
	"testing"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
	"github.com/solanyn/tgp-operator/pkg/providers"
)

func TestClient_LaunchInstance(t *testing.T) {
	client := NewClient("fake-api-key")
	ctx := context.Background()

	t.Run("should launch a mock instance", func(t *testing.T) {
		req := &providers.LaunchRequest{
			GPUType: "RTX3090",
			Region:  "us-west-2",
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
		pricing, err := client.GetNormalizedPricing(ctx, "RTX3090", "us-west-2")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if pricing.PricePerHour <= 0 {
			t.Errorf("Expected price to be > 0, got: %f", pricing.PricePerHour)
		}
	})
}
