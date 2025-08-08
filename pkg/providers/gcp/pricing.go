package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

// getMachinePricing returns hourly pricing for machine types
func (c *Client) getMachinePricing(machineType, region string) float64 {
	// GCP machine type pricing (approximate USD per hour)
	// These should ideally come from the Cloud Billing API
	machinePricing := map[string]float64{
		// N1 Standard
		"n1-standard-1":  0.0475,
		"n1-standard-2":  0.0950,
		"n1-standard-4":  0.1900,
		"n1-standard-8":  0.3800,
		"n1-standard-16": 0.7600,
		"n1-standard-32": 1.5200,
		
		// N2 Standard  
		"n2-standard-2":  0.0971,
		"n2-standard-4":  0.1943,
		"n2-standard-8":  0.3886,
		"n2-standard-16": 0.7772,
		"n2-standard-32": 1.5544,
		
		// A2 (GPU-optimized)
		"a2-highgpu-1g":  3.673,
		"a2-highgpu-2g":  7.346,
		"a2-highgpu-4g":  14.692,
		"a2-highgpu-8g":  29.384,
		"a2-ultragpu-1g": 5.868,
		"a2-ultragpu-2g": 11.736,
		"a2-ultragpu-4g": 23.472,
		"a2-ultragpu-8g": 46.944,
		
		// A3 (H100)
		"a3-highgpu-8g": 12.000, // Approximate
		
		// G2 (L4)
		"g2-standard-4":  1.35,
		"g2-standard-8":  2.70,
		"g2-standard-12": 4.05,
		"g2-standard-16": 5.40,
	}
	
	basePrice, exists := machinePricing[machineType]
	if !exists {
		// Fallback pricing for unknown machine types
		basePrice = 0.1900 // n1-standard-4 equivalent
	}
	
	// Apply regional pricing multiplier
	multiplier := c.getRegionalPricingMultiplier(region)
	return basePrice * multiplier
}

// getGPUPricing returns hourly pricing for GPU types
func (c *Client) getGPUPricing(gpuType, region string) float64 {
	// GCP GPU pricing (approximate USD per hour per GPU)
	gpuPricing := map[string]float64{
		"K80":             0.45,
		"P4":              0.60,
		"P100":            1.46,
		"V100":            2.48,
		"T4":              0.35,
		"A100":            2.93,
		"A100-80GB":       3.67,
		"H100":            8.00, // Approximate
		"L4":              0.60,
		"NVIDIA_K80":      0.45,
		"NVIDIA_P4":       0.60,
		"NVIDIA_P100":     1.46,
		"NVIDIA_V100":     2.48,
		"NVIDIA_T4":       0.35,
		"NVIDIA_A100":     2.93,
		"NVIDIA_A100_80GB": 3.67,
		"NVIDIA_H100_80GB": 8.00, // Approximate
		"NVIDIA_L4":       0.60,
	}
	
	basePrice, exists := gpuPricing[gpuType]
	if !exists {
		// Fallback pricing
		basePrice = 1.00
	}
	
	// Apply regional pricing multiplier
	multiplier := c.getRegionalPricingMultiplier(region)
	return basePrice * multiplier
}

// getRegionalPricingMultiplier returns pricing multiplier for different regions
func (c *Client) getRegionalPricingMultiplier(region string) float64 {
	// Regional pricing variations (approximate)
	multipliers := map[string]float64{
		// US regions
		"us-central1": 1.0,
		"us-east1":    1.0,
		"us-east4":    1.0,
		"us-west1":    1.0,
		"us-west2":    1.0,
		"us-west3":    1.0,
		"us-west4":    1.0,
		
		// Europe regions
		"europe-west1": 1.08,
		"europe-west2": 1.15,
		"europe-west3": 1.08,
		"europe-west4": 1.08,
		"europe-west6": 1.12,
		
		// Asia regions
		"asia-east1":      1.08,
		"asia-northeast1": 1.08,
		"asia-southeast1": 1.08,
		
		// Australia
		"australia-southeast1": 1.15,
	}
	
	if multiplier, exists := multipliers[region]; exists {
		return multiplier
	}
	
	// Default multiplier
	return 1.0
}

