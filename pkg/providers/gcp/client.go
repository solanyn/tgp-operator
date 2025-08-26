package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/solanyn/tgp-operator/pkg/providers"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
)

// Client implements the ProviderClient interface for Google Cloud Platform
type Client struct {
	projectID     string
	credentials   string
	computeClient *compute.InstancesClient
	machineClient *compute.MachineTypesClient
	imagesClient  *compute.ImagesClient
	regionsClient *compute.RegionsClient
}

// ServiceAccountKey represents the structure of a GCP service account JSON key
type ServiceAccountKey struct {
	Type          string `json:"type"`
	ProjectID     string `json:"project_id"`
	PrivateKeyID  string `json:"private_key_id"`
	PrivateKey    string `json:"private_key"`
	ClientEmail   string `json:"client_email"`
	ClientID      string `json:"client_id"`
	AuthURI       string `json:"auth_uri"`
	TokenURI      string `json:"token_uri"`
	AuthProvider  string `json:"auth_provider_x509_cert_url"`
	ClientCertURL string `json:"client_x509_cert_url"`
}

// NewClient creates a new GCP provider client
func NewClient(credentialsJSON string) *Client {
	return &Client{
		credentials: credentialsJSON,
	}
}

// Initialize sets up the GCP client with proper authentication
func (c *Client) Initialize(ctx context.Context) error {
	// Parse service account key to get project ID
	var serviceAccount ServiceAccountKey
	if err := json.Unmarshal([]byte(c.credentials), &serviceAccount); err != nil {
		return fmt.Errorf("failed to parse service account JSON: %w", err)
	}
	c.projectID = serviceAccount.ProjectID

	// Set up client options
	opts := []option.ClientOption{
		option.WithCredentialsJSON([]byte(c.credentials)),
	}

	// Initialize compute clients
	var err error
	c.computeClient, err = compute.NewInstancesRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create instances client: %w", err)
	}

	c.machineClient, err = compute.NewMachineTypesRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create machine types client: %w", err)
	}

	c.imagesClient, err = compute.NewImagesRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create images client: %w", err)
	}

	c.regionsClient, err = compute.NewRegionsRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create regions client: %w", err)
	}

	return nil
}

// GetProviderInfo returns information about the GCP provider
func (c *Client) GetProviderInfo() *providers.ProviderInfo {
	return &providers.ProviderInfo{
		Name:       "gcp",
		APIVersion: "v1",
		SupportedRegions: []string{
			"us-central1", "us-east1", "us-east4", "us-west1", "us-west2", "us-west3", "us-west4",
			"europe-west1", "europe-west2", "europe-west3", "europe-west4", "europe-west6",
			"asia-east1", "asia-northeast1", "asia-southeast1", "australia-southeast1",
		},
		SupportedGPUTypes: []string{
			"NVIDIA_K80", "NVIDIA_P4", "NVIDIA_P100",
			"NVIDIA_V100", "NVIDIA_T4", "NVIDIA_A100",
			"NVIDIA_A100_80GB", "NVIDIA_H100_80GB", "NVIDIA_L4",
		},
		SupportsSpotInstances: true,
		BillingGranularity:    "per-minute",
	}
}

// GetRateLimits returns the rate limits for the GCP API
func (c *Client) GetRateLimits() *providers.RateLimitInfo {
	return &providers.RateLimitInfo{
		RequestsPerSecond: 20,   // Conservative estimate
		RequestsPerMinute: 1000, // GCP has generous limits
		BurstCapacity:     50,
	}
}

// ListAvailableGPUs returns available GPU instances matching the filters
func (c *Client) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	var offers []providers.GPUOffer

	regions := c.getRegionsToSearch(filters.Region)

	for _, region := range regions {
		zones := c.getZonesForRegion(region)
		for _, zone := range zones {
			zoneOffers, err := c.getGPUOffersForZone(ctx, zone, filters)
			if err != nil {
				// Log but don't fail for individual zone errors
				continue
			}
			for _, offer := range zoneOffers {
				offers = append(offers, *offer)
			}
		}
	}

	return c.filterOffers(offers, filters), nil
}

