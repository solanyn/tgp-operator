// Package providers contains the provider implementations for GPU cloud services
package providers

import (
	"context"
	"fmt"
	"net"
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

// RetryConfig defines retry behavior for provider operations
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetriableErrors []RetriableErrorType
}

// RetriableErrorType defines types of errors that should be retried
type RetriableErrorType int

const (
	RetriableErrorRateLimit RetriableErrorType = iota
	RetriableErrorNetwork
	RetriableErrorTemporary
	RetriableErrorServerError
)

// DefaultRetryConfig returns sensible retry defaults for providers
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		MaxDelay:      time.Minute,
		BackoffFactor: 2.0,
		RetriableErrors: []RetriableErrorType{
			RetriableErrorRateLimit,
			RetriableErrorNetwork,
			RetriableErrorTemporary,
			RetriableErrorServerError,
		},
	}
}

// IsRetriableError determines if an error should be retried
func IsRetriableError(err error) (bool, RetriableErrorType) {
	if err == nil {
		return false, 0
	}

	// Network errors
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return true, RetriableErrorNetwork
		}
	}

	// Note: HTTP response checking would need custom error types
	// For now, rely on error message patterns below

	// Check error message for known patterns
	errMsg := err.Error()
	if containsAny(errMsg, []string{"rate limit", "too many requests", "throttled"}) {
		return true, RetriableErrorRateLimit
	}
	if containsAny(errMsg, []string{"timeout", "connection", "network"}) {
		return true, RetriableErrorNetwork
	}
	if containsAny(errMsg, []string{"temporary", "unavailable", "try again"}) {
		return true, RetriableErrorTemporary
	}

	return false, 0
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config *RetryConfig, operation func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// First attempt doesn't wait
		if attempt > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = operation()
		if lastErr == nil {
			return nil // Success
		}

		// Check if we should retry this error
		shouldRetry, errorType := IsRetriableError(lastErr)
		if !shouldRetry {
			return lastErr // Non-retriable error
		}

		// Check if this error type is configured to be retried
		retriable := false
		for _, retryType := range config.RetriableErrors {
			if retryType == errorType {
				retriable = true
				break
			}
		}
		if !retriable {
			return lastErr
		}

		// Don't retry on the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return lastErr
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
