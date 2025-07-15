// Package vast provides Vast.ai cloud GPU provider implementation for the TGP operator
package vast

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

// Client implements the ProviderClient interface for Vast.ai
type Client struct {
	*providers.BaseProvider
	apiKey            string
	httpClient        *http.Client
	baseURL           string
	gpuTranslator     *providers.GPUTypeTranslator
	regionTranslator  *providers.RegionTranslator
	pricingNormalizer *providers.PricingNormalizer
}

// NewClient creates a new Vast.ai client
func NewClient(apiKey string) *Client {
	// Vast.ai provider info
	info := &providers.ProviderInfo{
		Name:                  "vast.ai",
		APIVersion:            "v1",
		SupportedRegions:      []string{providers.RegionUSEast, providers.RegionUSWest, providers.RegionEUCentral},
		SupportedGPUTypes:     []string{providers.GPUTypeRTX4090, providers.GPUTypeRTX3090, providers.GPUTypeH100, providers.GPUTypeA100},
		SupportsSpotInstances: true,
		SupportsMultiGPU:      true,
		BillingGranularity:    providers.BillingPerHour,
		MinBillingPeriod:      time.Hour,
	}

	// Rate limits based on observed Vast.ai behavior
	rateLimits := &providers.RateLimitInfo{
		RequestsPerSecond: 1, // Conservative estimate
		RequestsPerMinute: 60,
		BurstCapacity:     5,
		BackoffStrategy:   "exponential",
		ResetWindow:       time.Minute,
	}

	// GPU type mappings from standard to Vast.ai specific
	gpuMappings := map[string]string{
		providers.GPUTypeRTX4090: "RTX_4090",
		providers.GPUTypeRTX3090: "RTX_3090",
		providers.GPUTypeH100:    "H100",
		providers.GPUTypeA100:    "A100",
	}

	// Region mappings
	regionMappings := map[string]string{
		providers.RegionUSEast:    "US",
		providers.RegionUSWest:    "US",
		providers.RegionEUCentral: "EU",
	}

	return &Client{
		BaseProvider:      providers.NewBaseProvider(info, rateLimits),
		apiKey:            apiKey,
		httpClient:        &http.Client{Timeout: 30 * time.Second},
		baseURL:           "https://console.vast.ai/api/v0",
		gpuTranslator:     providers.NewGPUTypeTranslator(gpuMappings),
		regionTranslator:  providers.NewRegionTranslator(regionMappings),
		pricingNormalizer: providers.NewPricingNormalizer(providers.BillingPerHour),
	}
}

func (c *Client) TranslateGPUType(standard string) (string, error) {
	return c.gpuTranslator.Translate(standard)
}

func (c *Client) TranslateRegion(standard string) (string, error) {
	return c.regionTranslator.Translate(standard)
}

type vastOffer struct {
	ID            int     `json:"id"`
	GPUName       string  `json:"gpu_name"`
	NumGPUs       int     `json:"num_gpus"`
	DiskSpace     float64 `json:"disk_space"`
	RAMAmount     float64 `json:"ram_amount"`
	StorageCost   float64 `json:"storage_cost"`
	DpPh          float64 `json:"dph_total"`
	Reliability   float64 `json:"reliability"`
	Verified      bool    `json:"verified"`
	Available     bool    `json:"available"`
	ComputeCap    int     `json:"compute_cap"`
	DriverVersion string  `json:"driver_version"`
	CudaVersion   string  `json:"cuda_max_good"`
	Datacenter    string  `json:"datacenter"`
	CountryCode   string  `json:"geolocation"`
}

