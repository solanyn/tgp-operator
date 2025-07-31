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
3. **Manual Implementation**: Custom HTTP clients for simple APIs

## Provider Implementation Details

### Lambda Labs - OpenAPI Generated Client

**Implementation**: `pkg/providers/lambdalabs/`
**API Type**: REST API with OpenAPI specification
**Base URL**: `https://cloud.lambdalabs.com`

#### Code Generation

The Lambda Labs client uses OpenAPI code generation:

```bash
# Generated from OpenAPI spec
oapi-codegen -package api -generate client,types \
  https://cloud.lambdalabs.com/api/openapi.json > pkg/providers/lambdalabs/api/client.go
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

#### API Operations

- `ListInstanceTypesWithResponse()` - Discovers available GPU types
- `LaunchInstanceWithResponse()` - Creates new instances
- `GetInstanceWithResponse()` - Retrieves instance status
- `TerminateInstanceWithResponse()` - Terminates instances

#### Type Mapping

The client maps OpenAPI-generated types to provider interface types:

```go
func (c *Client) ListAvailableGPUs(ctx context.Context, filters *providers.GPUFilters) ([]providers.GPUOffer, error) {
    resp, err := c.apiClient.ListInstanceTypesWithResponse(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list instance types: %w", err)
    }

    var offers []providers.GPUOffer
    for _, item := range resp.JSON200.Data {
        instanceType := item.InstanceType
        pricePerHour := float64(instanceType.PriceCentsPerHour) / 100.0

        for _, region := range item.RegionsWithCapacityAvailable {
            offer := providers.GPUOffer{
                ID:          fmt.Sprintf("%s-%s", instanceType.Name, string(region.Name)),
                Provider:    "lambda-labs",
                GPUType:     instanceType.GpuDescription,
                Region:      string(region.Name),
                HourlyPrice: pricePerHour,
                Memory:      int64(instanceType.Specs.MemoryGib),
                Storage:     int64(instanceType.Specs.StorageGib),
                Available:   true,
                IsSpot:      false, // Lambda Labs doesn't support spot instances
            }
            offers = append(offers, offer)
        }
    }
    return offers, nil
}
```

### RunPod - GraphQL Generated Client

**Implementation**: `pkg/providers/runpod/`
**API Type**: GraphQL API with schema-based generation
**Endpoint**: `https://api.runpod.io/graphql`

#### Code Generation

The RunPod client uses GraphQL code generation with genqlient:

```yaml
# genqlient.yaml
schema: pkg/providers/runpod/schema.graphql
operations:
  - pkg/providers/runpod/queries.graphql
generated: pkg/providers/runpod/generated.go
package: runpod
```

#### GraphQL Operations

Queries and mutations are defined in `.graphql` files:

```graphql
# queries.graphql
query ListGPUTypes {
  gpuTypes {
    id
    displayName
    memoryInGb
    communityPrice
    communitySpotPrice
  }
}

mutation RentSpotInstance($input: PodRentInterruptableInput!) {
  podRentInterruptable(input: $input) {
    id
    status
    machine {
      podHostId
    }
  }
}

mutation TerminatePod($input: PodTerminateInput!) {
  podTerminate(input: $input) {
    id
  }
}

query GetPod($podId: String!) {
  pod(input: { podId: $podId }) {
    id
    name
    status
    runtime {
      uptimeInSeconds
    }
  }
}
```

#### Generated Client Usage

The GraphQL client is generated with type-safe methods:

```go
type Client struct {
    *providers.BaseProvider
    apiKey        string
    graphqlClient graphql.Client
}

// Authentication wrapper
type authHTTPClient struct {
    client *http.Client
    apiKey string
}

func (a *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
    req.Header.Set("Authorization", "Bearer "+a.apiKey)
    return a.client.Do(req)
}

// Provider interface implementation using generated functions
func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
    input := PodRentInterruptableInput{
        GpuTypeId:           req.GPUType,
        Name:                fmt.Sprintf("tgp-%d", time.Now().Unix()),
        ImageName:           "runpod/base:3.10-cuda11.8.0-devel-ubuntu22.04",
        GpuCount:            1,
        VolumeInGb:          20,
    }

    response, err := RentSpotInstance(ctx, c.graphqlClient, input) // Generated function
    if err != nil {
        return nil, fmt.Errorf("failed to launch RunPod spot instance: %w", err)
    }

    return &providers.GPUInstance{
        ID:        response.PodRentInterruptable.Id,
        Status:    c.mapRunPodStatusToProviderStatus(response.PodRentInterruptable.Status),
        CreatedAt: time.Now(),
    }, nil
}
```

