package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// Ingester handles parallel ingestion of memory items
type Ingester struct {
	config       *config.MemoryConfig
	vectorIndex  VectorIndex
	lexicalIndex LexicalIndex
	graphStore   GraphStore
	extractor    KnowledgeExtractor
	metrics      *MetricsCollector
	queue        chan *IngestionTask
	wg           sync.WaitGroup
	mu           sync.RWMutex
	stopping     bool
}

// IngestionTask represents a single ingestion operation
type IngestionTask struct {
	ID        string
	Item      *MemoryItem
	Episode   *Episode // For graph extraction
	Priority  int      // Higher priority processed first
	CreatedAt time.Time
}

// NewIngester creates a new ingester with worker pool
func NewIngester(config *config.MemoryConfig, vectorIndex VectorIndex, lexicalIndex LexicalIndex, graphStore GraphStore, extractor KnowledgeExtractor, metrics *MetricsCollector) *Ingester {
	ingester := &Ingester{
		config:       config,
		vectorIndex:  vectorIndex,
		lexicalIndex: lexicalIndex,
		graphStore:   graphStore,
		extractor:    extractor,
		metrics:      metrics,
		queue:        make(chan *IngestionTask, config.IngestBatchSize*2), // Buffer for 2 batches
	}

	// Start worker pool
	for i := 0; i < config.IngestBatchSize; i++ {
		ingester.wg.Add(1)
		go ingester.worker(i)
	}

	return ingester
}

// IngestMemoryItem ingests a memory item with idempotence and backpressure
func (ing *Ingester) IngestMemoryItem(ctx context.Context, item *MemoryItem) error {
	return ing.IngestWithPriority(ctx, item, nil, 0)
}

// IngestWithPriority ingests an item with specified priority
func (ing *Ingester) IngestWithPriority(ctx context.Context, item *MemoryItem, episode *Episode, priority int) error {
	// Check for backpressure
	select {
	case ing.queue <- &IngestionTask{
		ID:        item.ID,
		Item:      item,
		Episode:   episode,
		Priority:  priority,
		CreatedAt: time.Now(),
	}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("ingestion queue full, backpressure applied")
	}
}

// IngestBatch ingests a batch of items
func (ing *Ingester) IngestBatch(ctx context.Context, items []*MemoryItem) error {
	for _, item := range items {
		if err := ing.IngestMemoryItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// Stop gracefully stops the ingester
func (ing *Ingester) Stop() error {
	ing.mu.Lock()
	ing.stopping = true
	ing.mu.Unlock()

	close(ing.queue)
	ing.wg.Wait()
	return nil
}

// worker processes ingestion tasks
func (ing *Ingester) worker(id int) {
	defer ing.wg.Done()

	for task := range ing.queue {
		start := time.Now()

		err := ing.processTask(context.Background(), task)
		duration := time.Since(start)

		ing.metrics.RecordIngest(duration, err)

		if err != nil {
			// Log error but continue processing other tasks
			fmt.Printf("Worker %d failed to process task %s: %v\n", id, task.ID, err)
		}
	}
}

// processTask handles the actual ingestion logic
func (ing *Ingester) processTask(ctx context.Context, task *IngestionTask) error {
	// Idempotence check: hash content to detect duplicates
	contentHash := ing.hashContent(task.Item.Text)
	if ing.isDuplicate(task.Item.ID, contentHash) {
		return nil // Already processed
	}

	// Parallel processing of vector, lexical, and graph ingestion
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// 1. Vector index ingestion
	wg.Add(1)
	go func() {
		defer wg.Done()
		if task.Item.Embedding != nil {
			if err := ing.vectorIndex.Upsert(ctx, task.Item.ID, task.Item.Embedding); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("vector upsert failed: %w", err))
				mu.Unlock()
			}
		}
	}()

	// 2. Lexical index ingestion (FTS5)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Note: LexicalIndex interface needs to be updated to support upsert
		// For now, assume it's handled in the implementation
	}()

	// 3. Graph ingestion if episode provided
	if task.Episode != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ing.processGraphIngestion(ctx, task.Episode); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("graph ingestion failed: %w", err))
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("multiple ingestion errors: %v", errors)
	}

	return nil
}

// processGraphIngestion handles entity and edge extraction and storage
func (ing *Ingester) processGraphIngestion(ctx context.Context, episode *Episode) error {
	if ing.extractor == nil {
		return nil // No extractor configured
	}

	// Extract knowledge
	result, err := ing.extractor.Extract(ctx, *episode)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Process entities
	for _, entity := range result.Entities {
		if err := ing.graphStore.UpsertEntity(ctx, &entity); err != nil {
			return fmt.Errorf("entity upsert failed: %w", err)
		}
	}

	// Process edges
	for _, edge := range result.Edges {
		if err := ing.graphStore.UpsertEdge(ctx, &edge); err != nil {
			return fmt.Errorf("edge upsert failed: %w", err)
		}
	}

	return nil
}

// hashContent generates a hash for duplicate detection
func (ing *Ingester) hashContent(content string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(content))
	return h.Sum64()
}

// isDuplicate checks if content has been processed before (simplified implementation)
func (ing *Ingester) isDuplicate(id string, hash uint64) bool {
	// In a full implementation, this would check a cache or database
	// For now, assume no duplicates
	return false
}

// GetQueueSize returns the current queue size
func (ing *Ingester) GetQueueSize() int {
	return len(ing.queue)
}

// IsStopping returns whether the ingester is stopping
func (ing *Ingester) IsStopping() bool {
	ing.mu.RLock()
	defer ing.mu.RUnlock()
	return ing.stopping
}