func (c *Client) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	if err := c.WaitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	var vastGPUType string
	if filters.GPUType != "" {
		translated, err := c.TranslateGPUType(filters.GPUType)
		if err != nil {
			return nil, fmt.Errorf("unsupported GPU type: %w", err)
		}
		vastGPUType = translated
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/bundles", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	if vastGPUType != "" {
		q.Add("q", fmt.Sprintf("gpu_name:%q", vastGPUType))
	}
	if filters.MaxPrice > 0 {
		q.Add("order", "dph_total")
	}
	q.Add("type", "on-demand")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var vastOffers []vastOffer
	if err := json.NewDecoder(resp.Body).Decode(&vastOffers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	offers := make([]providers.GPUOffer, 0, len(vastOffers))
	for _, vo := range vastOffers {
		if filters.MaxPrice > 0 && vo.DpPh > filters.MaxPrice {
			continue
		}
		if filters.MinMemory > 0 && int64(vo.RAMAmount) < filters.MinMemory {
			continue
		}
		if filters.MinStorage > 0 && int64(vo.DiskSpace) < filters.MinStorage {
			continue
		}
		if !vo.Available {
			continue
		}

		offers = append(offers, providers.GPUOffer{
			ID:          fmt.Sprintf("%d", vo.ID),
			GPUType:     filters.GPUType,
			GPUCount:    vo.NumGPUs,
			Region:      c.mapLocationToRegion(vo.CountryCode),
			HourlyPrice: vo.DpPh,
			Memory:      int64(vo.RAMAmount),
			Storage:     int64(vo.DiskSpace),
			Available:   vo.Available,
			Provider:    "vast.ai",
		})
	}

	return offers, nil
}

func (c *Client) GetNormalizedPricing(ctx context.Context, gpuType, region string) (*providers.NormalizedPricing, error) {
	if err := c.WaitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	filters := &providers.GPUFilters{
		GPUType: gpuType,
		Region:  region,
	}

	offers, err := c.ListAvailableGPUs(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get offers: %w", err)
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no offers available for %s in %s", gpuType, region)
	}

	cheapest := offers[0]
	for _, offer := range offers[1:] {
		if offer.HourlyPrice < cheapest.HourlyPrice {
			cheapest = offer
		}
	}

	return c.pricingNormalizer.Normalize(cheapest.HourlyPrice, "USD"), nil
}

type vastInstance struct {
	ID        int    `json:"id"`
	Status    string `json:"actual_status"`
	PublicIP  string `json:"public_ipaddr"`
	SSHHost   string `json:"ssh_host"`
	SSHPort   int    `json:"ssh_port"`
	CreatedAt string `json:"start_date"`
}

func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	if err := c.WaitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	filters := &providers.GPUFilters{
		GPUType:  req.GPUType,
		Region:   req.Region,
		MaxPrice: req.MaxPrice,
	}

	offers, err := c.ListAvailableGPUs(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to find offers: %w", err)
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no suitable offers available")
	}

	selectedOffer := offers[0]
	for _, offer := range offers[1:] {
		if offer.HourlyPrice < selectedOffer.HourlyPrice {
			selectedOffer = offer
		}
	}

	payload := map[string]interface{}{
		"client_id": "tgp-operator",
		"image":     req.Image,
		"label":     fmt.Sprintf("tgp-%s", req.GPUType),
		"disk":      10,
		"python":    "3.11",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/asks/%s/", c.baseURL, selectedOffer.ID),
		bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to launch instance: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instanceID, ok := result["new_contract"].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return &providers.GPUInstance{
		ID:        fmt.Sprintf("%.0f", instanceID),
		Status:    providers.InstanceStatePending,
		CreatedAt: time.Now(),
	}, nil
}

func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	if err := c.WaitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/instances/%s/", c.baseURL, instanceID), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get instance status: status %d", resp.StatusCode)
	}

	var instance vastInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &providers.InstanceStatus{
		State:     c.mapVastStatusToStandard(instance.Status),
		PublicIP:  instance.PublicIP,
		UpdatedAt: time.Now(),
		Message:   fmt.Sprintf("Vast.ai status: %s", instance.Status),
	}, nil
}

func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	if err := c.WaitForRateLimit(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/instances/%s/", c.baseURL, instanceID), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to terminate instance: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) mapLocationToRegion(countryCode string) string {
	switch countryCode {
	case "US", "CA":
		return providers.RegionUSEast
	case "GB", "DE", "FR", "NL":
		return providers.RegionEUCentral
	default:
		return providers.RegionUSEast
	}
}

func (c *Client) mapVastStatusToStandard(vastStatus string) providers.InstanceState {
	switch vastStatus {
	case "loading", "created":
		return providers.InstanceStatePending
	case "running":
		return providers.InstanceStateRunning
	case "exited":
		return providers.InstanceStateTerminated
	default:
		return providers.InstanceStateUnknown
	}
}