// GetNormalizedPricing returns pricing information for a specific GPU type and region
func (c *Client) GetNormalizedPricing(ctx context.Context, gpuType, region string) (*providers.NormalizedPricing, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	// Get base instance pricing (e.g., n1-standard-4 with GPU)
	machineType := c.getRecommendedMachineTypeForGPU(gpuType)
	machinePrice := c.getMachinePricing(machineType, region)

	// Get GPU pricing
	gpuPrice := c.getGPUPricing(gpuType, region)

	totalHourlyPrice := machinePrice + gpuPrice

	return &providers.NormalizedPricing{
		PricePerHour:   totalHourlyPrice,
		PricePerSecond: totalHourlyPrice / 3600,
		Currency:       "USD",
		BillingModel:   providers.BillingPerMinute,
		LastUpdated:    time.Now(),
	}, nil
}

// LaunchInstance creates a new GPU instance
func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	// Generate instance name
	instanceName := c.generateInstanceName(req)
	zone := c.selectBestZone(req.Region, req.GPUType)

	// Build instance configuration
	instance := &computepb.Instance{
		Name:              proto.String(instanceName),
		MachineType:       proto.String(c.getMachineTypeURL(c.getRecommendedMachineTypeForGPU(req.GPUType), zone)),
		Labels:            c.buildLabels(req),
		Metadata:          c.buildMetadata(req),
		Disks:             c.buildDiskConfig(),
		NetworkInterfaces: c.buildNetworkConfig(),
		ServiceAccounts:   c.buildServiceAccountConfig(),
		GuestAccelerators: c.buildGPUConfig(req.GPUType, 1),
		Scheduling: &computepb.Scheduling{
			Preemptible: proto.Bool(req.SpotInstance),
		},
	}

	// Launch the instance
	op, err := c.computeClient.Insert(ctx, &computepb.InsertInstanceRequest{
		Project:          c.projectID,
		Zone:             zone,
		InstanceResource: instance,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch instance: %w", err)
	}

	// Wait for operation to complete
	if err := c.waitForZoneOperation(ctx, op.Name(), zone); err != nil {
		return nil, fmt.Errorf("instance launch failed: %w", err)
	}

	// Get the created instance details
	createdInstance, err := c.computeClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  c.projectID,
		Zone:     zone,
		Instance: instanceName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get created instance: %w", err)
	}

	return c.instanceToGPUInstance(createdInstance, zone), nil
}

// TerminateInstance destroys an existing instance
func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	if err := c.ensureInitialized(ctx); err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	zone, instanceName := c.parseInstanceID(instanceID)

	op, err := c.computeClient.Delete(ctx, &computepb.DeleteInstanceRequest{
		Project:  c.projectID,
		Zone:     zone,
		Instance: instanceName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	return c.waitForZoneOperation(ctx, op.Name(), zone)
}

// GetInstanceStatus returns the current status of an instance
func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	zone, instanceName := c.parseInstanceID(instanceID)

	instance, err := c.computeClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  c.projectID,
		Zone:     zone,
		Instance: instanceName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	return &providers.InstanceStatus{
		State:     c.translateInstanceState(instance.GetStatus()),
		PublicIP:  c.extractPublicIP(instance),
		PrivateIP: c.extractPrivateIP(instance),
		UpdatedAt: time.Now(),
	}, nil
}

// ensureInitialized checks if the client is initialized and initializes if needed
func (c *Client) ensureInitialized(ctx context.Context) error {
	if c.computeClient == nil {
		return c.Initialize(ctx)
	}
	return nil
}

// generateInstanceName creates a unique name for the instance
func (c *Client) generateInstanceName(req *providers.LaunchRequest) string {
	timestamp := time.Now().Unix()
	nodepool := "default"
	if pool, ok := req.Labels["nodepool"]; ok {
		nodepool = pool
	}

	// GCP instance names must be lowercase and start with letter
	name := fmt.Sprintf("tgp-%s-%d", strings.ToLower(nodepool), timestamp)

	// Ensure name is valid (max 63 chars, only lowercase letters, numbers, hyphens)
	if len(name) > 63 {
		name = name[:63]
	}

	return strings.Trim(name, "-")
}

// Close cleans up the client connections
func (c *Client) Close() error {
	var errs []error

	if c.computeClient != nil {
		if err := c.computeClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if c.machineClient != nil {
		if err := c.machineClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if c.imagesClient != nil {
		if err := c.imagesClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if c.regionsClient != nil {
		if err := c.regionsClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing clients: %v", errs)
	}

	return nil
}

// TranslateGPUType translates standard GPU types to GCP accelerator types
func (c *Client) TranslateGPUType(standard string) (string, error) {
	return c.translateGPUTypeToGCP(standard), nil
}

// TranslateRegion translates standard regions to GCP regions
func (c *Client) TranslateRegion(standard string) (string, error) {
	// GCP regions are used directly, no translation needed
	return standard, nil
}
