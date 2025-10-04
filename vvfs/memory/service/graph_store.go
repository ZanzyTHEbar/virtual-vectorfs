package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// GraphStoreImpl implements GraphStore using SQL database
type GraphStoreImpl struct {
	db *sql.DB
}

// NewGraphStore creates a new graph store
func NewGraphStore(db *sql.DB) *GraphStoreImpl {
	return &GraphStoreImpl{db: db}
}

// GetEntity retrieves an entity by ID
func (gs *GraphStoreImpl) GetEntity(ctx context.Context, id string) (*Entity, error) {
	query := `
		SELECT id, kind, name, summary, attrs_json, created_at, updated_at
		FROM entities
		WHERE id = $1
	`

	entity := &Entity{}
	var attrsJSON string
	err := gs.db.QueryRowContext(ctx, query, id).Scan(
		&entity.ID, &entity.Kind, &entity.Name, &entity.Summary,
		&attrsJSON, &entity.CreatedAt, &entity.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entity not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	if err := json.Unmarshal([]byte(attrsJSON), &entity.Attrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attrs: %w", err)
	}

	return entity, nil
}

// UpsertEntity inserts or updates an entity
func (gs *GraphStoreImpl) UpsertEntity(ctx context.Context, entity *Entity) error {
	// Check if entity exists
	existing, err := gs.GetEntity(ctx, entity.ID)
	if err != nil && err.Error() != "entity not found: "+entity.ID {
		return err
	}

	attrsJSON, err := json.Marshal(entity.Attrs)
	if err != nil {
		return fmt.Errorf("failed to marshal attrs: %w", err)
	}

	if existing != nil {
		// Update existing entity
		query := `
			UPDATE entities
			SET kind = $2, name = $3, summary = $4, attrs_json = $5, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`
		_, err = gs.db.ExecContext(ctx, query, entity.ID, entity.Kind, entity.Name, entity.Summary, string(attrsJSON))
	} else {
		// Insert new entity
		query := `
			INSERT INTO entities (id, kind, name, summary, attrs_json, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`
		_, err = gs.db.ExecContext(ctx, query, entity.ID, entity.Kind, entity.Name, entity.Summary, string(attrsJSON))
	}

	if err != nil {
		return fmt.Errorf("failed to upsert entity: %w", err)
	}

	return nil
}

// DeleteEntity removes an entity
func (gs *GraphStoreImpl) DeleteEntity(ctx context.Context, id string) error {
	query := `DELETE FROM entities WHERE id = $1`
	_, err := gs.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}
	return nil
}

