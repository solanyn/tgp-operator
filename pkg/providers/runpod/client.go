package runpod

import (
	"context"
	"fmt"
	"time"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

type Client struct {
	*providers.BaseProvider
	apiKey string
}

func NewClient(apiKey string) *Client {
	info := &providers.ProviderInfo{
		Name:                  "runpod",
		APIVersion:            "v1",
		SupportedRegions:      []string{providers.RegionUSEast, providers.RegionUSWest},
		SupportedGPUTypes:     []string{providers.GPUTypeRTX4090, providers.GPUTypeH100, providers.GPUTypeA100},
		SupportsSpotInstances: true,
		BillingGranularity:    providers.BillingPerSecond,
	}

	rateLimits := &providers.RateLimitInfo{
		RequestsPerSecond: 20,
		RequestsPerMinute: 1000,
		BurstCapacity:     50,
	}

	return &Client{
		BaseProvider: providers.NewBaseProvider(info, rateLimits),
		apiKey:       apiKey,
	}
}

func (c *Client) TranslateGPUType(standard string) (string, error) {
	translations := map[string]string{
		providers.GPUTypeRTX4090: "NVIDIA GeForce RTX 4090",
		providers.GPUTypeH100:    "NVIDIA H100",
		providers.GPUTypeA100:    "NVIDIA A100",
	}
	if translated, ok := translations[standard]; ok {
		return translated, nil
	}
	return "", fmt.Errorf("unsupported GPU type: %s", standard)
}

func (c *Client) TranslateRegion(standard string) (string, error) {
	translations := map[string]string{
		providers.RegionUSEast: "US-CA-1",
		providers.RegionUSWest: "US-TX-1",
	}
	if translated, ok := translations[standard]; ok {
		return translated, nil
	}
	return "", fmt.Errorf("unsupported region: %s", standard)
}

func (c *Client) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	return []providers.GPUOffer{
		{
			ID:          "runpod-offer-123",
			Provider:    "runpod",
			GPUType:     filters.GPUType,
			Region:      filters.Region,
			HourlyPrice: 0.38,
			Memory:      24,
			Storage:     100,
			Available:   true,
		},
	}, nil
}

func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	return &providers.GPUInstance{
		ID:        fmt.Sprintf("runpod-%d", time.Now().Unix()),
		Status:    providers.InstanceStatePending,
		PublicIP:  "",
		CreatedAt: time.Now(),
	}, nil
}

func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	return &providers.InstanceStatus{
		State:     providers.InstanceStateRunning,
		Message:   "Instance is running normally",
		UpdatedAt: time.Now(),
	}, nil
}

func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (c *Client) GetNormalizedPricing(ctx context.Context, gpuType, region string) (*providers.NormalizedPricing, error) {
	return &providers.NormalizedPricing{
		PricePerSecond: 0.38 / 3600,
		PricePerHour:   0.38,
		Currency:       "USD",
		BillingModel:   providers.BillingPerSecond,
		LastUpdated:    time.Now(),
	}, nil
}
