// Package providers contains the provider implementations for GPU cloud services
package providers

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	limiter    *rate.Limiter
	info       *ProviderInfo
	rateLimits *RateLimitInfo
}

// NewBaseProvider creates a base provider with rate limiting
func NewBaseProvider(info *ProviderInfo, rateLimits *RateLimitInfo) *BaseProvider {
	// Create rate limiter based on provider limits
	limit := rate.Limit(rateLimits.RequestsPerSecond)
	if limit <= 0 {
		limit = rate.Limit(1) // Default to 1 req/sec if not specified
	}

	return &BaseProvider{
		limiter:    rate.NewLimiter(limit, rateLimits.BurstCapacity),
		info:       info,
		rateLimits: rateLimits,
	}
}

// WaitForRateLimit blocks until rate limit allows request
func (b *BaseProvider) WaitForRateLimit(ctx context.Context) error {
	return b.limiter.Wait(ctx)
}

// GetProviderInfo returns provider metadata
func (b *BaseProvider) GetProviderInfo() *ProviderInfo {
	return b.info
}

// GetRateLimits returns rate limiting information
func (b *BaseProvider) GetRateLimits() *RateLimitInfo {
	return b.rateLimits
}

// GPUTypeTranslator provides translation between standard and provider-specific GPU types
type GPUTypeTranslator struct {
	translations map[string]string
}

// NewGPUTypeTranslator creates a translator with predefined mappings
func NewGPUTypeTranslator(translations map[string]string) *GPUTypeTranslator {
	return &GPUTypeTranslator{
		translations: translations,
	}
}

// Translate converts standard GPU type to provider-specific format
func (t *GPUTypeTranslator) Translate(standard string) (string, error) {
	if providerSpecific, exists := t.translations[standard]; exists {
		return providerSpecific, nil
	}
	return "", fmt.Errorf("unsupported GPU type: %s", standard)
}

// RegionTranslator provides translation between standard and provider-specific regions
type RegionTranslator struct {
	translations map[string]string
}

// NewRegionTranslator creates a translator with predefined mappings
func NewRegionTranslator(translations map[string]string) *RegionTranslator {
	return &RegionTranslator{
		translations: translations,
	}
}

// Translate converts standard region to provider-specific format
func (t *RegionTranslator) Translate(standard string) (string, error) {
	if providerSpecific, exists := t.translations[standard]; exists {
		return providerSpecific, nil
	}
	return "", fmt.Errorf("unsupported region: %s", standard)
}

// PricingNormalizer converts provider-specific pricing to normalized format
type PricingNormalizer struct {
	billingModel BillingModel
}

// NewPricingNormalizer creates a normalizer for the provider's billing model
func NewPricingNormalizer(billingModel BillingModel) *PricingNormalizer {
	return &PricingNormalizer{
		billingModel: billingModel,
	}
}

// Normalize converts provider pricing to standard per-second and per-hour rates
func (n *PricingNormalizer) Normalize(providerPrice float64, currency string) *NormalizedPricing {
	var pricePerSecond, pricePerHour float64

	switch n.billingModel {
	case BillingPerSecond:
		pricePerSecond = providerPrice
		pricePerHour = providerPrice * 3600
	case BillingPerMinute:
		pricePerSecond = providerPrice / 60
		pricePerHour = providerPrice * 60
	case BillingPerHour:
		pricePerSecond = providerPrice / 3600
		pricePerHour = providerPrice
	default:
		// Default to per-hour
		pricePerSecond = providerPrice / 3600
		pricePerHour = providerPrice
	}

	return &NormalizedPricing{
		PricePerSecond: pricePerSecond,
		PricePerHour:   pricePerHour,
		Currency:       currency,
		BillingModel:   n.billingModel,
		LastUpdated:    time.Now(),
	}
}
