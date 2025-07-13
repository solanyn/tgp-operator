package providers

import (
	"context"
	"time"

	v1 "github.com/solanyn/tgp-operator/pkg/api/v1"
)

// ProviderClient defines the interface for cloud GPU providers
type ProviderClient interface {
	// LaunchInstance provisions a new GPU instance
	LaunchInstance(ctx context.Context, spec v1.GPURequestSpec) (*GPUInstance, error)

	// TerminateInstance terminates an existing GPU instance
	TerminateInstance(ctx context.Context, instanceID string) error

	// GetInstanceStatus retrieves the current status of an instance
	GetInstanceStatus(ctx context.Context, instanceID string) (*InstanceStatus, error)

	// GetPricing retrieves current pricing for a GPU type in a region
	GetPricing(ctx context.Context, gpuType, region string) (*PricingInfo, error)

	// ListOffers lists available GPU offers matching criteria
	ListOffers(ctx context.Context, gpuType, region string) ([]GPUOffer, error)

	// GetProviderName returns the name of this provider
	GetProviderName() string
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

// PricingInfo contains pricing information for a GPU type
type PricingInfo struct {
	GPUType     string
	Region      string
	HourlyPrice float64
	SpotPrice   float64
	Currency    string
	LastUpdated time.Time
}

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

// LaunchConfig contains configuration for launching an instance
type LaunchConfig struct {
	Image      string
	UserData   string
	SSHKeyName string
	Labels     map[string]string
	Metadata   map[string]string
}
