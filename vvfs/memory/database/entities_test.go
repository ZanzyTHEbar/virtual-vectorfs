package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/tursodatabase/go-libsql"
)

func TestEntitiesCRUD(t *testing.T) {
	// Create a test database
	db := createTestDB(t)
	defer db.Close()

	// Create a querier
	ctx := context.Background()
	queries, err := Prepare(ctx, db)
	require.NoError(t, err)
	defer queries.Close()

	// Prepare an embedding matching F32_BLOB(728) => 728 float32 => 728*4 bytes
	embed := make([]byte, 728*4)

	// Test Create Entity
	entityName := "test-entity"
	entityType := "person"
	metadata := `{"tags": ["test"], "created_at": "2024-01-01"}`
	createdAt := time.Now().Unix()
	updatedAt := time.Now().Unix()

	entity, err := queries.CreateEntity(ctx, CreateEntityParams{
		Name:       entityName,
		EntityType: entityType,
		Embedding:  embed,
		Metadata:   metadata,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	})
	require.NoError(t, err)
	assert.Equal(t, entityName, entity.Name)
	t.Logf("Created entity: %s, Type: %s", entity.Name, entity.EntityType)

	// Test Get Entity
	entity, err = queries.GetEntity(ctx, entityName)
	require.NoError(t, err)
	assert.Equal(t, entityName, entity.Name)
	assert.Equal(t, entityType, entity.EntityType)
	assert.Equal(t, metadata, entity.Metadata)
	t.Logf("Retrieved entity: %s, Type: %s", entity.Name, entity.EntityType)

	// Test Get Entities By Type
	entities, err := queries.GetEntitiesByType(ctx, entityType)
	require.NoError(t, err)
	assert.Len(t, entities, 1)
	assert.Equal(t, entityName, entities[0].Name)
	t.Logf("GetEntitiesByType found %d entities for type '%s'", len(entities), entityType)

	// Test Search Entities by type
	searchResults, err := queries.SearchEntities(ctx, SearchEntitiesParams{
		Column1: sql.NullString{String: "person", Valid: true}, // entity_type filter (person)
		Column2: sql.NullString{String: "", Valid: false},      // name pattern (empty)
		Limit:   10,                                            // limit
		Offset:  0,
	})
	require.NoError(t, err)
	t.Logf("Search by type 'person' returned %d results", len(searchResults))
	assert.Len(t, searchResults, 1)

	// Test Search Entities by name
	nameSearchResults, err := queries.SearchEntities(ctx, SearchEntitiesParams{
		Column1: sql.NullString{String: "", Valid: false},        // entity_type filter (empty)
		Column2: sql.NullString{String: entityName, Valid: true}, // name pattern (test-entity)
		Limit:   10,                                              // limit
		Offset:  0,
	})
	require.NoError(t, err)
	assert.Len(t, nameSearchResults, 1)

	// Test Update Entity
	newMetadata := `{"tags": ["test", "updated"], "created_at": "2024-01-01"}`
	updatedEntity, err := queries.UpdateEntity(ctx, UpdateEntityParams{
		Name:      entityName,
		Metadata:  newMetadata,
		UpdatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	assert.Equal(t, newMetadata, updatedEntity.Metadata)

	// Verify update
	updatedEntity, err = queries.GetEntity(ctx, entityName)
	require.NoError(t, err)
	assert.Equal(t, newMetadata, updatedEntity.Metadata)

	// Test Delete Entity
	err = queries.DeleteEntity(ctx, entityName)
	require.NoError(t, err)

	// Verify deletion
	_, err = queries.GetEntity(ctx, entityName)
	assert.Error(t, err) // Should return error for not found
}

func TestEntitiesSearch(t *testing.T) {
	// Create a test database
	db := createTestDB(t)
	defer db.Close()

	ctx := context.Background()
	queries := New(db)

	embed := make([]byte, 728*4)

	// Create test entities
	testEntities := []struct {
		name       string
		entityType string
		metadata   string
	}{
		{"person1", "person", `{"tags": ["developer"]}`},
		{"person2", "person", `{"tags": ["designer"]}`},
		{"company1", "company", `{"tags": ["tech"]}`},
		{"project1", "project", `{"tags": ["open-source"]}`},
	}

	for _, entity := range testEntities {
		_, err := queries.CreateEntity(ctx, CreateEntityParams{
			Name:       entity.name,
			EntityType: entity.entityType,
			Embedding:  embed,
			Metadata:   entity.metadata,
			CreatedAt:  time.Now().Unix(),
			UpdatedAt:  time.Now().Unix(),
		})
		require.NoError(t, err)
	}

	// Test search by type
	persons, err := queries.GetEntitiesByType(ctx, "person")
	require.NoError(t, err)
	assert.Len(t, persons, 2)

	companies, err := queries.GetEntitiesByType(ctx, "company")
	require.NoError(t, err)
	assert.Len(t, companies, 1)

	// Test search with partial name match
	partialNameResults, err := queries.SearchEntities(ctx, SearchEntitiesParams{
		Column1: sql.NullString{String: "", Valid: false},      // no type filter
		Column2: sql.NullString{String: "person", Valid: true}, // name contains "person"
		Limit:   10,                                            // limit
		Offset:  0,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(partialNameResults), 2) // at least person1 and person2
}

func TestEntityWithObservations(t *testing.T) {
	// Create a test database
	db := createTestDB(t)
	defer db.Close()

	ctx := context.Background()
	queries := New(db)

	embed := make([]byte, 728*4)

	// Create an entity
	entityName := "test-entity-with-obs"
	_, err := queries.CreateEntity(ctx, CreateEntityParams{
		Name:       entityName,
		EntityType: "person",
		Embedding:  embed,
		Metadata:   `{"tags": ["test"]}`,
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	})
	require.NoError(t, err)

	// Create observations for the entity
	observationContent := "This is a test observation"
	_, err = queries.CreateObservation(ctx, CreateObservationParams{
		EntityName: entityName,
		Content:    observationContent,
		Embedding:  embed,
		CreatedAt:  time.Now().Unix(),
	})
	require.NoError(t, err)

	// Test GetEntityWithObservations
	result, err := queries.GetEntityWithObservations(ctx, GetEntityWithObservationsParams{
		Name:       entityName,
		EntityName: entityName,
		Limit:      10,
	})
	require.NoError(t, err)
	assert.Equal(t, entityName, result.Name)
	assert.Greater(t, result.ObservationCount, int64(0))
	assert.Contains(t, result.ObservationsContent, observationContent)
}

// Helper function to create test database
func createTestDB(t *testing.T) *sql.DB {
	// Use in-memory libSQL for testing
	db, err := sql.Open("libsql", "file::memory:?cache=shared")
	require.NoError(t, err)

	// Run migrations with Turso/libSQL dialect using Provider API
	migrationsDir := "../migrations"
	provider, err := goose.NewProvider(goose.DialectTurso, db, os.DirFS(migrationsDir), goose.WithVerbose(true))
	if err != nil {
		t.Fatalf("Failed to create goose provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return db
}
