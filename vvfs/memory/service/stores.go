package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MemoryStoreImpl implements MemoryStore interface
type MemoryStoreImpl struct {
	db *sql.DB
}

// NewMemoryStoreImpl creates a new memory store
func NewMemoryStoreImpl(db *sql.DB) *MemoryStoreImpl {
	return &MemoryStoreImpl{db: db}
}

// GetItem retrieves a memory item by ID (interface method)
func (m *MemoryStoreImpl) GetItem(ctx context.Context, id string) (*MemoryItem, error) {
	return m.GetMemoryItem(ctx, id)
}

// PutItem creates or updates a memory item (interface method)
func (m *MemoryStoreImpl) PutItem(ctx context.Context, item *MemoryItem) error {
	// Try update first, fallback to create
	err := m.UpdateMemoryItem(ctx, item)
	if err != nil {
		return m.CreateMemoryItem(ctx, item)
	}
	return nil
}

// DeleteItem deletes a memory item (interface method)
func (m *MemoryStoreImpl) DeleteItem(ctx context.Context, id string) error {
	return m.DeleteMemoryItem(ctx, id)
}

// ListItems lists memory items with pagination (interface method)
func (m *MemoryStoreImpl) ListItems(ctx context.Context, opts ListOptions) ([]*MemoryItem, error) {
	return m.ListMemoryItems(ctx, opts)
}

// CreateMemoryItem inserts a new memory item
func (m *MemoryStoreImpl) CreateMemoryItem(ctx context.Context, item *MemoryItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}

	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO memory_items (id, type, text, metadata_json, embedding, created_at, expires_at, source_ref)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	var embeddingBlob []byte
	if item.Embedding != nil {
		embeddingBlob, err = encodeFloatSlice(item.Embedding)
		if err != nil {
			return fmt.Errorf("failed to encode embedding: %w", err)
		}
	}

	_, err = m.db.ExecContext(ctx, query,
		item.ID,
		item.Type,
		item.Text,
		string(metadataJSON),
		embeddingBlob,
		item.CreatedAt,
		item.ExpiresAt,
		item.SourceRef,
	)

	return err
}

// GetMemoryItem retrieves a memory item by ID
func (m *MemoryStoreImpl) GetMemoryItem(ctx context.Context, id string) (*MemoryItem, error) {
	query := `
		SELECT id, type, text, metadata_json, embedding, created_at, expires_at, source_ref
		FROM memory_items
		WHERE id = ?
	`

	var item MemoryItem
	var metadataJSON string
	var embeddingBlob []byte
	var expiresAt sql.NullTime

	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.Type,
		&item.Text,
		&metadataJSON,
		&embeddingBlob,
		&item.CreatedAt,
		&expiresAt,
		&item.SourceRef,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory item not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}

	if err := json.Unmarshal([]byte(metadataJSON), &item.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	if len(embeddingBlob) > 0 {
		item.Embedding, err = decodeFloatSlice(embeddingBlob)
		if err != nil {
			return nil, fmt.Errorf("failed to decode embedding: %w", err)
		}
	}

	return &item, nil
}

// UpdateMemoryItem updates an existing memory item
func (m *MemoryStoreImpl) UpdateMemoryItem(ctx context.Context, item *MemoryItem) error {
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var embeddingBlob []byte
	if item.Embedding != nil {
		embeddingBlob, err = encodeFloatSlice(item.Embedding)
		if err != nil {
			return fmt.Errorf("failed to encode embedding: %w", err)
		}
	}

	query := `
		UPDATE memory_items
		SET type = ?, text = ?, metadata_json = ?, embedding = ?, expires_at = ?, source_ref = ?
		WHERE id = ?
	`

	result, err := m.db.ExecContext(ctx, query,
		item.Type,
		item.Text,
		string(metadataJSON),
		embeddingBlob,
		item.ExpiresAt,
		item.SourceRef,
		item.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("memory item not found: %s", item.ID)
	}

	return nil
}

