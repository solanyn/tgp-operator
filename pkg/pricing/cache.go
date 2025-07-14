package pricing

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/solanyn/tgp-operator/pkg/providers"
)

type cacheEntry struct {
	pricing   map[string]*providers.NormalizedPricing
	timestamp time.Time
}

type Cache struct {
	data  map[string]*cacheEntry
	mutex sync.RWMutex
	ttl   time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		data: make(map[string]*cacheEntry),
		ttl:  ttl,
	}
}

func (c *Cache) getCacheKey(gpuType, region string) string {
	return fmt.Sprintf("%s:%s", gpuType, region)
}

func (c *Cache) isExpired(entry *cacheEntry) bool {
	return time.Since(entry.timestamp) > c.ttl
}

func (c *Cache) GetPricing(ctx context.Context, providerClients map[string]providers.ProviderClient, gpuType, region string) (map[string]*providers.NormalizedPricing, error) {
	key := c.getCacheKey(gpuType, region)

	c.mutex.RLock()
	entry, exists := c.data[key]
	if exists && !c.isExpired(entry) {
		c.mutex.RUnlock()
		return entry.pricing, nil
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, exists = c.data[key]
	if exists && !c.isExpired(entry) {
		return entry.pricing, nil
	}

	pricing := make(map[string]*providers.NormalizedPricing)

	for providerName, provider := range providerClients {
		priceInfo, err := provider.GetNormalizedPricing(ctx, gpuType, region)
		if err != nil {
			continue
		}
		pricing[providerName] = priceInfo
	}

	c.data[key] = &cacheEntry{
		pricing:   pricing,
		timestamp: time.Now(),
	}

	return pricing, nil
}

func (c *Cache) GetBestPrice(ctx context.Context, providerClients map[string]providers.ProviderClient, gpuType, region string) (*providers.NormalizedPricing, error) {
	pricing, err := c.GetPricing(ctx, providerClients, gpuType, region)
	if err != nil {
		return nil, err
	}

	if len(pricing) == 0 {
		return nil, fmt.Errorf("no pricing available for %s in %s", gpuType, region)
	}

	var bestPrice *providers.NormalizedPricing
	var lowestPrice float64

	for _, price := range pricing {
		if bestPrice == nil || price.PricePerHour < lowestPrice {
			bestPrice = price
			lowestPrice = price.PricePerHour
		}
	}

	return bestPrice, nil
}

func (c *Cache) GetSortedPricing(ctx context.Context, providerClients map[string]providers.ProviderClient, gpuType, region string) ([]*providers.NormalizedPricing, error) {
	pricing, err := c.GetPricing(ctx, providerClients, gpuType, region)
	if err != nil {
		return nil, err
	}

	var sortedPricing []*providers.NormalizedPricing
	for _, price := range pricing {
		sortedPricing = append(sortedPricing, price)
	}

	sort.Slice(sortedPricing, func(i, j int) bool {
		return sortedPricing[i].PricePerHour < sortedPricing[j].PricePerHour
	})

	return sortedPricing, nil
}

func (c *Cache) ClearCache() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data = make(map[string]*cacheEntry)
}
