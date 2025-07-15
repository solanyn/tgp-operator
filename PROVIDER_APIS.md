# Provider API Documentation

This document describes how the TGP operator integrates with various cloud GPU providers, including API endpoints, data models, and spot instance capabilities.

## Overview

The TGP operator abstracts different cloud GPU providers through a unified interface. Each provider client implements the `ProviderClient` interface defined in `pkg/providers/interface.go`.

## Provider Interface

```go
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
```

## Provider Implementations

### 1. Vast.ai

**Base URL**: `https://console.vast.ai/api/v0`
**API Documentation**: [Vast.ai API Documentation](https://vast.ai/docs/api/overview/)

#### Endpoints Used:
- **List Offers**: `GET /bundles` - Get available GPU instances with filters
- **Launch Instance**: `PUT /asks/{offer_id}/` - Launch a specific offer
- **Instance Status**: `GET /instances/{instance_id}/` - Get status of specific instance
- **Terminate Instance**: `DELETE /instances/{instance_id}/` - Stop an instance

#### API Authentication:
- Uses API key in `Authorization: Bearer {token}` header
- Content-Type: `application/json` for POST/PUT requests

#### Request/Response Examples:

**List Offers Request:**
```http
GET /bundles?verified=true&external=false&rentable=true HTTP/1.1
Host: console.vast.ai
Authorization: Bearer {api_key}
```

**Launch Instance Request:**
```http
PUT /asks/{offer_id}/ HTTP/1.1
Host: console.vast.ai
Authorization: Bearer {api_key}
Content-Type: application/json

{
  "client_id": "beta",
  "image": "ubuntu:20.04",
  "args": [],
  "env": {},
  "onstart": "echo 'Starting'"
}
```

#### Data Models:
```go
type vastOffer struct {
    ID            int     `json:"id"`
    GPUName       string  `json:"gpu_name"`
    NumGPUs       int     `json:"num_gpus"`
    DpPh          float64 `json:"dph_total"` // Price per hour
    RAMAmount     float64 `json:"ram_amount"` // GB
    DiskSpace     float64 `json:"disk_space"` // GB
    Available     bool    `json:"available"`
    CountryCode   string  `json:"geolocation"`
    Verified      bool    `json:"verified"`
    Reliability   float64 `json:"reliability"`
}

type vastInstance struct {
    ID        int    `json:"id"`
    Status    string `json:"actual_status"` // "running", "exited", etc.
    PublicIP  string `json:"public_ipaddr"`
    SSHHost   string `json:"ssh_host"`
    SSHPort   int    `json:"ssh_port"`
}
```

#### Spot Instance Support:
- **Current**: Vast.ai offers both interruptible and dedicated instances
- **API Field**: `interruptible` boolean in instance creation
- **Pricing**: Interruptible instances typically 60-80% cheaper
- **Implementation Status**: ⚠️ Not yet implemented in our client

#### Rate Limits:
- 60 requests per minute per API key
- HTTP timeout: 30 seconds

---

### 2. Lambda Labs

**Base URL**: `https://cloud.lambdalabs.com/api/v1/`
**API Documentation**: [Lambda Labs API Documentation](https://docs.lambdalabs.com/cloud/)

#### Current Implementation Status: 
🚧 **STUB IMPLEMENTATION** - Returns mock data only

#### API Design (To Be Implemented):

**Endpoints:**
- **List Instance Types**: `GET /instance-types` - Available GPU configurations
- **Launch Instance**: `POST /instance-operations/launch` - Create new instance
- **Instance Status**: `GET /instances` - List all instances
- **Terminate Instance**: `POST /instance-operations/terminate` - Stop instance

#### API Authentication:
- Uses API key in `Authorization: Bearer {token}` header

#### Current Mock Data:
```go
// Current stub returns static data:
{
    ID:          "lambda-offer-123",
    Provider:    "lambda-labs", 
    GPUType:     gpuType,
    Region:      region,
    HourlyPrice: 0.45,
    Memory:      24,
    Storage:     200,
    Available:   true,
}
```

#### Target Data Models (From API Docs):
```go
type lambdaInstanceType struct {
    Name        string            `json:"name"`
    Price       lambdaPricing     `json:"price_cents_per_hour"`
    Specs       lambdaSpecs       `json:"instance_type"`
    Regions     map[string]bool   `json:"regions_with_capacity_available"`
}
```

#### Spot Instance Support:
- **Current**: Lambda Labs doesn't offer traditional spot instances
- **Alternative**: Reserved instances with hourly billing
- **Implementation Status**: ⚠️ No spot support available from provider

---

### 3. RunPod

**Base URL**: `https://api.runpod.io/graphql`
**API Documentation**: [RunPod GraphQL API](https://docs.runpod.io/docs/api/graphql)

#### Current Implementation Status:
🚧 **STUB IMPLEMENTATION** - Returns mock data only

#### API Design (To Be Implemented):

**GraphQL Endpoint**: Single endpoint for all operations
- **Query**: `podRentInterruptable` - List available spot instances
- **Query**: `pods` - Get instance status
- **Mutation**: `podRentInterruptable` - Launch spot instance  
- **Mutation**: `podTerminate` - Stop instance

#### API Authentication:
- Uses API key in `Authorization: Bearer {token}` header

#### Current Implementation Details:
```go
// Current provider info:
info := &providers.ProviderInfo{
    Name:                  "runpod",
    APIVersion:            "v1", 
    SupportedRegions:      []string{"us-east", "us-west"},
    SupportedGPUTypes:     []string{"RTX4090", "H100", "A100"},
    SupportsSpotInstances: true,
    BillingGranularity:    providers.BillingPerSecond,
}

// GPU type translations:
translations := map[string]string{
    "RTX4090": "NVIDIA GeForce RTX 4090",
    "H100":    "NVIDIA H100", 
    "A100":    "NVIDIA A100",
}

// Region translations:
translations := map[string]string{
    "us-east": "US-CA-1",
    "us-west": "US-TX-1", 
}
```

#### Current Mock Data:
```go
{
    ID:          "runpod-offer-123",
    Provider:    "runpod",
    GPUType:     filters.GPUType,
    Region:      filters.Region, 
    HourlyPrice: 0.38,
    Memory:      24,
    Storage:     100,
    Available:   true,
}
```

#### Target Data Models (GraphQL):
```graphql
type Pod {
  id: String!
  name: String
  runtime: PodRuntime
  machine: Machine
  costPerHr: Float
}

type Machine {
  podHostId: String!
  gpuCount: Int!
  gpuDisplayName: String!
  memoryInGb: Int!
  diskInGb: Int!
}
```

#### Spot Instance Support:
- **Current**: RunPod offers "interruptible" instances (spot equivalent)
- **API**: Separate GraphQL mutations for spot vs on-demand
- **Pricing**: Typically 50-70% cheaper than on-demand
- **Implementation Status**: ⚠️ Needs full GraphQL API implementation

#### Rate Limits:
- 20 requests per second
- 1000 requests per minute  
- Burst capacity: 50

---

### 4. Paperspace

**Base URL**: `https://api.paperspace.io/`
**API Documentation**: [Paperspace Core API](https://docs.paperspace.com/core/api-reference/)

#### Current Implementation Status:
🚧 **STUB IMPLEMENTATION** - Returns mock data only

#### API Design (To Be Implemented):

**Endpoints:**
- **List Machines**: `GET /machines/getAvailability` - Available GPU types
- **Launch Instance**: `POST /machines/createMachine` - Create new machine
- **Instance Status**: `GET /machines/{machineId}` - Get machine details
- **Terminate Instance**: `POST /machines/{machineId}/stop` - Stop machine

#### API Authentication:
- Uses API key in `X-API-Key` header

#### Current Mock Data:
```go
{
    ID:          "paperspace-offer-123",
    Provider:    "paperspace",
    GPUType:     gpuType,
    Region:      region,
    HourlyPrice: 0.51,
    Memory:      24,
    Storage:     50,
    Available:   true,
}
```

#### Spot Instance Support:
- **Current**: Paperspace offers preemptible instances
- **API Field**: `isPreemptible` boolean in machine creation
- **Pricing**: Up to 80% cheaper than dedicated instances
- **Implementation Status**: ⚠️ Needs full API implementation

#### Rate Limits:
- 10 requests per second (estimated from current stub)

---

## Implementation Status Summary

### Production Ready:
- **Vast.ai**: ✅ Full API implementation with real HTTP calls

### Development Required:
- **Lambda Labs**: 🚧 Stub implementation - needs full API integration
- **RunPod**: 🚧 Stub implementation - needs GraphQL integration  
- **Paperspace**: 🚧 Stub implementation - needs REST API integration

## Authentication & Error Handling Patterns

### Current Patterns:

#### Vast.ai (Implemented):
```go
// Authentication
req.Header.Set("Authorization", "Bearer "+c.apiKey)
req.Header.Set("Content-Type", "application/json")

// Error Handling
if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("API request failed with status %d", resp.StatusCode)
}
```

#### Other Providers (To Be Implemented):
- **Lambda Labs**: Bearer token authentication
- **RunPod**: Bearer token with GraphQL
- **Paperspace**: X-API-Key header authentication

### Error Handling Strategy:
- HTTP status code validation
- JSON response parsing
- Rate limit detection (HTTP 429)
- Retry logic with exponential backoff (implemented in BaseProvider)

## Spot Instance Implementation Strategy

### Current Status by Provider:

1. **Vast.ai**: ⚠️ API supports interruptible instances, not yet implemented
2. **Lambda Labs**: ❌ No spot instance support available
3. **RunPod**: ⚠️ Supports interruptible instances, needs GraphQL implementation
4. **Paperspace**: ⚠️ Supports preemptible instances, needs API implementation

### Current Implementation Gaps:

1. **Provider Clients**: Only stubs exist for Lambda Labs, RunPod, Paperspace
2. **Spot Fields**: `SpotPrice` and `IsSpot` fields not populated in `GPUOffer`
3. **Filtering**: No handling of `SpotOnly`/`OnDemandOnly` filters
4. **Pricing Logic**: Controller doesn't differentiate spot vs on-demand pricing
5. **Interruption Handling**: No detection of spot interruptions

### Implementation Plan

#### Phase 1: Complete Basic API Integration
- Implement full API clients for Lambda Labs, RunPod, Paperspace
- Replace stub methods with real API calls
- Add proper error handling and rate limiting

#### Phase 2: Spot Instance Enhancement
- Update Vast.ai client to support interruptible instances
- Add spot instance support to RunPod and Paperspace clients
- Populate `SpotPrice` and `IsSpot` fields in `GPUOffer`
- Implement spot/on-demand filtering in `ListAvailableGPUs`

#### Phase 3: Controller Logic Enhancement
- Enhance instance selection to prefer spot when requested
- Add spot price validation against `MaxHourlyPrice`
- Update metrics to track spot vs on-demand usage

#### Phase 4: Interruption Handling
- Add interruption detection via provider status APIs
- Implement graceful handling of spot terminations
- Add automatic restart logic with spot preferences

## API Update Strategy

### Monitoring Provider API Changes

#### Detection Methods:
1. **Version Headers**: Monitor API version headers in responses
2. **Error Patterns**: Watch for new error codes or deprecation warnings
3. **Response Schema**: Validate response structure against expected models
4. **Provider Announcements**: Subscribe to provider API change notifications

#### Implementation Approach:

```go
// API Version Tracking
type APIVersion struct {
    Provider     string    `json:"provider"`
    Version      string    `json:"version"`
    LastChecked  time.Time `json:"last_checked"`
    IsSupported  bool      `json:"is_supported"`
    DeprecatedAt *time.Time `json:"deprecated_at,omitempty"`
}

// Response Validation
func (c *Client) validateResponse(resp *http.Response, data interface{}) error {
    // Check for deprecation warnings
    if warning := resp.Header.Get("Deprecation"); warning != "" {
        log.Warnf("API deprecation warning: %s", warning)
    }
    
    // Validate API version compatibility
    if version := resp.Header.Get("API-Version"); version != c.expectedVersion {
        return fmt.Errorf("API version mismatch: expected %s, got %s", 
            c.expectedVersion, version)
    }
    
    return nil
}
```

### Maintenance Strategy:

#### 1. **Backward Compatibility Layer**
- Maintain support for multiple API versions when possible
- Use feature flags to toggle between API versions
- Implement adapter patterns for schema changes

#### 2. **Gradual Migration Process**
```go
// Example: Multi-version support
type ProviderClient interface {
    GetAPIVersion() string
    SupportsFeature(feature string) bool
    // ... existing methods
}

// Feature detection
func (c *VastClient) SupportsFeature(feature string) bool {
    switch c.apiVersion {
    case "v1":
        return feature != "interruptible_instances"
    case "v2":
        return true // supports all features
    default:
        return false
    }
}
```

#### 3. **Update Workflow**
1. **Detection**: Monitor for API changes via automated checks
2. **Assessment**: Evaluate impact on existing functionality
3. **Planning**: Create migration timeline and backward compatibility strategy
4. **Implementation**: Update client code with feature flags
5. **Testing**: Validate against both old and new API versions
6. **Deployment**: Gradual rollout with monitoring
7. **Cleanup**: Remove deprecated code after grace period

#### 4. **Automated Monitoring**
```go
// Health check with API version validation
func (c *Client) healthCheck(ctx context.Context) error {
    resp, err := c.makeRequest(ctx, "GET", "/health", nil)
    if err != nil {
        return err
    }
    
    // Track API version changes
    currentVersion := resp.Header.Get("API-Version")
    if currentVersion != c.lastKnownVersion {
        c.logVersionChange(c.lastKnownVersion, currentVersion)
        c.lastKnownVersion = currentVersion
    }
    
    return nil
}
```

### Documentation Maintenance:

#### 1. **Change Log Tracking**
- Maintain a CHANGELOG.md for each provider integration
- Document API version compatibility matrix
- Track deprecation timelines and migration paths

#### 2. **Regular Review Schedule**
- Monthly API compatibility checks
- Quarterly documentation updates
- Semi-annual full integration reviews

#### 3. **Provider Communication**
- Establish contacts with provider API teams when possible
- Subscribe to provider API mailing lists and changelogs
- Participate in provider developer communities

### Emergency Response:

#### Breaking Changes:
1. **Immediate**: Feature flag to disable affected provider
2. **Short-term**: Hotfix for critical functionality
3. **Medium-term**: Full implementation update
4. **Long-term**: Architecture improvements based on lessons learned

#### Provider Deprecation:
1. **Assessment**: Evaluate provider usage and criticality
2. **Communication**: Notify users of upcoming changes
3. **Migration**: Provide alternative provider recommendations
4. **Sunset**: Graceful removal with sufficient notice period

## Testing Strategy

### Mock Providers
- Current test providers return static data
- Need to add spot instance scenarios for testing
- Should simulate spot interruptions for robustness testing
- Add API version compatibility testing

### Integration Tests
- Test spot instance requests with real provider APIs (optional, with credentials)
- Validate price comparison logic
- Test interruption recovery scenarios
- Test API version change handling

### Monitoring in Production
- Track API response times and error rates
- Monitor for API version changes
- Alert on unexpected response schemas
- Log deprecation warnings

## Rate Limiting Considerations

Each provider has different rate limits:
- **Vast.ai**: 60 req/min
- **Lambda Labs**: 1000 req/hour
- **RunPod**: 100 req/min (GraphQL)
- **Paperspace**: 200 req/min

The operator should respect these limits when querying for spot availability and pricing updates.

### Rate Limit Strategy:
- Implement adaptive rate limiting based on provider responses
- Use circuit breakers for failing providers
- Cache responses appropriately to reduce API calls
- Distribute requests across time to avoid bursts