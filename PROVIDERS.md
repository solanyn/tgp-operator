# Provider Integration Architecture

This document describes the technical architecture for integrating cloud GPU providers with the TGP operator, including CRD interactions, API client implementations, and code generation patterns.

## Architecture Overview

The TGP operator uses a two-layer architecture for provider integration:

1. **CRD Layer**: `GPUNodeClass` resources define provider configurations and credentials
2. **Client Layer**: Provider-specific clients implement the `ProviderClient` interface

## CRD-to-Provider Integration

### GPUNodeClass Configuration

The `GPUNodeClass` CRD configures providers through a declarative API:

```go
type ProviderConfig struct {
    Name           string              `json:"name"`
    Priority       int32               `json:"priority,omitempty"`
    Enabled        bool                `json:"enabled,omitempty"`
    CredentialsRef SecretReference     `json:"credentialsRef"`
    Regions        []string            `json:"regions,omitempty"`
}
```

### Controller-Provider Interaction

The `GPUNodeClassReconciler` validates provider configurations and credentials:

1. **Credential Resolution**: Fetches API keys from Kubernetes secrets
2. **Provider Instantiation**: Creates provider clients with resolved credentials
3. **Validation**: Tests provider connectivity and credential validity
4. **Status Updates**: Reports provider availability in CRD status

### GPUNodePool Provisioning Flow

The `GPUNodePoolReconciler` uses `GPUNodeClass` configurations to provision instances:

1. **Node Class Resolution**: References `GPUNodeClass` for provider configurations
2. **Provider Selection**: Chooses providers based on priority and availability
3. **Instance Provisioning**: Calls provider `LaunchInstance` methods
4. **Status Tracking**: Monitors instance lifecycle through provider `GetInstanceStatus`

### Implementation Pattern

```go
// Controller resolves providers from node class
func (r *GPUNodePoolReconciler) resolveProviders(ctx context.Context, nodeClass *v1.GPUNodeClass) (map[string]providers.ProviderClient, error) {
    clients := make(map[string]providers.ProviderClient)

    for _, providerConfig := range nodeClass.Spec.Providers {
        // Fetch credentials from secret
        credential, err := r.getProviderCredentials(ctx, &providerConfig.CredentialsRef)
        if err != nil {
            return nil, err
        }

        // Instantiate provider client
        client, err := r.createProviderClient(providerConfig.Name, credential)
        if err != nil {
            return nil, err
        }

        clients[providerConfig.Name] = client
    }

    return clients, nil
}
```

## Provider Client Interface

All provider clients implement a standardized interface for lifecycle operations:

```go
type ProviderClient interface {
    // Instance lifecycle
    LaunchInstance(ctx context.Context, req *LaunchRequest) (*GPUInstance, error)
    TerminateInstance(ctx context.Context, instanceID string) error
    GetInstanceStatus(ctx context.Context, instanceID string) (*InstanceStatus, error)

    // Resource discovery
    ListAvailableGPUs(ctx context.Context, filters *GPUFilters) ([]GPUOffer, error)
    GetNormalizedPricing(ctx context.Context, gpuType, region string) (*NormalizedPricing, error)

    // Provider capabilities
    GetProviderInfo() *ProviderInfo
    GetRateLimits() *RateLimitInfo
    TranslateGPUType(standard string) (providerSpecific string, err error)
    TranslateRegion(standard string) (providerSpecific string, err error)
}
```

### Client Implementation Patterns

Provider clients are implemented using different approaches:

1. **REST APIs**: OpenAPI code generation for type-safe clients
2. **GraphQL APIs**: Code generation from GraphQL schemas
3. **Cloud SDKs**: Official cloud provider Go SDKs

## Provider Implementation Details

### Vultr - OpenAPI Generated Client

**Implementation**: `pkg/providers/vultr/`
**API Type**: REST API with OpenAPI specification
**Base URL**: `https://api.vultr.com/v2`

#### Code Generation

The Vultr client uses OpenAPI code generation:

```bash
# Generated from OpenAPI spec
oapi-codegen -package api -generate client,types \
  vultr-openapi-spec.json > pkg/providers/vultr/api/client.go
```

#### Client Structure

```go
type Client struct {
    apiKey    string
    apiClient *api.ClientWithResponses  // Generated OpenAPI client
}

// Authentication is handled through request editor
api.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
    req.Header.Set("Authorization", "Bearer "+apiKey)
    return nil
})
```

#### Key Features