// DeleteMemoryItem deletes a memory item
func (m *MemoryStoreImpl) DeleteMemoryItem(ctx context.Context, id string) error {
	query := "DELETE FROM memory_items WHERE id = ?"
	result, err := m.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("memory item not found: %s", id)
	}

	return nil
}

// ListMemoryItems lists memory items with pagination
func (m *MemoryStoreImpl) ListMemoryItems(ctx context.Context, opts ListOptions) ([]*MemoryItem, error) {
	query := `
		SELECT id, type, text, metadata_json, embedding, created_at, expires_at, source_ref
		FROM memory_items
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := m.db.QueryContext(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*MemoryItem
	for rows.Next() {
		var item MemoryItem
		var metadataJSON string
		var embeddingBlob []byte
		var expiresAt sql.NullTime

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Text,
			&metadataJSON,
			&embeddingBlob,
			&item.CreatedAt,
			&expiresAt,
			&item.SourceRef,
		)
		if err != nil {
			return nil, err
		}

		if expiresAt.Valid {
			item.ExpiresAt = &expiresAt.Time
		}

		if err := json.Unmarshal([]byte(metadataJSON), &item.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		if len(embeddingBlob) > 0 {
			item.Embedding, err = decodeFloatSlice(embeddingBlob)
			if err != nil {
				return nil, fmt.Errorf("failed to decode embedding: %w", err)
			}
		}

		items = append(items, &item)
	}

	return items, rows.Err()
}

// SessionStoreImpl implements SessionStore interface
type SessionStoreImpl struct {
	db *sql.DB
}

// NewSessionStoreImpl creates a new session store
func NewSessionStoreImpl(db *sql.DB) *SessionStoreImpl {
	return &SessionStoreImpl{db: db}
}

// PutSession creates or updates a session (interface method)
func (s *SessionStoreImpl) PutSession(ctx context.Context, session *Session) error {
	// Try update first, fallback to create
	err := s.UpdateSession(ctx, session)
	if err != nil {
		return s.CreateSession(ctx, session)
	}
	return nil
}

// CreateSession creates a new session
func (s *SessionStoreImpl) CreateSession(ctx context.Context, session *Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	session.UpdatedAt = time.Now()

	messagesJSON, err := json.Marshal(session.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	query := `
		INSERT INTO sessions (id, messages_json, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		session.ID,
		string(messagesJSON),
		session.CreatedAt,
		session.UpdatedAt,
	)

	return err
}

// GetSession retrieves a session by ID
func (s *SessionStoreImpl) GetSession(ctx context.Context, id string) (*Session, error) {
	query := `
		SELECT id, messages_json, created_at, updated_at
		FROM sessions
		WHERE id = ?
	`

	var session Session
	var messagesJSON string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&session.ID,
		&messagesJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(messagesJSON), &session.Messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	return &session, nil
}

// UpdateSession updates a session
func (s *SessionStoreImpl) UpdateSession(ctx context.Context, session *Session) error {
	session.UpdatedAt = time.Now()

	messagesJSON, err := json.Marshal(session.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	query := `
		UPDATE sessions
		SET messages_json = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		string(messagesJSON),
		session.UpdatedAt,
		session.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	return nil
}

// DeleteSession deletes a session
func (s *SessionStoreImpl) DeleteSession(ctx context.Context, id string) error {
	query := "DELETE FROM sessions WHERE id = ?"
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session not found: %s", id)
	}

	return nil
}

// Helper functions for encoding/decoding float slices as BLOBs

func encodeFloatSlice(floats []float64) ([]byte, error) {
	// Simple encoding: convert to JSON for now
	// In production, you might use a more efficient binary format
	return json.Marshal(floats)
}

func decodeFloatSlice(data []byte) ([]float64, error) {
	var floats []float64
	err := json.Unmarshal(data, &floats)
	return floats, err
}
