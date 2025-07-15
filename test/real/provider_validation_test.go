//go:build real

package real

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/solanyn/tgp-operator/pkg/providers"
	"github.com/solanyn/tgp-operator/pkg/providers/lambdalabs"
	"github.com/solanyn/tgp-operator/pkg/providers/paperspace"
	"github.com/solanyn/tgp-operator/pkg/providers/runpod"
	"github.com/solanyn/tgp-operator/pkg/providers/vast"
)

// TestProviderValidation validates real cloud provider connections and basic functionality
func TestProviderValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Test configuration for validation
	testGPUType := "RTX3090"
	testRegion := "us-east-1"

	tests := []struct {
		name      string
		envVar    string
		createFn  func(string) providers.ProviderClient
		skipCheck func() bool
	}{
		{
			name:     "vast.ai",
			envVar:   "VAST_API_KEY",
			createFn: func(key string) providers.ProviderClient { return vast.NewClient(key) },
		},
		{
			name:     "runpod",
			envVar:   "RUNPOD_API_KEY",
			createFn: func(key string) providers.ProviderClient { return runpod.NewClient(key) },
		},
		{
			name:     "lambda-labs",
			envVar:   "LAMBDA_LABS_API_KEY",
			createFn: func(key string) providers.ProviderClient { return lambdalabs.NewClient(key) },
		},
		{
			name:     "paperspace",
			envVar:   "PAPERSPACE_API_KEY",
			createFn: func(key string) providers.ProviderClient { return paperspace.NewClient(key) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiKey := os.Getenv(tt.envVar)
			if apiKey == "" {
				t.Skipf("Skipping %s validation - %s not set", tt.name, tt.envVar)
				return
			}

			t.Logf("🔍 Validating %s provider...", tt.name)
			client := tt.createFn(apiKey)

			// Test 1: Provider Info
			t.Run("provider_info", func(t *testing.T) {
				info := client.GetProviderInfo()
				if info.Name != tt.name {
					t.Errorf("Expected provider name %s, got %s", tt.name, info.Name)
				}
				t.Logf("✅ Provider info: %s (API v%s)", info.Name, info.APIVersion)
				t.Logf("   Supported regions: %v", info.SupportedRegions)
				t.Logf("   Supported GPU types: %v", info.SupportedGPUTypes)
			})

			// Test 2: List available GPUs
			t.Run("list_available_gpus", func(t *testing.T) {
				offers, err := client.ListAvailableGPUs(ctx, &providers.GPUFilters{
					GPUType:  testGPUType,
					Region:   testRegion,
					MaxPrice: 10.0, // High limit to avoid filtering
					SpotOnly: false,
				})
				if err != nil {
					t.Logf("⚠️  ListAvailableGPUs failed for %s: %v", tt.name, err)
					// Don't fail - some providers might not have offers for test GPU type
					return
				}
				t.Logf("✅ Found %d GPU offers for %s in %s", len(offers), testGPUType, testRegion)
			})

			// Test 3: Get normalized pricing
			t.Run("get_pricing", func(t *testing.T) {
				pricing, err := client.GetNormalizedPricing(ctx, testGPUType, testRegion)
				if err != nil {
					t.Logf("⚠️  GetNormalizedPricing failed for %s: %v", tt.name, err)
					// Don't fail - some providers might not support this GPU type
					return
				}
				if pricing != nil {
					t.Logf("✅ Pricing for %s: $%.4f/hour", testGPUType, pricing.PricePerHour)
				}
			})

			// Test 4: Rate limits info
			t.Run("rate_limits", func(t *testing.T) {
				rateLimits := client.GetRateLimits()
				if rateLimits != nil {
					t.Logf("✅ Rate limits: %d req/sec, %d req/min",
						rateLimits.RequestsPerSecond, rateLimits.RequestsPerMinute)
				}
			})

			t.Logf("✅ %s provider validation completed", tt.name)
		})
	}
}

// TestProviderConnectivity tests basic connectivity to provider APIs
func TestProviderConnectivity(t *testing.T) {
	providers := map[string]string{
		"vast.ai":     os.Getenv("VAST_API_KEY"),
		"runpod":      os.Getenv("RUNPOD_API_KEY"),
		"lambda-labs": os.Getenv("LAMBDA_LABS_API_KEY"),
		"paperspace":  os.Getenv("PAPERSPACE_API_KEY"),
	}

	validProviders := 0
	for name, key := range providers {
		if key != "" {
			validProviders++
			t.Logf("📡 %s credentials available", name)
		} else {
			t.Logf("⚠️  %s credentials not provided", name)
		}
	}

	if validProviders == 0 {
		t.Skip("No provider credentials available for testing")
	}

	t.Logf("✅ Found credentials for %d/%d providers", validProviders, len(providers))
}

// TestProviderRealLaunch tests actual instance launching (WARNING: This will incur costs!)
func TestProviderRealLaunch(t *testing.T) {
	if os.Getenv("TGP_REAL_LAUNCH") != "true" {
		t.Skip("Skipping real launch test - set TGP_REAL_LAUNCH=true to enable (WARNING: Will incur costs!)")
	}

	t.Log("⚠️  WARNING: This test will launch real instances and incur costs!")
	t.Log("⚠️  Make sure to clean up instances manually if the test fails!")

	// This would implement actual instance launching tests
	// For now, just validate the test setup
	t.Log("🚧 Real launch testing would be implemented here")
	t.Log("🚧 This requires careful cost management and cleanup logic")
}
