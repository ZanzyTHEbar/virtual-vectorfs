package service

import (
	"context"
	"testing"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockKnowledgeExtractor for testing
type MockKnowledgeExtractor struct {
	mock.Mock
}

func (m *MockKnowledgeExtractor) Extract(ctx context.Context, episode Episode) (*ExtractionResult, error) {
	args := m.Called(ctx, episode)
	return args.Get(0).(*ExtractionResult), args.Error(1)
}

// TestGraphIngester_IngestEpisode tests basic ingestion
func TestGraphIngester_IngestEpisode(t *testing.T) {
	mockExtractor := new(MockKnowledgeExtractor)
	mockStore := new(MockGraphStore)

	// Setup expectations
	expectedEntities := []Entity{
		{ID: "entity1", Kind: "person", Name: "John Doe"},
	}
	expectedEdges := []Edge{
		{ID: "edge1", SourceID: "entity1", TargetID: "entity2", Relation: "works_with"},
	}
	extractResult := &ExtractionResult{
		Entities: expectedEntities,
		Edges:    expectedEdges,
	}

	mockExtractor.On("Extract", mock.Anything, mock.Anything).Return(extractResult, nil)
	mockStore.On("UpsertEntity", mock.Anything, mock.Anything).Return(nil)
	mockStore.On("UpsertEdge", mock.Anything, mock.Anything).Return(nil)

	config := &config.MemoryConfig{}
	ingester := NewGraphIngester(mockExtractor, mockStore, config)

	episode := Episode{
		ID:      "episode1",
		Content: "John Doe works with Jane Smith on the project.",
	}

	err := ingester.IngestEpisode(context.Background(), episode)
	assert.NoError(t, err)

	mockExtractor.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

// TestGraphIngester_BatchIngest tests batch processing
func TestGraphIngester_BatchIngest(t *testing.T) {
	mockExtractor := new(MockKnowledgeExtractor)
	mockStore := new(MockGraphStore)

	// Setup expectations for multiple episodes
	episodes := []Episode{
		{ID: "ep1", Content: "Content 1"},
		{ID: "ep2", Content: "Content 2"},
	}

	for _, ep := range episodes {
		mockExtractor.On("Extract", mock.Anything, ep).Return(&ExtractionResult{}, nil)
		mockStore.On("UpsertEntity", mock.Anything, mock.Anything).Return(nil)
		mockStore.On("UpsertEdge", mock.Anything, mock.Anything).Return(nil)
	}

	config := &config.MemoryConfig{}
	ingester := NewGraphIngester(mockExtractor, mockStore, config)

	err := ingester.BatchIngest(context.Background(), episodes)
	assert.NoError(t, err)

	mockExtractor.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}
