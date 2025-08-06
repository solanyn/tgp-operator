package vultr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vultr/govultr/v3"
	"golang.org/x/oauth2"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

const (
	ProviderName = "vultr"
	BaseURL      = "https://api.vultr.com/v2"
)

type Client struct {
	client *govultr.Client
	apiKey string
}

func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	config := &oauth2.Config{}
	ctx := context.Background()
	ts := config.TokenSource(ctx, &oauth2.Token{AccessToken: apiKey})
	vultrClient := govultr.NewClient(oauth2.NewClient(ctx, ts))

	return &Client{
		client: vultrClient,
		apiKey: apiKey,
	}, nil
}

func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	plan, err := c.findBestPlan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to find suitable plan: %w", err)
	}

	instanceReq := &govultr.InstanceCreateReq{
		Region:   req.Region,
		Plan:     plan.ID,
		OsID:     2284, // Talos Linux OS ID
		Label:    fmt.Sprintf("tgp-%s", req.GPUType),
		UserData: req.UserData,
	}

	instance, _, err := c.client.Instance.Create(ctx, instanceReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vultr instance: %w", err)
	}

	createdAt, _ := time.Parse("2006-01-02T15:04:05-07:00", instance.DateCreated)

	return &providers.GPUInstance{
		ID:        instance.ID,
		PublicIP:  instance.MainIP,
		PrivateIP: instance.InternalIP,
		Status:    c.mapInstanceStatus(instance.Status),
		CreatedAt: createdAt,
	}, nil
}

func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	err := c.client.Instance.Delete(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to delete Vultr instance %s: %w", instanceID, err)
	}
	return nil
}

func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	instance, _, err := c.client.Instance.Get(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Vultr instance %s: %w", instanceID, err)
	}

	return &providers.InstanceStatus{
		State:     c.mapInstanceStatus(instance.Status),
		PublicIP:  instance.MainIP,
		PrivateIP: instance.InternalIP,
		UpdatedAt: time.Now(),
		Message:   instance.Status,
	}, nil
}

func (c *Client) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	options := &govultr.ListOptions{}
	plans, _, _, err := c.client.Plan.List(ctx, "gpu", options)
	if err != nil {
		return nil, fmt.Errorf("failed to list GPU plans: %w", err)
	}

	var offers []providers.GPUOffer
	for _, plan := range plans {
		gpuType, gpuCount := c.extractGPUFromPlan(&plan)
		if gpuType == "" {
			continue
		}

		if filters.GPUType != "" && !strings.EqualFold(gpuType, filters.GPUType) {
			continue
		}

		if filters.Region != "" && !c.isPlanAvailableInRegion(&plan, filters.Region) {
			continue
		}

		hourlyPrice := c.calculateHourlyPrice(plan.MonthlyCost)
		if filters.MaxPrice > 0 && hourlyPrice > filters.MaxPrice {
			continue
		}

		offer := providers.GPUOffer{
			ID:          plan.ID,
			GPUType:     gpuType,
			GPUCount:    gpuCount,
			Region:      filters.Region,
			HourlyPrice: hourlyPrice,
			Memory:      int64(plan.RAM / 1024), // Convert MB to GB
			Storage:     int64(plan.Disk),
			Available:   true,
			Provider:    ProviderName,
		}

		offers = append(offers, offer)
	}

	return offers, nil
}

func (c *Client) GetNormalizedPricing(ctx context.Context, gpuType, region string) (*providers.NormalizedPricing, error) {
	filters := &providers.GPUFilters{
		GPUType: gpuType,
		Region:  region,
	}

	offers, err := c.ListAvailableGPUs(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing: %w", err)
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no pricing available for %s in region %s", gpuType, region)
	}

	// Find the cheapest offer
	bestOffer := offers[0]
	for _, offer := range offers[1:] {
		if offer.HourlyPrice < bestOffer.HourlyPrice {
			bestOffer = offer
		}
	}

	return &providers.NormalizedPricing{
		PricePerSecond: bestOffer.HourlyPrice / 3600,
		PricePerHour:   bestOffer.HourlyPrice,
		Currency:       "USD",
		BillingModel:   providers.BillingPerHour,
		LastUpdated:    time.Now(),
	}, nil
}

