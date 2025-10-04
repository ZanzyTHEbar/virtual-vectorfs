package service

import (
	"sync"
	"time"
)

// MetricsCollector collects performance metrics for memory operations
type MetricsCollector struct {
	mu sync.RWMutex

	// Counters
	ingestCount      int64
	retrievalCount   int64
	graphIngestCount int64

	// Latency tracking
	ingestLatency    []time.Duration
	retrievalLatency []time.Duration
	graphLatency     []time.Duration

	// Error tracking
	ingestErrors    int64
	retrievalErrors int64
	graphErrors     int64

	// Index-specific metrics
	indexStats map[string]IndexStats

	// Graph metrics
	entityCount int64
	edgeCount   int64
}

// IndexStats tracks metrics for individual indexes
type IndexStats struct {
	QueryCount   int64
	TotalLatency time.Duration
	ErrorCount   int64
	Size         int64 // Approximate size in bytes
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		ingestLatency:    make([]time.Duration, 0, 1000),
		retrievalLatency: make([]time.Duration, 0, 1000),
		graphLatency:     make([]time.Duration, 0, 1000),
		indexStats:       make(map[string]IndexStats),
	}
}

// RecordIngest records an ingestion operation
func (mc *MetricsCollector) RecordIngest(duration time.Duration, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.ingestCount++
	mc.ingestLatency = append(mc.ingestLatency, duration)
	if err != nil {
		mc.ingestErrors++
	}
}

// RecordRetrieval records a retrieval operation
func (mc *MetricsCollector) RecordRetrieval(indexName string, duration time.Duration, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.retrievalCount++
	mc.retrievalLatency = append(mc.retrievalLatency, duration)

	stats := mc.indexStats[indexName]
	stats.QueryCount++
	stats.TotalLatency += duration
	if err != nil {
		stats.ErrorCount++
		mc.retrievalErrors++
	}
	mc.indexStats[indexName] = stats
}

// RecordGraphIngest records a graph ingestion operation
func (mc *MetricsCollector) RecordGraphIngest(duration time.Duration, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.graphIngestCount++
	mc.graphLatency = append(mc.graphLatency, duration)
	if err != nil {
		mc.graphErrors++
	}
}

// UpdateEntityCount updates the entity count
func (mc *MetricsCollector) UpdateEntityCount(count int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.entityCount = count
}

// UpdateEdgeCount updates the edge count
func (mc *MetricsCollector) UpdateEdgeCount(count int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.edgeCount = count
}

// UpdateIndexSize updates the size for an index
func (mc *MetricsCollector) UpdateIndexSize(indexName string, size int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	stats := mc.indexStats[indexName]
	stats.Size = size
	mc.indexStats[indexName] = stats
}

// GetSummary returns a summary of collected metrics
func (mc *MetricsCollector) GetSummary() MetricsSummary {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return MetricsSummary{
		IngestCount:      mc.ingestCount,
		RetrievalCount:   mc.retrievalCount,
		GraphIngestCount: mc.graphIngestCount,
		IngestErrors:     mc.ingestErrors,
		RetrievalErrors:  mc.retrievalErrors,
		GraphErrors:      mc.graphErrors,
		EntityCount:      mc.entityCount,
		EdgeCount:        mc.edgeCount,
		IndexStats:       mc.indexStats,
		IngestLatency:    mc.calculatePercentiles(mc.ingestLatency),
		RetrievalLatency: mc.calculatePercentiles(mc.retrievalLatency),
		GraphLatency:     mc.calculatePercentiles(mc.graphLatency),
	}
}

// calculatePercentiles calculates p50, p95, p99 latencies
func (mc *MetricsCollector) calculatePercentiles(latencies []time.Duration) LatencyPercentiles {
	if len(latencies) == 0 {
		return LatencyPercentiles{}
	}

	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	// Simple bubble sort for small arrays
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return LatencyPercentiles{
		P50: sorted[len(sorted)*50/100],
		P95: sorted[len(sorted)*95/100],
		P99: sorted[len(sorted)*99/100],
	}
}

// MetricsSummary represents a summary of collected metrics
type MetricsSummary struct {
	IngestCount      int64                 `json:"ingest_count"`
	RetrievalCount   int64                 `json:"retrieval_count"`
	GraphIngestCount int64                 `json:"graph_ingest_count"`
	IngestErrors     int64                 `json:"ingest_errors"`
	RetrievalErrors  int64                 `json:"retrieval_errors"`
	GraphErrors      int64                 `json:"graph_errors"`
	EntityCount      int64                 `json:"entity_count"`
	EdgeCount        int64                 `json:"edge_count"`
	IndexStats       map[string]IndexStats `json:"index_stats"`
	IngestLatency    LatencyPercentiles    `json:"ingest_latency"`
	RetrievalLatency LatencyPercentiles    `json:"retrieval_latency"`
	GraphLatency     LatencyPercentiles    `json:"graph_latency"`
}

// LatencyPercentiles represents latency percentiles
type LatencyPercentiles struct {
	P50 time.Duration `json:"p50"`
	P95 time.Duration `json:"p95"`
	P99 time.Duration `json:"p99"`
}

// Reset clears all collected metrics
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.ingestCount = 0
	mc.retrievalCount = 0
	mc.graphIngestCount = 0
	mc.ingestErrors = 0
	mc.retrievalErrors = 0
	mc.graphErrors = 0
	mc.entityCount = 0
	mc.edgeCount = 0
	mc.ingestLatency = mc.ingestLatency[:0]
	mc.retrievalLatency = mc.retrievalLatency[:0]
	mc.graphLatency = mc.graphLatency[:0]
	mc.indexStats = make(map[string]IndexStats)
}
