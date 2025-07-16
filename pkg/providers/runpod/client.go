// Package runpod provides RunPod cloud GPU provider implementation for the TGP operator
package runpod

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

const (
	runpodGraphQLEndpoint = "https://api.runpod.io/graphql"
)

// authHTTPClient wraps an HTTP client to add authentication headers
type authHTTPClient struct {
	client *http.Client
	apiKey string
}

func (a *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return a.client.Do(req)
}

type Client struct {
	*providers.BaseProvider
	apiKey        string
	httpClient    *http.Client
	baseURL       string
	graphqlClient graphql.Client
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

	// Create GraphQL client with authentication
	baseHTTPClient := &http.Client{Timeout: 30 * time.Second}
	authClient := &authHTTPClient{baseHTTPClient, apiKey}
	graphqlClient := graphql.NewClient(runpodGraphQLEndpoint, authClient)

	return &Client{
		BaseProvider:  providers.NewBaseProvider(info, rateLimits),
		apiKey:        apiKey,
		httpClient:    baseHTTPClient,
		baseURL:       runpodGraphQLEndpoint,
		graphqlClient: graphqlClient,
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
	// Use the generated GraphQL client
	resp, err := ListGPUTypes(ctx, c.graphqlClient)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GPU types: %w", err)
	}

	var offers []providers.GPUOffer
	for _, gpu := range resp.GpuTypes {
		// Filter by GPU type if specified
		if filters != nil && filters.GPUType != "" {
			standardGPU := c.translateGPUTypeToStandard(gpu.DisplayName)
			if standardGPU != filters.GPUType {
				continue
			}
		}

		// Use spot price if available and requested, otherwise use regular price
		spotPrice := gpu.CommunitySpotPrice
		regularPrice := gpu.CommunityPrice

		// Filter by price if specified
		if filters != nil && filters.MaxPrice > 0 {
			if regularPrice > filters.MaxPrice && spotPrice > filters.MaxPrice {
				continue
			}
		}

		// Create offer for spot instances
		if spotPrice > 0 {
			offers = append(offers, providers.GPUOffer{
				ID:          fmt.Sprintf("runpod-spot-%s", gpu.Id),
				Provider:    "runpod",
				GPUType:     c.translateGPUTypeToStandard(gpu.DisplayName),
				GPUCount:    1,
				Region:      providers.RegionUSEast, // Default region
				HourlyPrice: regularPrice,
				SpotPrice:   spotPrice,
				Memory:      int64(gpu.MemoryInGb),
				Storage:     50, // Default storage
				IsSpot:      true,
				Available:   true,
			})
		}

		// Create offer for on-demand instances
		if regularPrice > 0 {
			offers = append(offers, providers.GPUOffer{
				ID:          fmt.Sprintf("runpod-ondemand-%s", gpu.Id),
				Provider:    "runpod",
				GPUType:     c.translateGPUTypeToStandard(gpu.DisplayName),
				GPUCount:    1,
				Region:      providers.RegionUSEast, // Default region
				HourlyPrice: regularPrice,
				SpotPrice:   0,
				Memory:      int64(gpu.MemoryInGb),
				Storage:     50, // Default storage
				IsSpot:      false,
				Available:   true,
			})
		}
	}

	return offers, nil
}

// translateGPUTypeToStandard converts RunPod GPU names to standard names
func (c *Client) translateGPUTypeToStandard(runpodType string) string {
	// Convert RunPod GPU names to our standard names
	switch {
	case strings.Contains(strings.ToLower(runpodType), "rtx 4090"):
		return providers.GPUTypeRTX4090
	case strings.Contains(strings.ToLower(runpodType), "h100"):
		return providers.GPUTypeH100
	case strings.Contains(strings.ToLower(runpodType), "a100"):
		return providers.GPUTypeA100
	default:
		return runpodType // Return as-is if no match
	}
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