func (c *Client) GetProviderInfo() *providers.ProviderInfo {
	return &providers.ProviderInfo{
		Name:                  ProviderName,
		APIVersion:            "v2",
		SupportedGPUTypes:     []string{"H100", "L40S", "A100", "A40", "A16", "MI325X", "MI300X"},
		SupportsSpotInstances: false,
		SupportsMultiGPU:      true,
		BillingGranularity:    providers.BillingPerHour,
		MinBillingPeriod:      time.Hour,
	}
}

func (c *Client) GetRateLimits() *providers.RateLimitInfo {
	return &providers.RateLimitInfo{
		RequestsPerSecond: 30,
		BurstCapacity:     100,
		BackoffStrategy:   "exponential",
		ResetWindow:       time.Second,
	}
}

func (c *Client) TranslateGPUType(standard string) (string, error) {
	gpuMapping := map[string]string{
		"H100":   "H100",
		"L40S":   "L40S",
		"A100":   "A100",
		"A40":    "A40",
		"A16":    "A16",
		"MI325X": "MI325X",
		"MI300X": "MI300X",
	}

	if vultrType, exists := gpuMapping[strings.ToUpper(standard)]; exists {
		return vultrType, nil
	}
	return "", fmt.Errorf("unsupported GPU type: %s", standard)
}

func (c *Client) TranslateRegion(standard string) (string, error) {
	// Vultr uses region IDs like "ewr" (New Jersey), "lax" (Los Angeles), etc.
	// This would need to be implemented based on actual region mappings
	return standard, nil
}

func (c *Client) findBestPlan(ctx context.Context, req *providers.LaunchRequest) (*govultr.Plan, error) {
	options := &govultr.ListOptions{}
	plans, _, _, err := c.client.Plan.List(ctx, "gpu", options)
	if err != nil {
		return nil, fmt.Errorf("failed to list GPU plans: %w", err)
	}

	var bestPlan *govultr.Plan
	for _, plan := range plans {
		gpuType, _ := c.extractGPUFromPlan(&plan)
		if !strings.EqualFold(gpuType, req.GPUType) {
			continue
		}

		if !c.isPlanAvailableInRegion(&plan, req.Region) {
			continue
		}

		if bestPlan == nil || plan.MonthlyCost < bestPlan.MonthlyCost {
			bestPlan = &plan
		}
	}

	if bestPlan == nil {
		return nil, fmt.Errorf("no suitable GPU plan found for %s in region %s", req.GPUType, req.Region)
	}

	return bestPlan, nil
}

func (c *Client) extractGPUFromPlan(plan *govultr.Plan) (string, int) {
	// Use GPUType field if available, otherwise try to parse from Type
	if plan.GPUType != "" {
		return plan.GPUType, 1
	}
	
	planType := strings.ToUpper(plan.Type)

	if strings.Contains(planType, "H100") {
		return "H100", 1
	}
	if strings.Contains(planType, "L40S") {
		return "L40S", 1
	}
	if strings.Contains(planType, "A100") {
		return "A100", 1
	}
	if strings.Contains(planType, "A40") {
		return "A40", 1
	}
	if strings.Contains(planType, "A16") {
		return "A16", 1
	}
	if strings.Contains(planType, "MI325X") {
		return "MI325X", 1
	}
	if strings.Contains(planType, "MI300X") {
		return "MI300X", 1
	}

	return "", 0
}

func (c *Client) calculateHourlyPrice(monthlyCost float32) float64 {
	return float64(monthlyCost) / 730.0
}

func (c *Client) isPlanAvailableInRegion(plan *govultr.Plan, region string) bool {
	if region == "" {
		return true
	}

	for _, availableRegion := range plan.Locations {
		if availableRegion == region {
			return true
		}
	}
	return false
}

func (c *Client) mapInstanceStatus(status string) providers.InstanceState {
	switch strings.ToLower(status) {
	case "active", "running":
		return providers.InstanceStateRunning
	case "pending", "installing":
		return providers.InstanceStatePending
	case "stopped", "halted":
		return providers.InstanceStateTerminated
	case "resizing":
		return providers.InstanceStatePending
	default:
		return providers.InstanceStateUnknown
	}
}