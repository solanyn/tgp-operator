package runpod

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
	return "runpod"
}

func (c *Client) ListOffers(ctx context.Context, gpuType string, region string) ([]providers.GPUOffer, error) {
	return []providers.GPUOffer{
		{
			ID:          "runpod-offer-123",
			Provider:    "runpod",
			GPUType:     gpuType,
			Region:      region,
			HourlyPrice: 0.38,
			Memory:      24,
			Storage:     100,
			Available:   true,
		},
	}, nil
}

func (c *Client) LaunchInstance(ctx context.Context, spec tgpv1.GPURequestSpec) (*providers.GPUInstance, error) {
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

func (c *Client) GetPricing(ctx context.Context, gpuType string, region string) (*providers.PricingInfo, error) {
	return &providers.PricingInfo{
		GPUType:     gpuType,
		Region:      region,
		HourlyPrice: 0.38,
		Currency:    "USD",
		LastUpdated: time.Now(),
	}, nil
}