// getGPUOffersForZone returns available GPU offers for a specific zone
func (c *Client) getGPUOffersForZone(ctx context.Context, zone string, filters *providers.GPUFilters) ([]*providers.GPUOffer, error) {
	var offers []*providers.GPUOffer
	
	// Get available GPU types for this zone
	availableGPUs := c.getAvailableGPUsInZone(zone)
	
	for _, gpuType := range availableGPUs {
		// Skip if not matching filter
		if filters.GPUType != "" && !strings.EqualFold(filters.GPUType, gpuType) {
			continue
		}
		
		region := c.zoneToRegion(zone)
		
		// Calculate pricing
		machineType := c.getRecommendedMachineTypeForGPU(gpuType)
		machinePrice := c.getMachinePricing(machineType, region)
		gpuPrice := c.getGPUPricing(gpuType, region)
		totalPrice := machinePrice + gpuPrice
		
		// Skip if over budget
		if filters.MaxPrice > 0 && totalPrice > filters.MaxPrice {
			continue
		}
		
		offer := &providers.GPUOffer{
			ID:          fmt.Sprintf("gcp-%s-%s", zone, strings.ToLower(gpuType)),
			Provider:    "gcp",
			Region:      region,
			GPUType:     gpuType,
			HourlyPrice: totalPrice,
			SpotPrice:   totalPrice * 0.7, // Spot instances ~30% cheaper
			Memory:      c.getGPUMemory(gpuType),
			Storage:     50, // Default 50GB SSD
			Available:   true,
			IsSpot:      false,
		}
		
		offers = append(offers, offer)
		
		// Add spot instance variant if not filtering spot only
		if !filters.SpotOnly {
			spotOffer := *offer
			spotOffer.ID = fmt.Sprintf("gcp-%s-%s-spot", zone, strings.ToLower(gpuType))
			spotOffer.HourlyPrice = spotOffer.SpotPrice
			spotOffer.IsSpot = true
			offers = append(offers, &spotOffer)
		}
	}
	
	return offers, nil
}

// getAvailableGPUsInZone returns GPU types available in a zone
func (c *Client) getAvailableGPUsInZone(zone string) []string {
	// This is a simplified mapping. In production, this should query the actual API
	// to get real-time availability
	zoneGPUAvailability := map[string][]string{
		"us-central1-a": {"K80", "P4", "P100", "V100", "T4", "A100"},
		"us-central1-b": {"K80", "P4", "T4", "A100"},
		"us-central1-c": {"K80", "P4", "P100", "V100", "T4"},
		"us-central1-f": {"K80", "P4", "P100", "V100", "T4", "A100"},
		
		"us-east1-b": {"K80", "P4", "P100", "V100", "T4"},
		"us-east1-c": {"K80", "P4", "T4", "A100"},
		"us-east1-d": {"K80", "P4", "P100", "V100", "T4"},
		
		"us-west1-a": {"K80", "P4", "P100", "V100", "T4"},
		"us-west1-b": {"K80", "P4", "T4", "A100"},
		
		"us-west2-a": {"T4", "L4"},
		"us-west2-b": {"T4", "L4", "A100"},
		"us-west2-c": {"T4", "L4"},
		
		"us-west4-a": {"T4", "A100", "H100"},
		"us-west4-b": {"T4", "A100"},
		"us-west4-c": {"T4", "A100"},
		
		"europe-west1-b": {"K80", "P4", "P100", "V100", "T4"},
		"europe-west1-c": {"K80", "P4", "T4"},
		"europe-west1-d": {"K80", "P4", "P100", "V100", "T4", "A100"},
		
		"europe-west4-a": {"K80", "P4", "P100", "V100", "T4", "A100"},
		"europe-west4-b": {"T4", "A100"},
		"europe-west4-c": {"K80", "P4", "T4"},
		
		"asia-east1-a": {"K80", "P4", "P100", "V100", "T4"},
		"asia-east1-b": {"K80", "P4", "T4"},
		"asia-east1-c": {"K80", "P4", "P100", "V100", "T4", "A100"},
	}
	
	if gpus, exists := zoneGPUAvailability[zone]; exists {
		return gpus
	}
	
	// Default availability for unknown zones
	return []string{"T4", "A100"}
}

// getGPUMemory returns memory in GB for GPU types
func (c *Client) getGPUMemory(gpuType string) int64 {
	memoryMap := map[string]int64{
		"K80":       12,
		"P4":        8,
		"P100":      16,
		"V100":      16,
		"T4":        16,
		"A100":      40,
		"A100-80GB": 80,
		"H100":      80,
		"L4":        24,
	}
	
	if memory, exists := memoryMap[gpuType]; exists {
		return memory
	}
	
	return 16 // Default
}

