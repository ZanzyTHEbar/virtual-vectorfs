package models

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// TODO: Implement build flag for

// CascadeManager manages the intelligent fallback cascade system
type CascadeManager struct {
	providers map[string]*GGUFProvider
	health    map[string]*ModelHealth
	mu        sync.RWMutex
}

// NewCascadeManager creates a new cascade manager
func NewCascadeManager() *CascadeManager {
	return &CascadeManager{
		providers: make(map[string]*GGUFProvider),
		health:    make(map[string]*ModelHealth),
	}
}

// AddProvider adds a provider to the cascade
func (c *CascadeManager) AddProvider(name string, provider *GGUFProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers[name] = provider
	c.health[name] = provider.GetHealth()
}

// RemoveProvider removes a provider from the cascade
func (c *CascadeManager) RemoveProvider(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.providers, name)
	delete(c.health, name)
}

// GetBestProvider returns the best available provider based on health metrics
func (c *CascadeManager) GetBestProvider(ctx context.Context) (*GGUFProvider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Priority order: LFM-2-Chat > Gemma 3 > Nomic > Cloud
	priorityOrder := []string{"lfm2-chat", "gemma3", "nomic", "cloud"}

	for _, name := range priorityOrder {
		if provider, exists := c.providers[name]; exists {
			health := provider.GetHealth()
			if health.IsHealthy && time.Since(health.LastUsed) < 5*time.Minute {
				return provider, nil
			}
		}
	}

	return nil, fmt.Errorf("no healthy providers available")
}

// UpdateHealth updates health metrics for all providers
func (c *CascadeManager) UpdateHealth() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, provider := range c.providers {
		c.health[name] = provider.GetHealth()
	}
}

// GetHealthSummary returns a summary of all provider health
func (c *CascadeManager) GetHealthSummary() map[string]*ModelHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := make(map[string]*ModelHealth)
	for name, health := range c.health {
		// Copy to avoid race conditions
		healthCopy := *health
		summary[name] = &healthCopy
	}

	return summary
}

// DetectOptimalHardware detects optimal hardware configuration
func DetectOptimalHardware() (int, int, bool) {
	// Detect CPU cores
	cpuCores := runtime.NumCPU()

	// Detect GPU layers (simplified - in production, check CUDA availability)
	gpuLayers := 0
	if hasCUDA() {
		gpuLayers = -1 // Use all available GPU layers
	}

	// Detect memory for F16 decision
	useF16 := true // Default to F16 for memory efficiency

	return cpuCores, gpuLayers, useF16
}

// hasCUDA checks if CUDA is available (simplified check)
func hasCUDA() bool {
	// FIXME: In production, this would check for CUDA runtime
	// For now, assume no CUDA on this system
	return false
}
