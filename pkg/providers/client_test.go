package providers

import (
	"testing"
	"time"
)

func TestGPUOffer(t *testing.T) {
	t.Run("should have required fields", func(t *testing.T) {
		offer := GPUOffer{
			ID:          "test-offer-123",
			Provider:    "vast.ai",
			GPUType:     "RTX3090",
			Region:      "us-east-1",
			HourlyPrice: 0.42,
			Memory:      24,
		}

		if offer.ID != "test-offer-123" {
			t.Errorf("Expected ID to be 'test-offer-123', got: %s", offer.ID)
		}
		if offer.Provider != "vast.ai" {
			t.Errorf("Expected Provider to be 'vast.ai', got: %s", offer.Provider)
		}
		if offer.GPUType != "RTX3090" {
			t.Errorf("Expected GPUType to be 'RTX3090', got: %s", offer.GPUType)
		}
		if offer.Region != "us-east-1" {
			t.Errorf("Expected Region to be 'us-east-1', got: %s", offer.Region)
		}
		if offer.HourlyPrice != 0.42 {
			t.Errorf("Expected HourlyPrice to be 0.42, got: %f", offer.HourlyPrice)
		}
		if offer.Memory != 24 {
			t.Errorf("Expected Memory to be 24, got: %d", offer.Memory)
		}
	})
}

func TestGPUInstance(t *testing.T) {
	t.Run("should have required fields", func(t *testing.T) {
		now := time.Now()
		instance := GPUInstance{
			ID:        "instance-456",
			Status:    InstanceStateRunning,
			PublicIP:  "203.0.113.1",
			CreatedAt: now,
		}

		if instance.ID != "instance-456" {
			t.Errorf("Expected ID to be 'instance-456', got: %s", instance.ID)
		}
		if instance.Status != InstanceStateRunning {
			t.Errorf("Expected Status to be %s, got: %s", InstanceStateRunning, instance.Status)
		}
		if instance.PublicIP != "203.0.113.1" {
			t.Errorf("Expected PublicIP to be '203.0.113.1', got: %s", instance.PublicIP)
		}
		if instance.CreatedAt != now {
			t.Errorf("Expected CreatedAt to be %v, got: %v", now, instance.CreatedAt)
		}
	})
}
