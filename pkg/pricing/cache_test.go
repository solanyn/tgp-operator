package pricing

import (
	"context"
	"testing"
	"time"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

type mockProvider struct {
	name      string
	pricing   *providers.NormalizedPricing
	callCount int
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
	m.callCount++
	return &providers.NormalizedPricing{
		PricePerHour:   m.pricing.PricePerHour,
		PricePerSecond: m.pricing.PricePerHour / 3600,
		Currency:       "USD",
		BillingModel:   providers.BillingPerHour,
		LastUpdated:    time.Now(),
	}, nil
}

func (m *mockProvider) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	return nil, nil
}

func (m *mockProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (m *mockProvider) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	return nil, nil
}

func (m *mockProvider) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	return nil, nil
}


func TestNewCache(t *testing.T) {
	t.Run("should create a new cache with TTL", func(t *testing.T) {
		cache := NewCache(time.Minute * 5)
		if cache == nil {
			t.Fatal("Expected cache to not be nil")
		}
	})
}

func TestCache_GetPricing(t *testing.T) {
	ctx := context.Background()

	provider1 := &mockProvider{
		name: "vast.ai",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.42,
		},
	}

	provider2 := &mockProvider{
		name: "runpod",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.38,
		},
	}

	providers := map[string]providers.ProviderClient{
		"vast.ai": provider1,
		"runpod":  provider2,
	}

	cache := NewCache(time.Minute * 5)

	t.Run("should fetch pricing from all providers", func(t *testing.T) {
		pricing, err := cache.GetPricing(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if len(pricing) != 2 {
			t.Errorf("Expected 2 pricing entries, got: %d", len(pricing))
		}

		if provider1.callCount != 1 {
			t.Errorf("Expected provider1 to be called once, got: %d", provider1.callCount)
		}

		if provider2.callCount != 1 {
			t.Errorf("Expected provider2 to be called once, got: %d", provider2.callCount)
		}
	})

	t.Run("should return cached pricing on second call", func(t *testing.T) {
		pricing, err := cache.GetPricing(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if len(pricing) != 2 {
			t.Errorf("Expected 2 pricing entries, got: %d", len(pricing))
		}

		if provider1.callCount != 1 {
			t.Errorf("Expected provider1 to still be called once, got: %d", provider1.callCount)
		}

		if provider2.callCount != 1 {
			t.Errorf("Expected provider2 to still be called once, got: %d", provider2.callCount)
		}
	})
}

func TestCache_GetBestPrice(t *testing.T) {
	ctx := context.Background()

	provider1 := &mockProvider{
		name: "vast.ai",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.42,
		},
	}

	provider2 := &mockProvider{
		name: "runpod",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.38,
		},
	}

	providers := map[string]providers.ProviderClient{
		"vast.ai": provider1,
		"runpod":  provider2,
	}

	cache := NewCache(time.Minute * 5)

	t.Run("should return cheapest provider", func(t *testing.T) {
		bestPrice, err := cache.GetBestPrice(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if bestPrice.PricePerHour != 0.38 {
			t.Errorf("Expected price to be 0.38, got: %f", bestPrice.PricePerHour)
		}
	})
}

func TestCache_Expiry(t *testing.T) {
	ctx := context.Background()

	provider := &mockProvider{
		name: "vast.ai",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.42,
		},
	}

	providers := map[string]providers.ProviderClient{
		"vast.ai": provider,
	}

	cache := NewCache(time.Millisecond * 100)

	t.Run("should refresh cache after TTL expires", func(t *testing.T) {
		_, err := cache.GetPricing(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if provider.callCount != 1 {
			t.Errorf("Expected provider to be called once, got: %d", provider.callCount)
		}

		time.Sleep(time.Millisecond * 150)

		_, err = cache.GetPricing(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if provider.callCount != 2 {
			t.Errorf("Expected provider to be called twice after expiry, got: %d", provider.callCount)
		}
	})
}

func TestCache_GetSortedPricing(t *testing.T) {
	ctx := context.Background()

	provider1 := &mockProvider{
		name: "vast.ai",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.42,
		},
	}

	provider2 := &mockProvider{
		name: "runpod",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.38,
		},
	}

	provider3 := &mockProvider{
		name: "lambda-labs",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.45,
		},
	}

	providers := map[string]providers.ProviderClient{
		"vast.ai":     provider1,
		"runpod":      provider2,
		"lambda-labs": provider3,
	}

	cache := NewCache(time.Minute * 5)

	t.Run("should return pricing sorted by price", func(t *testing.T) {
		sortedPricing, err := cache.GetSortedPricing(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if len(sortedPricing) != 3 {
			t.Errorf("Expected 3 pricing entries, got: %d", len(sortedPricing))
		}

		if sortedPricing[0].PricePerHour != 0.38 {
			t.Errorf("Expected first price to be 0.38, got: %f", sortedPricing[0].PricePerHour)
		}

		if sortedPricing[1].PricePerHour != 0.42 {
			t.Errorf("Expected second price to be 0.42, got: %f", sortedPricing[1].PricePerHour)
		}

		if sortedPricing[2].PricePerHour != 0.45 {
			t.Errorf("Expected third price to be 0.45, got: %f", sortedPricing[2].PricePerHour)
		}
	})
}

func TestCache_ClearCache(t *testing.T) {
	ctx := context.Background()

	provider := &mockProvider{
		name: "vast.ai",
		pricing: &providers.NormalizedPricing{
			PricePerHour: 0.42,
		},
	}

	providers := map[string]providers.ProviderClient{
		"vast.ai": provider,
	}

	cache := NewCache(time.Minute * 5)

	t.Run("should clear cache and force refresh", func(t *testing.T) {
		_, err := cache.GetPricing(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if provider.callCount != 1 {
			t.Errorf("Expected provider to be called once, got: %d", provider.callCount)
		}

		cache.ClearCache()

		_, err = cache.GetPricing(ctx, providers, "RTX3090", "us-east-1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if provider.callCount != 2 {
			t.Errorf("Expected provider to be called twice after cache clear, got: %d", provider.callCount)
		}
	})
}
