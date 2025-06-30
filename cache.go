package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type fileCache struct {
	filename string
	data     *CacheData
	mu       sync.RWMutex
	memCache map[string]*RepositoryAnalysis // In-memory cache for faster access
}

func NewFileCache(filename string) CacheService {
	cache := &fileCache{
		filename: filename,
		data: &CacheData{
			Repositories: make(map[string]*RepositoryAnalysis),
			LastUpdated:  time.Now(),
			Version:      "2.0.0", // Updated for enhanced caching
		},
		memCache: make(map[string]*RepositoryAnalysis),
	}
	
	// Try to load existing cache
	if err := cache.Load(); err != nil {
		fmt.Printf("📁 Creating new cache file: %s\n", filename)
	} else {
		fmt.Printf("📁 Loaded cache from: %s (%d repositories)\n", filename, len(cache.data.Repositories))
		// Populate in-memory cache
		cache.mu.Lock()
		for k, v := range cache.data.Repositories {
			cache.memCache[k] = v
		}
		cache.mu.Unlock()
	}
	
	return cache
}

func (c *fileCache) Get(key string) (*RepositoryAnalysis, bool) {
	c.mu.RLock()
	analysis, exists := c.memCache[key]
	c.mu.RUnlock()
	
	if !exists {
		return nil, false
	}
	
	// Check if cache is still valid (24 hours)
	if time.Since(analysis.LastAnalyzed) > 24*time.Hour {
		c.mu.Lock()
		delete(c.memCache, key)
		delete(c.data.Repositories, key)
		c.mu.Unlock()
		return nil, false
	}
	
	// Create a copy to avoid race conditions
	analysisCopy := *analysis
	analysisCopy.CacheHit = true
	return &analysisCopy, true
}

func (c *fileCache) Set(key string, analysis *RepositoryAnalysis) error {
	analysis.LastAnalyzed = time.Now()
	analysis.CacheHit = false
	
	c.mu.Lock()
	c.data.Repositories[key] = analysis
	c.memCache[key] = analysis
	c.data.LastUpdated = time.Now()
	c.mu.Unlock()
	
	return c.Save()
}

func (c *fileCache) Load() error {
	file, err := os.Open(c.filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(c.data)
}

func (c *fileCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	file, err := os.Create(c.filename)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c.data); err != nil {
		return fmt.Errorf("failed to encode cache data: %w", err)
	}

	return nil
}

// GetCacheStats returns statistics about the cache
func (c *fileCache) GetCacheStats() (total int, age time.Duration) {
	return len(c.data.Repositories), time.Since(c.data.LastUpdated)
}

// CleanExpired removes expired entries from cache
func (c *fileCache) CleanExpired() int {
	expiredCount := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	
	for key, analysis := range c.data.Repositories {
		if analysis.LastAnalyzed.Before(cutoff) {
			delete(c.data.Repositories, key)
			expiredCount++
		}
	}
	
	if expiredCount > 0 {
		c.data.LastUpdated = time.Now()
		c.Save()
	}
	
	return expiredCount
}