### Paperspace - OpenAPI Generated Client (Partial)

**Implementation**: `pkg/providers/paperspace/`
**API Type**: REST API with complex union types
**Base URL**: `https://api.paperspace.com/v1`

#### Implementation Challenge

Paperspace API uses complex union types in create operations that complicate code generation:

```json
{
  "machineType": {
    "oneOf": [
      { "type": "string", "enum": ["Air", "Standard", "Pro"] },
      { "$ref": "#/components/schemas/MachineTypeConfig" }
    ]
  }
}
```

#### Current Implementation

The client uses generated types for read operations and simplified approach for create:

```go
type Client struct {
    *providers.BaseProvider
    apiKey    string
    apiClient *api.ClientWithResponses  // Generated from OpenAPI
}

// Status and terminate use real API calls
func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (*providers.InstanceStatus, error) {
    resp, err := c.apiClient.MachinesGetWithResponse(ctx, instanceID)
    if err != nil {
        return nil, fmt.Errorf("failed to get instance status: %w", err)
    }

    state := c.translateStatus(string(resp.JSON200.State))
    return &providers.InstanceStatus{
        State:     state,
        Message:   c.getStatusMessage(string(resp.JSON200.State)),
        UpdatedAt: time.Now(),
    }, nil
}

// Launch uses simplified mock approach due to complex union types
func (c *Client) LaunchInstance(ctx context.Context, req *providers.LaunchRequest) (*providers.GPUInstance, error) {
    // Enhanced mock implementation until union types are properly handled
    return &providers.GPUInstance{
        ID:        fmt.Sprintf("paperspace-real-%d", time.Now().Unix()),
        Status:    providers.InstanceStatePending,
        CreatedAt: time.Now(),
    }, nil
}
```

#### Authentication Pattern

Paperspace uses Bearer token authentication:

```go
api.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
    req.Header.Set("Authorization", "Bearer "+apiKey)
    return nil
})
```

## Code Generation Summary

### Implementation Approaches

| Provider    | API Type | Generation Tool | Client Pattern             | Status      |
| ----------- | -------- | --------------- | -------------------------- | ----------- |
| RunPod      | GraphQL  | genqlient       | Schema-based generation    | ✅ Complete |
| Lambda Labs | REST     | oapi-codegen    | OpenAPI specification      | ✅ Complete |
| Paperspace  | REST     | oapi-codegen    | Partial due to union types | 🔄 Partial  |

### Generated Artifacts

#### RunPod GraphQL

- `pkg/providers/runpod/generated.go` - Generated types and functions
- `pkg/providers/runpod/queries.graphql` - GraphQL operations
- `pkg/providers/runpod/schema.graphql` - GraphQL schema

#### Lambda Labs OpenAPI

- `pkg/providers/lambdalabs/api/client.go` - Generated REST client
- `pkg/providers/lambdalabs/api/types.go` - Generated type definitions

#### Paperspace OpenAPI

- `pkg/providers/paperspace/api/client.go` - Generated REST client (read operations)
- `pkg/providers/paperspace/api/types.go` - Generated type definitions

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
    apiKey := os.Getenv("RUNPOD_API_KEY")
    if apiKey == "" {
        apiKey = "fake-api-key" // Falls back to mock behavior
    }

    client := runpod.NewClient(apiKey)
    // Test with either real or mock behavior
}
```

## Summary

The TGP operator's provider integration architecture uses:

1. **Declarative Configuration**: `GPUNodeClass` CRDs define provider settings
2. **Code Generation**: OpenAPI and GraphQL tools generate type-safe clients
3. **Standardized Interface**: Common `ProviderClient` interface abstracts provider differences
4. **Resilience Patterns**: Circuit breakers, retries, and error classification
5. **Testing Strategy**: Conditional mock behavior enables both unit and integration testing

This architecture provides a scalable foundation for adding new providers while maintaining type safety and operational reliability. The focus on code generation ensures that API changes are caught at compile time and that client implementations remain consistent with provider specifications.

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
    case "runpod":
        return runpod.NewClient(credential), nil
    case "lambdalabs":
        return lambdalabs.NewClient(credential), nil
    case "paperspace":
        return paperspace.NewClient(credential), nil
    default:
        return nil, fmt.Errorf("unsupported provider: %s", providerName)
    }
}
```
