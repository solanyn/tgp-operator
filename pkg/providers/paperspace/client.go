package paperspace

import (
	"context"
	"fmt"
	"time"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

type Client struct {
	apiKey string
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
	}
}

func (c *Client) GetProviderInfo() *providers.ProviderInfo {
	return &providers.ProviderInfo{Name: "paperspace"}
}

func (c *Client) GetRateLimits() *providers.RateLimitInfo {
	return &providers.RateLimitInfo{RequestsPerSecond: 10}
}

func (c *Client) TranslateGPUType(standard string) (string, error) {
	return standard, nil
}

func (c *Client) TranslateRegion(standard string) (string, error) {
	return standard, nil
}

func (c *Client) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	return []providers.GPUOffer{}, nil
}

func (c *Client) ListOffers(ctx context.Context, gpuType string, region string) ([]providers.GPUOffer, error) {
	return []providers.GPUOffer{
		{
			ID:          "paperspace-offer-123",
			Provider:    "paperspace",
			GPUType:     gpuType,
			Region:      region,
			HourlyPrice: 0.51,
			Memory:      24,
			Storage:     50,
			Available:   true,
		},
	}, nil
}

func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	return &providers.GPUInstance{
		ID:        fmt.Sprintf("paperspace-%d", time.Now().Unix()),
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

func (c *Client) GetNormalizedPricing(ctx context.Context, gpuType string, region string) (*providers.NormalizedPricing, error) {
	return &providers.NormalizedPricing{
		PricePerSecond: 0.51 / 3600,
		PricePerHour:   0.51,
		Currency:       "USD",
		BillingModel:   providers.BillingPerSecond,
		LastUpdated:    time.Now(),
	}, nil
}
