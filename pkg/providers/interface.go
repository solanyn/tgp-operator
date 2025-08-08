package providers

import (
	"context"
	"time"

	v1 "github.com/solanyn/tgp-operator/pkg/api/v1"
)

// ProviderClient defines the interface for cloud GPU providers
type ProviderClient interface {
	// Core lifecycle operations
	LaunchInstance(ctx context.Context, req *LaunchRequest) (*GPUInstance, error)
	TerminateInstance(ctx context.Context, instanceID string) error
	GetInstanceStatus(ctx context.Context, instanceID string) (*InstanceStatus, error)

	// Discovery and pricing with normalization
	ListAvailableGPUs(ctx context.Context, filters *GPUFilters) ([]GPUOffer, error)
	GetNormalizedPricing(ctx context.Context, gpuType, region string) (*NormalizedPricing, error)

	// Provider metadata and capabilities
	GetProviderInfo() *ProviderInfo
	GetRateLimits() *RateLimitInfo

	// Resource translation between standard and provider-specific names
	TranslateGPUType(standard string) (providerSpecific string, err error)
	TranslateRegion(standard string) (providerSpecific string, err error)
}

// LaunchRequest contains all parameters needed to launch an instance
type LaunchRequest struct {
	GPUType      string
	Region       string
	Image        string
	UserData     string
	Labels       map[string]string
	SpotInstance bool
	MaxPrice     float64 // Per hour in USD
	TalosConfig  *v1.TalosConfig
}

type GPUFilters struct {
	GPUType         string
	Region          string
	MaxPrice        float64
	MinMemory       int64
	MinStorage      int64
	SpotOnly        bool
	OnDemandOnly    bool
	PreferredVendor string
	WorkloadType    string
}

// NormalizedPricing provides standardized pricing across providers
type NormalizedPricing struct {
	PricePerSecond   float64
	PricePerHour     float64
	Currency         string
	BillingModel     BillingModel
	StorageCost      float64 // Per GB per hour
	NetworkCost      float64 // Per GB transfer
	LastUpdated      time.Time
	ProviderSpecific map[string]interface{} // Provider-specific pricing details
}

// BillingModel represents how providers charge for usage
type BillingModel string

const (
	BillingPerSecond BillingModel = "per-second"
	BillingPerMinute BillingModel = "per-minute"
	BillingPerHour   BillingModel = "per-hour"
)

// ProviderInfo contains metadata about provider capabilities
type ProviderInfo struct {
	Name                  string
	APIVersion            string
	SupportedRegions      []string
	SupportedGPUTypes     []string
	SupportsSpotInstances bool
	SupportsMultiGPU      bool
	BillingGranularity    BillingModel
	MinBillingPeriod      time.Duration
}

// RateLimitInfo contains rate limiting information for the provider
type RateLimitInfo struct {
	RequestsPerSecond int
	RequestsPerMinute int
	BurstCapacity     int
	BackoffStrategy   string
	ResetWindow       time.Duration
}

// GPUInstance represents a provisioned GPU instance
type GPUInstance struct {
	ID        string
	PublicIP  string
	PrivateIP string
	Status    InstanceState
	CreatedAt time.Time
}

// InstanceStatus represents the current status of an instance
type InstanceStatus struct {
	State     InstanceState
	PublicIP  string
	PrivateIP string
	UpdatedAt time.Time
	Message   string
}

// InstanceState represents the state of a GPU instance
type InstanceState string

const (
	InstanceStatePending     InstanceState = "pending"
	InstanceStateRunning     InstanceState = "running"
	InstanceStateTerminating InstanceState = "terminating"
	InstanceStateTerminated  InstanceState = "terminated"
	InstanceStateFailed      InstanceState = "failed"
	InstanceStateUnknown     InstanceState = "unknown"
)

const (
	GPUTypeRTX4090 = "RTX4090"
	GPUTypeRTX3090 = "RTX3090"
	GPUTypeH100    = "H100"
	GPUTypeA100    = "A100"
	GPUTypeV100    = "V100"
)

const (
	ResourceTGPGPU    = "tgp.io/gpu"
	ResourceTGPMemory = "tgp.io/memory"
)

const (
	AnnotationVendor   = "tgp.io/vendor"
	AnnotationWorkload = "tgp.io/workload"
)

// Standard regions for translation
const (
	RegionUSEast      = "us-east"
	RegionUSWest      = "us-west"
	RegionEUCentral   = "eu-central"
	RegionAsiaPacific = "asia-pacific"
)

// GPUOffer represents an available GPU offer from a provider
type GPUOffer struct {
	ID          string
	GPUType     string
	GPUCount    int
	Region      string
	HourlyPrice float64
	SpotPrice   float64
	Memory      int64 // GB
	Storage     int64 // GB
	Bandwidth   int64 // Mbps
	IsSpot      bool
	Available   bool
	Provider    string
}

// ProviderCredentials contains authentication credentials for a provider
type ProviderCredentials struct {
	APIKey    string
	APISecret string
	Token     string
	Endpoint  string
}
