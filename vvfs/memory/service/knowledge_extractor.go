package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// KnowledgeExtractorImpl implements KnowledgeExtractor using LLM structured output
type KnowledgeExtractorImpl struct {
	llmClient LLMClient // Assuming we have an LLM client interface
	config    *config.MemoryConfig
}

// NewKnowledgeExtractor creates a new knowledge extractor
func NewKnowledgeExtractor(llmClient LLMClient, config *config.MemoryConfig) *KnowledgeExtractorImpl {
	return &KnowledgeExtractorImpl{
		llmClient: llmClient,
		config:    config,
	}
}

// Extract performs LLM-based entity and edge extraction from an episode
func (ke *KnowledgeExtractorImpl) Extract(ctx context.Context, episode Episode) (*ExtractionResult, error) {
	// Build the extraction prompt
	prompt := ke.buildExtractionPrompt(episode)

	// Call LLM with structured output request
	response, err := ke.llmClient.GenerateStructured(ctx, prompt, ExtractionSchema{})
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse response into entities and edges
	result := &ExtractionResult{
		Entities: []Entity{},
		Edges:    []Edge{},
	}

	// Convert LLM response to our structs
	if entitiesData, ok := response["entities"].([]interface{}); ok {
		for _, eData := range entitiesData {
			entity, err := ke.parseEntity(eData)
			if err != nil {
				// Log warning but continue
				continue
			}
			result.Entities = append(result.Entities, *entity)
		}
	}

	if edgesData, ok := response["edges"].([]interface{}); ok {
		for _, eData := range edgesData {
			edge, err := ke.parseEdge(eData, episode)
			if err != nil {
				// Log warning but continue
				continue
			}
			result.Edges = append(result.Edges, *edge)
		}
	}

	// Deduplicate entities based on name similarity (simple implementation)
	result.Entities = ke.deduplicateEntities(result.Entities)

	return result, nil
}

// buildExtractionPrompt creates the LLM prompt for entity/edge extraction
func (ke *KnowledgeExtractorImpl) buildExtractionPrompt(episode Episode) string {
	return fmt.Sprintf(`
You are an expert knowledge extraction system. Analyze the following episode and extract entities (people, organizations, concepts, projects) and relationships between them.

Episode content: "%s"

Instructions:
- Identify distinct entities (people, organizations, projects, concepts, etc.)
- Create relationships (edges) between entities based on the content
- Use specific, descriptive names for entities
- For relationships, use clear, concise relation types (e.g., "works_on", "mentions", "related_to", "manages")
- Ensure entities have unique names within the context
- Output in strict JSON format

Example output format:
{
  "entities": [
    {
      "kind": "person",
      "name": "John Doe",
      "summary": "Software engineer at TechCorp",
      "attrs": {"role": "engineer", "department": "engineering"}
    }
  ],
  "edges": [
    {
      "source_name": "John Doe",
      "target_name": "TechCorp",
      "relation": "works_for",
      "attrs": {"since": "2023"}
    }
  ]
}

Return only valid JSON.
`, episode.Content)
}

// parseEntity converts LLM entity data to Entity struct
func (ke *KnowledgeExtractorImpl) parseEntity(data interface{}) (*Entity, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid entity data format")
	}

	entity := &Entity{
		ID:      generateEntityID(dataMap["name"].(string)),
		Kind:    getString(dataMap, "kind", "concept"),
		Name:    getString(dataMap, "name", ""),
		Summary: getString(dataMap, "summary", ""),
		Attrs:   make(map[string]interface{}),
	}

	if attrs, ok := dataMap["attrs"].(map[string]interface{}); ok {
		entity.Attrs = attrs
	}

	return entity, nil
}

// parseEdge converts LLM edge data to Edge struct
func (ke *KnowledgeExtractorImpl) parseEdge(data interface{}, episode Episode) (*Edge, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid edge data format")
	}

	edge := &Edge{
		ID:       generateEdgeID(dataMap["source_name"].(string), dataMap["target_name"].(string), dataMap["relation"].(string)),
		SourceID: generateEntityID(dataMap["source_name"].(string)),
		TargetID: generateEntityID(dataMap["target_name"].(string)),
		Relation: getString(dataMap, "relation", "related_to"),
		Attrs:    make(map[string]interface{}),
		Provenance: map[string]interface{}{
			"extractor":  ke.config.ExtractorProvider,
			"episode_id": episode.ID,
		},
	}

	if attrs, ok := dataMap["attrs"].(map[string]interface{}); ok {
		edge.Attrs = attrs
	}

	return edge, nil
}

// deduplicateEntities removes duplicate entities based on name similarity
func (ke *KnowledgeExtractorImpl) deduplicateEntities(entities []Entity) []Entity {
	seen := make(map[string]*Entity)
	for _, entity := range entities {
		key := strings.ToLower(entity.Name)
		if existing, exists := seen[key]; exists {
			// Merge attributes if names match
			for k, v := range entity.Attrs {
				existing.Attrs[k] = v
			}
		} else {
			seen[key] = &entity
		}
	}

	var result []Entity
	for _, entity := range seen {
		result = append(result, *entity)
	}
	return result
}

// Helper functions
func getString(data map[string]interface{}, key, defaultValue string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return defaultValue
}

func generateEntityID(name string) string {
	// Simple ID generation - in practice, use UUID or hash
	return strings.ToLower(strings.ReplaceAll(name, " ", "_"))
}

func generateEdgeID(sourceName, targetName, relation string) string {
	// Simple ID generation
	return fmt.Sprintf("%s_%s_%s", sourceName, relation, targetName)
}

// ExtractionSchema defines the expected LLM output structure
type ExtractionSchema struct {
	Entities []map[string]interface{} `json:"entities"`
	Edges    []map[string]interface{} `json:"edges"`
}

// LLMClient interface for abstraction (implement based on your LLM setup)
type LLMClient interface {
	GenerateStructured(ctx context.Context, prompt string, schema interface{}) (map[string]interface{}, error)
}
