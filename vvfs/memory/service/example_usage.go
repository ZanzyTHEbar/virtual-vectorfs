package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// ExampleBasicMemoryUsage demonstrates basic memory system usage
func ExampleBasicMemoryUsage() {
	// Step 1: Open database connection
	db, err := sql.Open("sqlite3", "memory.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Step 2: Configure memory system
	memCfg := &config.MemoryConfig{
		VectorIndex:     "flat", // Start with flat index
		EnsembleEnabled: false,  // Disable ensemble for simplicity
		GraphEnabled:    false,  // Disable graph for now
		Alpha:           0.5,    // Hybrid fusion weight
		K:               10,     // Top-K results
		Lambda:          0.1,    // Time decay factor
		Threshold:       0.3,    // Similarity threshold
		Autocut:         true,   // Enable autocut
	}

	// Step 3: Initialize memory system
	ctx := context.Background()
	memSys, err := NewMemorySystem(ctx, MemorySystemConfig{
		Config: memCfg,
		DB:     db,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memSys.Close()

	// Step 4: Ingest a memory item
	item := &MemoryItem{
		Type: "document",
		Text: "The quick brown fox jumps over the lazy dog.",
		Metadata: map[string]interface{}{
			"category": "example",
			"source":   "demo",
		},
	}

	if err := memSys.Ingest(ctx, item); err != nil {
		log.Fatal(err)
	}

	fmt.Println("✅ Ingested memory item:", item.ID)

	// Step 5: Search for similar items
	results, err := memSys.Search(ctx, "fox and dog", SearchOptions{
		K:     5,
		Alpha: 0.5,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n🔍 Search Results:")
	for i, result := range results {
		fmt.Printf("%d. ID: %s, Score: %.4f, Source: %s\n",
			i+1, result.ID, result.Score, result.Provenance)
	}
}

// ExampleGraphKnowledgeExtraction demonstrates graph-based knowledge extraction
func ExampleGraphKnowledgeExtraction() {
	// Setup (similar to basic example)
	db, _ := sql.Open("sqlite3", "memory.db")
	defer db.Close()

	memCfg := &config.MemoryConfig{
		VectorIndex:  "flat",
		GraphEnabled: true, // Enable graph features
	}

	ctx := context.Background()
	memSys, err := NewMemorySystem(ctx, MemorySystemConfig{
		Config: memCfg,
		DB:     db,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memSys.Close()

	// Create an episode for knowledge extraction
	episode := &Episode{
		Content: `
			Alice works on the VectorDB project.
			Bob is the tech lead for the VectorDB project.
			Alice and Bob collaborate on improving search algorithms.
		`,
		Metadata: map[string]interface{}{
			"source": "team_notes",
		},
	}

	item := &MemoryItem{
		Type: "episode",
		Text: episode.Content,
		Metadata: map[string]interface{}{
			"type": "knowledge_graph",
		},
	}

	// Ingest with graph extraction
	if err := memSys.IngestWithEpisode(ctx, item, episode); err != nil {
		log.Fatal(err)
	}

	fmt.Println("✅ Ingested episode with graph extraction")

	// The knowledge extractor will have identified:
	// - Entities: Alice (person), Bob (person), VectorDB (project)
	// - Edges: Alice-[works_on]->VectorDB, Bob-[tech_lead]->VectorDB
}

// ExampleEnsembleSearch demonstrates advanced ensemble search
func ExampleEnsembleSearch() {
	db, _ := sql.Open("sqlite3", "memory.db")
	defer db.Close()

	memCfg := &config.MemoryConfig{
		VectorIndex:      "hnsw",         // Use HNSW for better performance
		EnsembleEnabled:  true,           // Enable ensemble
		EnsembleStrategy: "weighted_rrf", // Use weighted RRF fusion
		GraphEnabled:     true,           // Enable graph for reranking
		Alpha:            0.5,
		K:                10,
	}

	ctx := context.Background()
	memSys, err := NewMemorySystem(ctx, MemorySystemConfig{
		Config: memCfg,
		DB:     db,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memSys.Close()

	// Perform ensemble search
	results, err := memSys.Search(ctx, "machine learning algorithms", SearchOptions{
		K:          10,
		Alpha:      0.6, // Favor vector search
		GraphDepth: 2,   // 2-hop graph reranking
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n🎯 Ensemble Search Results:")
	for i, result := range results {
		fmt.Printf("%d. ID: %s, Score: %.4f, Source: %s\n",
			i+1, result.ID, result.Score, result.Provenance)
	}

	// Get metrics
	metrics := memSys.GetMetrics()
	fmt.Printf("\n📊 Metrics: %+v\n", metrics)
}

// ExampleWorkingMemorySummarization demonstrates conversation summarization
func ExampleWorkingMemorySummarization() {
	db, _ := sql.Open("sqlite3", "memory.db")
	defer db.Close()

	memCfg := &config.MemoryConfig{
		VectorIndex: "flat",
	}

	ctx := context.Background()
	memSys, err := NewMemorySystem(ctx, MemorySystemConfig{
		Config: memCfg,
		DB:     db,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memSys.Close()

	// Simulate a conversation
	messages := []ConversationMessage{
		{Role: "user", Content: "What is machine learning?"},
		{Role: "assistant", Content: "Machine learning is a subset of AI that enables systems to learn from data."},
		{Role: "user", Content: "What are common algorithms?"},
		{Role: "assistant", Content: "Common algorithms include decision trees, neural networks, and SVM."},
		{Role: "user", Content: "Tell me about neural networks"},
		{Role: "assistant", Content: "Neural networks are inspired by biological neurons and consist of layers of connected nodes."},
	}

	// Generate summary
	summary, err := memSys.Summarize(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n📝 Conversation Summary:")
	fmt.Println(summary.Content)
	fmt.Println("\nSections:")
	for section, content := range summary.Sections {
		fmt.Printf("  - %s: %s\n", section, content)
	}
}

// ExampleCustomConfiguration shows advanced configuration options
func ExampleCustomConfiguration() {
	memCfg := &config.MemoryConfig{
		// Vector index configuration
		VectorIndex:        "hnsw",
		HNSWM:              32,  // Number of connections per layer
		HNSWEFConstruction: 128, // Size of dynamic candidate list during construction
		HNSWEFSearch:       64,  // Size of dynamic candidate list during search

		// Ensemble configuration
		EnsembleEnabled:  true,
		EnsembleStrategy: "weighted_rrf",
		RouterMode:       "rules",
		WeightsBM25:      0.35,
		WeightsVector:    0.55,
		WeightsGraph:     0.10,

		// Graph configuration
		GraphEnabled:      true,
		GraphDepth:        2,
		GraphCenterPolicy: "top_entity",
		GraphRerankOnly:   false,

		// Retrieval configuration
		Alpha:     0.5,
		K:         10,
		Lambda:    0.1,
		Threshold: 0.3,
		Autocut:   true,
		Rerank:    true,
		TimeDecay: true,

		// Performance tuning
		IngestBatchSize: 1000,
		CacheCapacity:   10000,

		// Metrics and observability
		EnableMetrics: true,
		EnableTracing: true,
	}

	fmt.Printf("✅ Advanced configuration created: %+v\n", memCfg)
}