- Official Talos Linux marketplace integration
- Fractional GPU options starting at $0.059/hr
- Wide GPU range from A16 to A100
- True on-demand billing

### Google Cloud - Official SDK

**Implementation**: `pkg/providers/gcp/`
**API Type**: Official Google Cloud Go SDK
**Authentication**: Service Account JSON or Application Default Credentials

#### SDK Usage

The GCP client leverages the official Google Cloud Compute Engine SDK:

```go
import (
    compute "google.golang.org/api/compute/v1"
    "google.golang.org/api/option"
)

type Client struct {
    projectID     string
    credentials   string
    computeClient *compute.Service
}

func NewClient(credentials string) *Client {
    ctx := context.Background()
    var opts []option.ClientOption
    
    if credentials != "" {
        opts = append(opts, option.WithCredentialsJSON([]byte(credentials)))
    }
    
    service, err := compute.NewService(ctx, opts...)
    if err != nil {
        return nil
    }
    
    return &Client{
        computeClient: service,
        credentials:   credentials,
    }
}
```

#### Talos Image Management

GCP requires custom image upload for Talos Linux:

```go
func (c *Client) ensureTalosImage(ctx context.Context) error {
    // Download Talos GCP image from Image Factory
    imageURL := "https://factory.talos.dev/gcp/image.tar.gz"
    
    // Upload to Cloud Storage bucket
    // Import as compute image
    // Track for cleanup
}
```

#### Key Features

- Enterprise-grade reliability and security
- Comprehensive GPU options (A100, V100, T4, L4)
- Global region availability
- Competitive pricing with committed use discounts

### Scaleway - OpenAPI Generated Client

**Implementation**: `pkg/providers/scaleway/`
**API Type**: REST API with OpenAPI specification
**Base URL**: `https://api.scaleway.com`

#### Client Structure

```go
type Client struct {
    apiKey      string
    apiClient   *api.ClientWithResponses
    projectID   string
}

// Authentication uses X-Auth-Token header
api.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
    req.Header.Set("X-Auth-Token", apiKey)
    return nil
})
```

#### Key Features

- European data centers and GDPR compliance
- Competitive H100 and L40S pricing
- Official Talos Linux support
- Developer-friendly signup process

### DigitalOcean - Official OpenAPI Specification

**Implementation**: `pkg/providers/digitalocean/`
**API Type**: REST API with official OpenAPI specification on GitHub
**Base URL**: `https://api.digitalocean.com/v2`

#### Code Generation

DigitalOcean provides the highest quality OpenAPI specification:

```bash
# Download official spec from GitHub
curl -o digitalocean-openapi.yaml \
  https://raw.githubusercontent.com/digitalocean/openapi/main/specification/DigitalOcean-public.v2.yaml

# Generate client
oapi-codegen -package api -generate client,types \
  digitalocean-openapi.yaml > pkg/providers/digitalocean/api/client.go
```

#### Key Features

- Excellent API documentation and SDKs
- RTX 4000 ADA GPUs at $0.76/hr
- Per-second billing with 5-minute minimum
- Official Talos Linux support

## Code Generation Summary

### Implementation Approaches

| Provider      | API Type | Generation Tool | Client Pattern           | Status      |
| ------------- | -------- | --------------- | ------------------------ | ----------- |
| Vultr         | REST     | oapi-codegen    | OpenAPI specification    | 🔄 Planned  |
| Google Cloud  | SDK      | Official SDK    | Cloud SDK integration    | 🔄 Planned  |
| Scaleway      | REST     | oapi-codegen    | OpenAPI specification    | 🔄 Planned  |
| DigitalOcean  | REST     | oapi-codegen    | GitHub OpenAPI spec      | 🔄 Planned  |

### Generated Artifacts

#### Vultr OpenAPI
- `pkg/providers/vultr/api/client.go` - Generated REST client
- `pkg/providers/vultr/api/types.go` - Generated type definitions

#### Google Cloud SDK
- Uses official `google.golang.org/api/compute/v1` package
- Custom image management for Talos Linux deployment

#### Scaleway OpenAPI
- `pkg/providers/scaleway/api/client.go` - Generated REST client
- `pkg/providers/scaleway/api/types.go` - Generated type definitions

#### DigitalOcean OpenAPI
- `pkg/providers/digitalocean/api/client.go` - Generated REST client
- `pkg/providers/digitalocean/api/types.go` - Generated type definitions

## Testing and Mocking Strategy

### Test Implementation Pattern

Provider clients implement conditional behavior for testing:

```go
const (
    fakeAPIKey = "fake-api-key" // #nosec G101 -- Test constant
)

func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
    if c.apiClient == nil || c.apiKey == fakeAPIKey {
        // Return mock data for testing
        return &providers.GPUInstance{
            ID:        fmt.Sprintf("test-%d", time.Now().Unix()),
            Status:    providers.InstanceStatePending,
            CreatedAt: time.Now(),
        }, nil
    }

    // Real API implementation
    return c.callRealAPI(ctx, req)
}
```

### Integration Testing

Controllers can test with real provider APIs when credentials are available:

```go
func TestGPUNodePoolController(t *testing.T) {
    // Use real API key if available in environment
    apiKey := os.Getenv("VULTR_API_KEY")
    if apiKey == "" {
        apiKey = "fake-api-key" // Falls back to mock behavior
    }

    client := vultr.NewClient(apiKey)
    // Test with either real or mock behavior
}
```

## Error Handling and Resilience

### Provider Error Classification

Provider clients implement standardized error handling:

```go
// Common error patterns across providers
func (c *Client) classifyError(err error) ErrorType {
    errMsg := strings.ToLower(err.Error())

    // Billing/credit errors
    if strings.Contains(errMsg, "credit") || strings.Contains(errMsg, "billing") {
        return BillingError
    }

    // Rate limiting
    if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") {
        return RateLimitError
    }

    // Resource availability
    if strings.Contains(errMsg, "capacity") || strings.Contains(errMsg, "unavailable") {
        return AvailabilityError
    }

    return UnknownError
}
```

### Retry and Circuit Breaker Patterns

The `BaseProvider` implements common resilience patterns:

```go
type BaseProvider struct {
    info        *ProviderInfo
    rateLimits  *RateLimitInfo
    circuitBreaker *CircuitBreaker
    retryPolicy    *RetryPolicy
}

func (b *BaseProvider) ExecuteWithRetry(ctx context.Context, operation func() error) error {
    return b.retryPolicy.Execute(ctx, func() error {
        if b.circuitBreaker.State() == CircuitOpen {
            return ErrCircuitOpen
        }

        err := operation()
        if err != nil {
            b.circuitBreaker.RecordFailure()
            return err
        }

        b.circuitBreaker.RecordSuccess()
        return nil
    })
}
```

## Credential Management

### Secret Resolution Flow

Controllers resolve provider credentials through Kubernetes secrets:

```go
func (r *GPUNodeClassReconciler) getProviderCredentials(ctx context.Context, ref *v1.SecretReference) (string, error) {
    namespace := ref.Namespace
    if namespace == "" {
        namespace = r.DefaultNamespace
    }

    secret := &corev1.Secret{}
    key := client.ObjectKey{Name: ref.Name, Namespace: namespace}

    if err := r.Get(ctx, key, secret); err != nil {
        return "", fmt.Errorf("failed to get credential secret %s/%s: %w", namespace, ref.Name, err)
    }

    credential, exists := secret.Data[ref.Key]
    if !exists {
        return "", fmt.Errorf("credential key %s not found in secret %s/%s", ref.Key, namespace, ref.Name)
    }

    return string(credential), nil
}
```

### Provider Factory Pattern

Controllers use a factory to instantiate provider clients:

```go
func (r *GPUNodeClassReconciler) createProviderClient(providerName, credential string) (providers.ProviderClient, error) {
    switch providerName {
    case "vultr":
        return vultr.NewClient(credential), nil
    case "gcp":
        return gcp.NewClient(credential), nil
    case "scaleway":
        return scaleway.NewClient(credential), nil
    case "digitalocean":
        return digitalocean.NewClient(credential), nil
    default:
        return nil, fmt.Errorf("unsupported provider: %s", providerName)
    }
}
```

## Summary

The TGP operator's provider integration architecture uses:

1. **Declarative Configuration**: `GPUNodeClass` CRDs define provider settings
2. **Code Generation**: OpenAPI tools and official SDKs generate type-safe clients
3. **Standardized Interface**: Common `ProviderClient` interface abstracts provider differences
4. **Native Talos Support**: All providers support Talos Linux deployment
5. **Developer-Friendly**: No business validation or upfront deposits required
6. **Cost-Effective**: Budget options starting at $0.059/hr with Vultr fractional GPUs

This architecture provides a scalable foundation for adding new providers while maintaining type safety, operational reliability, and cost optimization. The focus on native Talos Linux support ensures consistent deployment patterns across all cloud providers.