// ListEntities retrieves entities with pagination and filtering
func (gs *GraphStoreImpl) ListEntities(ctx context.Context, opts ListOptions) ([]*Entity, error) {
	query := `
		SELECT id, kind, name, summary, attrs_json, created_at, updated_at
		FROM entities
	`

	// Apply filters (simplified - can be extended)
	if filter := opts.Filter; filter != nil {
		// Example: filter by kind
		if _, ok := filter["kind"]; ok {
			query += " WHERE kind = $1"
			// Note: This is a simplified example; in practice, use proper query building
		}
	}

	// Apply sorting
	if opts.Sort != "" {
		query += " ORDER BY " + opts.Sort
	} else {
		query += " ORDER BY created_at DESC"
	}

	// Apply pagination
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := gs.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}
	defer rows.Close()

	var entities []*Entity
	for rows.Next() {
		entity := &Entity{}
		var attrsJSON string
		err := rows.Scan(
			&entity.ID, &entity.Kind, &entity.Name, &entity.Summary,
			&attrsJSON, &entity.CreatedAt, &entity.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entity: %w", err)
		}

		if err := json.Unmarshal([]byte(attrsJSON), &entity.Attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attrs: %w", err)
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// GetEdge retrieves an edge by ID
func (gs *GraphStoreImpl) GetEdge(ctx context.Context, id string) (*Edge, error) {
	query := `
		SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
		FROM edges
		WHERE id = $1
	`

	edge := &Edge{}
	var attrsJSON, provenanceJSON string
	var validTo, invalidatedAt sql.NullTime

	err := gs.db.QueryRowContext(ctx, query, id).Scan(
		&edge.ID, &edge.SourceID, &edge.TargetID, &edge.Relation,
		&attrsJSON, &edge.ValidFrom, &validTo, &edge.IngestedAt, &invalidatedAt, &provenanceJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("edge not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	if validTo.Valid {
		edge.ValidTo = &validTo.Time
	}
	if invalidatedAt.Valid {
		edge.InvalidatedAt = &invalidatedAt.Time
	}

	if err := json.Unmarshal([]byte(attrsJSON), &edge.Attrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attrs: %w", err)
	}
	if err := json.Unmarshal([]byte(provenanceJSON), &edge.Provenance); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provenance: %w", err)
	}

	return edge, nil
}

// UpsertEdge inserts or updates an edge
func (gs *GraphStoreImpl) UpsertEdge(ctx context.Context, edge *Edge) error {
	attrsJSON, err := json.Marshal(edge.Attrs)
	if err != nil {
		return fmt.Errorf("failed to marshal attrs: %w", err)
	}
	provenanceJSON, err := json.Marshal(edge.Provenance)
	if err != nil {
		return fmt.Errorf("failed to marshal provenance: %w", err)
	}

	query := `
		INSERT INTO edges (id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			rel = EXCLUDED.rel,
			attrs_json = EXCLUDED.attrs_json,
			valid_to = EXCLUDED.valid_to,
			invalidated_at = EXCLUDED.invalidated_at,
			provenance_json = EXCLUDED.provenance_json
	`

	var validToPtr *time.Time
	if edge.ValidTo != nil {
		validToPtr = edge.ValidTo
	}

	var invalidatedAtPtr *time.Time
	if edge.InvalidatedAt != nil {
		invalidatedAtPtr = edge.InvalidatedAt
	}

	_, err = gs.db.ExecContext(ctx, query,
		edge.ID, edge.SourceID, edge.TargetID, edge.Relation, string(attrsJSON),
		edge.ValidFrom, validToPtr, edge.IngestedAt, invalidatedAtPtr, string(provenanceJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert edge: %w", err)
	}

	return nil
}

// InvalidateEdge marks an edge as invalidated
func (gs *GraphStoreImpl) InvalidateEdge(ctx context.Context, id string, reason string) error {
	query := `
		UPDATE edges
		SET invalidated_at = CURRENT_TIMESTAMP, valid_to = CURRENT_TIMESTAMP
		WHERE id = $1 AND invalidated_at IS NULL
	`

	result, err := gs.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to invalidate edge: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("edge not found or already invalidated: %s", id)
	}

	return nil
}

// ListEdges retrieves edges with pagination and filtering
func (gs *GraphStoreImpl) ListEdges(ctx context.Context, opts ListOptions) ([]*Edge, error) {
	query := `
		SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
		FROM edges
	`

	// Apply filters (simplified)
	if filter := opts.Filter; filter != nil {
		// Example: filter by source entity
		if _, ok := filter["src_id"]; ok {
			query += " WHERE src_id = $1"
		}
	}

	// Apply sorting
	if opts.Sort != "" {
		query += " ORDER BY " + opts.Sort
	} else {
		query += " ORDER BY ingested_at DESC"
	}

	// Apply pagination
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := gs.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list edges: %w", err)
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		edge := &Edge{}
		var attrsJSON, provenanceJSON string
		var validTo, invalidatedAt sql.NullTime

		err := rows.Scan(
			&edge.ID, &edge.SourceID, &edge.TargetID, &edge.Relation,
			&attrsJSON, &edge.ValidFrom, &validTo, &edge.IngestedAt, &invalidatedAt, &provenanceJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan edge: %w", err)
		}

		if validTo.Valid {
			edge.ValidTo = &validTo.Time
		}
		if invalidatedAt.Valid {
			edge.InvalidatedAt = &invalidatedAt.Time
		}

		if err := json.Unmarshal([]byte(attrsJSON), &edge.Attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attrs: %w", err)
		}
		if err := json.Unmarshal([]byte(provenanceJSON), &edge.Provenance); err != nil {
			return nil, fmt.Errorf("failed to unmarshal provenance: %w", err)
		}

		edges = append(edges, edge)
	}

	return edges, nil
}

// GetEdgesAsOf retrieves edges valid at a specific time
func (gs *GraphStoreImpl) GetEdgesAsOf(ctx context.Context, timepoint time.Time, opts ListOptions) ([]*Edge, error) {
	// Use the edges_asof view
	query := `
		SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
		FROM edges_asof($1)
	`

	// Apply additional filters if needed
	if filter := opts.Filter; filter != nil {
		// Extend query with filters
	}

	// Apply sorting and pagination
	if opts.Sort != "" {
		query += " ORDER BY " + opts.Sort
	}
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := gs.db.QueryContext(ctx, query, timepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges as of: %w", err)
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		// Similar scanning logic as ListEdges
		edge := &Edge{}
		// ... (scan logic)
		edges = append(edges, edge)
	}

	return edges, nil
}

// GetCurrentEdges retrieves currently valid edges
func (gs *GraphStoreImpl) GetCurrentEdges(ctx context.Context, opts ListOptions) ([]*Edge, error) {
	// Use the edges_current view
	query := `
		SELECT id, src_id, dst_id, rel, attrs_json, valid_from, valid_to, ingested_at, invalidated_at, provenance_json
		FROM edges_current
	`

	// Apply additional filters if needed
	if filter := opts.Filter; filter != nil {
		// Extend query with filters
	}

	// Apply sorting and pagination
	if opts.Sort != "" {
		query += " ORDER BY " + opts.Sort
	}
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := gs.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get current edges: %w", err)
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		// Similar scanning logic as ListEdges
		edge := &Edge{}
		// ... (scan logic)
		edges = append(edges, edge)
	}

	return edges, nil
}
