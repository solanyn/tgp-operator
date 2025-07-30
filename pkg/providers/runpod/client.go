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
	// Optional: Check user credits before attempting to launch (if such query exists)
	// For now, we'll rely on the error handling in the RentSpotInstance call

	// Map GPU type to RunPod GPU type ID
	gpuTypeID, err := c.mapGPUTypeToID(req.GPUType)
	if err != nil {
		return nil, fmt.Errorf("unsupported GPU type %s: %w", req.GPUType, err)
	}

	// Generate a unique name for the pod
	podName := fmt.Sprintf("tgp-%s-%d", strings.ToLower(req.GPUType), time.Now().Unix())

	// Use TalosConfig image if available, otherwise default
	imageName := "runpod/base:3.10-cuda11.8.0-devel-ubuntu22.04"
	if req.TalosConfig != nil && req.TalosConfig.Image != "" {
		imageName = req.TalosConfig.Image
	}

	input := PodRentInterruptableInput{
		CloudType:           "ALL", // Allow any cloud type
		GpuCount:            1,     // Default to 1 GPU
		VolumeInGb:          20,    // Default storage
		ContainerDiskInGb:   10,    // Default container disk
		MinVcpuCount:        4,     // Minimum CPU cores
		MinMemoryInGb:       16,    // Minimum RAM
		GpuTypeId:           gpuTypeID,
		Name:                podName,
		ImageName:           imageName,
		Ports:               "22/tcp", // SSH port
		VolumeMountPath:     "/workspace",
		Env:                 []EnvVar{}, // No custom environment variables for now
		AllowedCudaVersions: []string{"11.8", "12.0", "12.1"},
	}

	response, err := RentSpotInstance(ctx, c.graphqlClient, input)
	if err != nil {
		// Check for billing-related errors and provide specific error messages
		errMsg := err.Error()
		if strings.Contains(strings.ToLower(errMsg), "credit") ||
			strings.Contains(strings.ToLower(errMsg), "billing") ||
			strings.Contains(strings.ToLower(errMsg), "insufficient") ||
			strings.Contains(strings.ToLower(errMsg), "balance") {
			return nil, fmt.Errorf("RunPod billing error (insufficient credits or billing issue): %w", err)
		}
		return nil, fmt.Errorf("failed to launch RunPod spot instance: %w", err)
	}

	pod := response.PodRentInterruptable

	// Validate that the pod was actually created
	if pod.Id == "" {
		return nil, fmt.Errorf("RunPod returned empty pod ID - instance may not have been created due to billing or availability issues")
	}

	// Check if the pod status indicates immediate failure
	if pod.Status == "FAILED" {
		return nil, fmt.Errorf("RunPod instance failed immediately after creation - likely due to billing or resource availability issues")
	}

	return &providers.GPUInstance{
		ID:        pod.Id,
		Status:    c.mapRunPodStatusToProviderStatus(pod.Status),
		PublicIP:  "", // Will be populated when pod is running
		CreatedAt: time.Now(),
	}, nil
}

// mapGPUTypeToID maps our generic GPU types to RunPod GPU type IDs
func (c *Client) mapGPUTypeToID(gpuType string) (string, error) {
	// These are example mappings - actual RunPod GPU type IDs need to be obtained from their API
	gpuTypeMap := map[string]string{
		providers.GPUTypeRTX4090: "NVIDIA GeForce RTX 4090",
		providers.GPUTypeH100:    "NVIDIA H100 80GB HBM3",
		providers.GPUTypeA100:    "NVIDIA A100 80GB PCIe",
		"RTX3090":                "NVIDIA GeForce RTX 3090",
		"A5000":                  "NVIDIA RTX A5000",
	}

	if typeID, exists := gpuTypeMap[gpuType]; exists {
		return typeID, nil
	}

	return "", fmt.Errorf("unsupported GPU type: %s", gpuType)
}

// mapRunPodStatusToProviderStatus maps RunPod pod status to our provider status
func (c *Client) mapRunPodStatusToProviderStatus(status string) providers.InstanceState {
	switch status {
	case "PENDING":
		return providers.InstanceStatePending
	case "RUNNING":
		return providers.InstanceStateRunning
	case "STOPPED":
		return providers.InstanceStateTerminated
	case "EXITED":
		return providers.InstanceStateTerminated
	case "FAILED":
		return providers.InstanceStateFailed
	default:
		return providers.InstanceStateUnknown
	}
}

func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	response, err := GetPod(ctx, c.graphqlClient, instanceID)
	if err != nil {
		// Check for billing-related errors in status queries
		errMsg := err.Error()
		var message string
		if strings.Contains(strings.ToLower(errMsg), "credit") ||
			strings.Contains(strings.ToLower(errMsg), "billing") ||
			strings.Contains(strings.ToLower(errMsg), "insufficient") ||
			strings.Contains(strings.ToLower(errMsg), "balance") {
			message = fmt.Sprintf("RunPod billing error: %v", err)
		} else if strings.Contains(strings.ToLower(errMsg), "not found") {
			message = fmt.Sprintf("Pod not found (may have been terminated due to billing issues): %v", err)
		} else {
			message = fmt.Sprintf("Failed to get pod status: %v", err)
		}

		return &providers.InstanceStatus{
			State:     providers.InstanceStateFailed,
			Message:   message,
			UpdatedAt: time.Now(),
		}, nil
	}

	podStatus := response.Pod.Status
	state := c.mapRunPodStatusToProviderStatus(podStatus)

	// Add additional context for failed states
	message := fmt.Sprintf("Pod status: %s", podStatus)
	if state == providers.InstanceStateFailed {
		message = fmt.Sprintf("Pod failed (status: %s) - check RunPod console for billing or resource issues", podStatus)
	}

	return &providers.InstanceStatus{
		State:     state,
		Message:   message,
		UpdatedAt: time.Now(),
	}, nil
}

func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	input := PodTerminateInput{
		PodId: instanceID,
	}

	response, err := TerminatePod(ctx, c.graphqlClient, input)
	if err != nil {
		return fmt.Errorf("failed to terminate RunPod instance %s: %w", instanceID, err)
	}

	// Check if the termination was successful by verifying the response
	if response.PodTerminate.Id != instanceID {
		return fmt.Errorf("termination response mismatch: expected pod ID %s, got %s", instanceID, response.PodTerminate.Id)
	}

	return nil
}

// IsBillingError checks if an error is related to billing/credits
func (c *Client) IsBillingError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "credit") ||
		strings.Contains(errMsg, "billing") ||
		strings.Contains(errMsg, "insufficient") ||
		strings.Contains(errMsg, "balance")
}

// IsRetryableError checks if an error should be retried
func (c *Client) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	// Retry on temporary network issues, rate limiting, etc.
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "temporarily unavailable") ||
		strings.Contains(errMsg, "service unavailable")
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