// getRegionsToSearch returns regions to search based on filter
func (c *Client) getRegionsToSearch(regionFilter string) []string {
	allRegions := []string{
		"us-central1", "us-east1", "us-east4", "us-west1", "us-west2", "us-west3", "us-west4",
		"europe-west1", "europe-west2", "europe-west3", "europe-west4", "europe-west6",
		"asia-east1", "asia-northeast1", "asia-southeast1", "australia-southeast1",
	}
	
	if regionFilter == "" {
		return allRegions
	}
	
	// Return matching regions
	var matchingRegions []string
	for _, region := range allRegions {
		if strings.Contains(strings.ToLower(region), strings.ToLower(regionFilter)) {
			matchingRegions = append(matchingRegions, region)
		}
	}
	
	if len(matchingRegions) == 0 {
		// Exact match fallback
		matchingRegions = []string{regionFilter}
	}
	
	return matchingRegions
}

// getZonesForRegion returns zones for a given region
func (c *Client) getZonesForRegion(region string) []string {
	regionZones := map[string][]string{
		"us-central1":         {"us-central1-a", "us-central1-b", "us-central1-c", "us-central1-f"},
		"us-east1":            {"us-east1-b", "us-east1-c", "us-east1-d"},
		"us-east4":            {"us-east4-a", "us-east4-b", "us-east4-c"},
		"us-west1":            {"us-west1-a", "us-west1-b", "us-west1-c"},
		"us-west2":            {"us-west2-a", "us-west2-b", "us-west2-c"},
		"us-west3":            {"us-west3-a", "us-west3-b", "us-west3-c"},
		"us-west4":            {"us-west4-a", "us-west4-b", "us-west4-c"},
		"europe-west1":        {"europe-west1-b", "europe-west1-c", "europe-west1-d"},
		"europe-west2":        {"europe-west2-a", "europe-west2-b", "europe-west2-c"},
		"europe-west3":        {"europe-west3-a", "europe-west3-b", "europe-west3-c"},
		"europe-west4":        {"europe-west4-a", "europe-west4-b", "europe-west4-c"},
		"europe-west6":        {"europe-west6-a", "europe-west6-b", "europe-west6-c"},
		"asia-east1":          {"asia-east1-a", "asia-east1-b", "asia-east1-c"},
		"asia-northeast1":     {"asia-northeast1-a", "asia-northeast1-b", "asia-northeast1-c"},
		"asia-southeast1":     {"asia-southeast1-a", "asia-southeast1-b", "asia-southeast1-c"},
		"australia-southeast1": {"australia-southeast1-a", "australia-southeast1-b", "australia-southeast1-c"},
	}
	
	if zones, exists := regionZones[region]; exists {
		return zones
	}
	
	// Fallback: generate zones for unknown regions
	return []string{region + "-a", region + "-b", region + "-c"}
}

// selectBestZone selects the best zone for launching an instance
func (c *Client) selectBestZone(region, gpuType string) string {
	zones := c.getZonesForRegion(region)
	
	// Find zones that have the requested GPU type
	for _, zone := range zones {
		availableGPUs := c.getAvailableGPUsInZone(zone)
		for _, availableGPU := range availableGPUs {
			if strings.EqualFold(availableGPU, gpuType) {
				return zone
			}
		}
	}
	
	// Fallback to first zone in region
	if len(zones) > 0 {
		return zones[0]
	}
	
	// Ultimate fallback
	return region + "-a"
}

// filterOffers applies additional filtering to offers
func (c *Client) filterOffers(offers []providers.GPUOffer, filters *providers.GPUFilters) []providers.GPUOffer {
	var filtered []providers.GPUOffer
	
	for _, offer := range offers {
		// Apply filters
		if filters.GPUType != "" && !strings.EqualFold(offer.GPUType, filters.GPUType) {
			continue
		}
		
		if filters.Region != "" && !strings.Contains(strings.ToLower(offer.Region), strings.ToLower(filters.Region)) {
			continue
		}
		
		if filters.MaxPrice > 0 && offer.HourlyPrice > filters.MaxPrice {
			continue
		}
		
		if filters.SpotOnly && !offer.IsSpot {
			continue
		}
		
		filtered = append(filtered, offer)
	}
	
	return filtered
}