package vast

import (
	"context"
	"fmt"
	"time"

	tgpv1 "github.com/solanyn/tgp-operator/pkg/api/v1"
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

func (c *Client) GetProviderName() string {
	return "vast.ai"
}

func (c *Client) ListOffers(ctx context.Context, gpuType, region string) ([]providers.GPUOffer, error) {
	return []providers.GPUOffer{
		{
			ID:          fmt.Sprintf("vast-offer-%s-%s", gpuType, region),
			Provider:    "vast.ai",
			GPUType:     gpuType,
			Region:      region,
			HourlyPrice: 0.42,
			Memory:      24,
			Available:   true,
		},
	}, nil
}

func (c *Client) LaunchInstance(ctx context.Context, spec tgpv1.GPURequestSpec) (*providers.GPUInstance, error) {
	return &providers.GPUInstance{
		ID:        fmt.Sprintf("vast-instance-%s", spec.GPUType),
		Status:    providers.InstanceStatePending,
		CreatedAt: time.Now(),
	}, nil
}

func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	return &providers.InstanceStatus{
		State:     providers.InstanceStateRunning,
		UpdatedAt: time.Now(),
	}, nil
}

func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	return nil
}

func (c *Client) GetPricing(ctx context.Context, gpuType, region string) (*providers.PricingInfo, error) {
	return &providers.PricingInfo{
		GPUType:     gpuType,
		Region:      region,
		HourlyPrice: 0.42,
		Currency:    "USD",
		LastUpdated: time.Now(),
	}, nil
}
