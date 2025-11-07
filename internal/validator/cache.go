package validator

import (
	"fmt"
	"sync"
	"time"
)

// ReferenceDataCache caches reference data resources (Practitioner, Organization, etc.)
type ReferenceDataCache struct {
	cache map[string]CachedResource
	mu    sync.RWMutex
	ttl   time.Duration
	maxSize int
}

// CachedResource represents a cached FHIR resource
type CachedResource struct {
	Resource  map[string]interface{}
	ExpiresAt time.Time
	LastAccess time.Time
}

// CachedResourceTypes contains reference resource types that should be cached.
var CachedResourceTypes = map[string]bool{
	"Practitioner":     true,
	"PractitionerRole": true,
	"Organization":     true,
	"Location":         true,
}

var referenceCache *ReferenceDataCache
var cacheOnce sync.Once

// InitReferenceCache initializes the reference data cache
func InitReferenceCache(ttl time.Duration, maxSize int) {
	cacheOnce.Do(func() {
		referenceCache = &ReferenceDataCache{
			cache:   make(map[string]CachedResource),
			ttl:     ttl,
			maxSize: maxSize,
		}
		// Start background refresh goroutine
		go referenceCache.backgroundRefresh()
	})
}

// GetReferenceCache returns the reference data cache instance
func GetReferenceCache() *ReferenceDataCache {
	if referenceCache == nil {
		// Initialize with defaults if not initialized
		InitReferenceCache(1*time.Hour, 10000)
	}
	return referenceCache
}

// Get retrieves a resource from cache
func (c *ReferenceDataCache) Get(resourceType, id string) (map[string]interface{}, bool) {
	if !CachedResourceTypes[resourceType] {
		return nil, false
	}

	key := fmt.Sprintf("%s/%s", resourceType, id)
	
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	cached, exists := c.cache[key]
	if !exists {
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		return nil, false
	}
	
	// Update last access
	cached.LastAccess = time.Now()
	c.cache[key] = cached
	
	return cached.Resource, true
}

// Set stores a resource in cache
func (c *ReferenceDataCache) Set(resourceType, id string, resource map[string]interface{}) {
	if !CachedResourceTypes[resourceType] {
		return
	}

	key := fmt.Sprintf("%s/%s", resourceType, id)
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if we need to evict (LRU)
	if len(c.cache) >= c.maxSize {
		c.evictLRU()
	}
	
	c.cache[key] = CachedResource{
		Resource:   resource,
		ExpiresAt: time.Now().Add(c.ttl),
		LastAccess: time.Now(),
	}
}

// Invalidate removes a resource from cache
func (c *ReferenceDataCache) Invalidate(resourceType, id string) {
	key := fmt.Sprintf("%s/%s", resourceType, id)
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.cache, key)
}

// Clear removes all resources from cache
func (c *ReferenceDataCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache = make(map[string]CachedResource)
}

// evictLRU evicts the least recently used resource
func (c *ReferenceDataCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	
	for key, cached := range c.cache {
		if first || cached.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.LastAccess
			first = false
		}
	}
	
	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// backgroundRefresh refreshes resources before expiration
func (c *ReferenceDataCache) backgroundRefresh() {
	ticker := time.NewTicker(c.ttl / 10) // Check every 10% of TTL
	defer ticker.Stop()
	
	for range ticker.C {
		c.refreshExpiring()
	}
}

// refreshExpiring refreshes resources that are about to expire
func (c *ReferenceDataCache) refreshExpiring() {
	c.mu.RLock()
	expiring := make([]string, 0)
	now := time.Now()
	refreshThreshold := now.Add(c.ttl * 9 / 10) // Refresh at 90% TTL
	
	for key, cached := range c.cache {
		if cached.ExpiresAt.Before(refreshThreshold) {
			expiring = append(expiring, key)
		}
	}
	c.mu.RUnlock()
	
	// Note: Actual refresh would require fetching from FHIR store
	// This is a placeholder for the refresh logic
	// In production, you'd call the FHIR store API here
	for _, key := range expiring {
		// TODO: Implement actual refresh from FHIR store
		// For now, we just extend the TTL
		c.mu.Lock()
		if cached, exists := c.cache[key]; exists {
			cached.ExpiresAt = time.Now().Add(c.ttl)
			c.cache[key] = cached
		}
		c.mu.Unlock()
	}
}

// Stats returns cache statistics
func (c *ReferenceDataCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	stats := CacheStats{
		Size:      len(c.cache),
		MaxSize:   c.maxSize,
		TTL:       c.ttl,
		HitRate:   0, // Would need to track hits/misses
	}
	
	return stats
}

// CacheStats represents cache statistics
type CacheStats struct {
	Size    int
	MaxSize int
	TTL     time.Duration
	HitRate float64
}

// IsCachedResourceType checks if a resource type should be cached
func IsCachedResourceType(resourceType string) bool {
	return CachedResourceTypes[resourceType]
}


