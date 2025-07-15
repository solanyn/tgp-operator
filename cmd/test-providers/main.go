package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/solanyn/tgp-operator/pkg/providers"
	"github.com/solanyn/tgp-operator/pkg/providers/lambdalabs"
	"github.com/solanyn/tgp-operator/pkg/providers/paperspace"
	"github.com/solanyn/tgp-operator/pkg/providers/runpod"
)

func main() {
	var (
		provider = flag.String("provider", "", "Provider to test (vast, runpod, lambdalabs, paperspace)")
		apiKey   = flag.String("api-key", "", "API key for the provider")
		action   = flag.String("action", "list", "Action to perform (list, pricing, info)")
		gpuType  = flag.String("gpu-type", "", "GPU type to filter by")
		region   = flag.String("region", "", "Region to filter by")
		maxPrice = flag.Float64("max-price", 0, "Maximum price to filter by")
		pretty   = flag.Bool("pretty", true, "Pretty print JSON output")
	)
	flag.Parse()

	if *provider == "" {
		fmt.Println("Usage: go run cmd/test-providers/main.go -provider=<provider> -api-key=<key> [options]")
		fmt.Println("Providers: runpod, lambdalabs, paperspace")
		fmt.Println("Actions: list, pricing, info")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get API key from environment if not provided
	if *apiKey == "" {
		envVars := map[string]string{
			"runpod":     "RUNPOD_API_KEY",
			"lambdalabs": "LAMBDA_LABS_API_KEY",
			"paperspace": "PAPERSPACE_API_KEY",
		}
		if envVar, ok := envVars[*provider]; ok {
			*apiKey = os.Getenv(envVar)
		}
	}

	if *apiKey == "" {
		fmt.Printf("API key required. Set via -api-key flag or environment variable\n")
		os.Exit(1)
	}

	// Create provider client
	var client providers.ProviderClient
	switch *provider {
	case "runpod":
		client = runpod.NewClient(*apiKey)
	case "lambdalabs":
		client = lambdalabs.NewClient(*apiKey)
	case "paperspace":
		client = paperspace.NewClient(*apiKey)
	default:
		fmt.Printf("Unknown provider: %s\n", *provider)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute action
	switch *action {
	case "info":
		testProviderInfo(client)
	case "list":
		testListGPUs(ctx, client, *gpuType, *region, *maxPrice, *pretty)
	case "pricing":
		testPricing(ctx, client, *gpuType, *region, *pretty)
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		os.Exit(1)
	}
}

func testProviderInfo(client providers.ProviderClient) {
	info := client.GetProviderInfo()
	rateLimits := client.GetRateLimits()

	fmt.Printf("Provider: %s (API v%s)\n", info.Name, info.APIVersion)
	fmt.Printf("Supported Regions: %v\n", info.SupportedRegions)
	fmt.Printf("Supported GPU Types: %v\n", info.SupportedGPUTypes)
	fmt.Printf("Supports Spot Instances: %v\n", info.SupportsSpotInstances)
	fmt.Printf("Billing Granularity: %s\n", info.BillingGranularity)

	if rateLimits != nil {
		fmt.Printf("Rate Limits: %d req/sec, %d req/min (burst: %d)\n",
			rateLimits.RequestsPerSecond, rateLimits.RequestsPerMinute, rateLimits.BurstCapacity)
	}
}

func testListGPUs(ctx context.Context, client providers.ProviderClient, gpuType, region string, maxPrice float64, pretty bool) {
	filters := &providers.GPUFilters{
		GPUType:  gpuType,
		Region:   region,
		MaxPrice: maxPrice,
	}

	fmt.Printf("Listing GPUs with filters: GPU=%s, Region=%s, MaxPrice=%.2f\n", gpuType, region, maxPrice)

	offers, err := client.ListAvailableGPUs(ctx, filters)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d GPU offers:\n", len(offers))

	if pretty {
		for i, offer := range offers {
			fmt.Printf("\n--- Offer %d ---\n", i+1)
			fmt.Printf("ID: %s\n", offer.ID)
			fmt.Printf("Provider: %s\n", offer.Provider)
			fmt.Printf("GPU Type: %s\n", offer.GPUType)
			fmt.Printf("Region: %s\n", offer.Region)
			fmt.Printf("Price: $%.4f/hour\n", offer.HourlyPrice)
			if offer.SpotPrice > 0 {
				fmt.Printf("Spot Price: $%.4f/hour\n", offer.SpotPrice)
			}
			fmt.Printf("Memory: %dGB\n", offer.Memory)
			fmt.Printf("Storage: %dGB\n", offer.Storage)
			fmt.Printf("Available: %v\n", offer.Available)
			fmt.Printf("Is Spot: %v\n", offer.IsSpot)
		}
	} else {
		data, _ := json.MarshalIndent(offers, "", "  ")
		fmt.Println(string(data))
	}
}

func testPricing(ctx context.Context, client providers.ProviderClient, gpuType, region string, pretty bool) {
	if gpuType == "" {
		fmt.Println("GPU type required for pricing test")
		os.Exit(1)
	}

	fmt.Printf("Getting pricing for GPU=%s, Region=%s\n", gpuType, region)

	pricing, err := client.GetNormalizedPricing(ctx, gpuType, region)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if pretty {
		fmt.Printf("Price per hour: $%.4f\n", pricing.PricePerHour)
		fmt.Printf("Price per second: $%.6f\n", pricing.PricePerSecond)
		fmt.Printf("Currency: %s\n", pricing.Currency)
		fmt.Printf("Billing Model: %s\n", pricing.BillingModel)
		fmt.Printf("Last Updated: %s\n", pricing.LastUpdated.Format(time.RFC3339))
	} else {
		data, _ := json.MarshalIndent(pricing, "", "  ")
		fmt.Println(string(data))
	}
}
