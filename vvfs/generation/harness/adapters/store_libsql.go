package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

// LibSQLConversationStore implements ConversationStore using LibSQL (via memory service).
type LibSQLConversationStore struct {
	db *sql.DB
}

// NewLibSQLConversationStore creates a new LibSQL conversation store.
func NewLibSQLConversationStore(db *sql.DB) *LibSQLConversationStore {
	return &LibSQLConversationStore{
		db: db,
	}
}

// SaveTurn saves a conversation turn to the database.
func (s *LibSQLConversationStore) SaveTurn(ctx context.Context, conversationID string, turn ports.Turn) error {
	// Convert turn to JSON
	turnJSON, err := json.Marshal(turn)
	if err != nil {
		return fmt.Errorf("failed to marshal turn: %w", err)
	}

	// Insert or replace turn
	query := `
		INSERT OR REPLACE INTO conversation_turns (conversation_id, turn_data, created_at)
		VALUES (?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query, conversationID, string(turnJSON), time.Now())
	if err != nil {
		return fmt.Errorf("failed to save turn: %w", err)
	}

	return nil
}

// LoadContext loads the last k turns for a conversation.
func (s *LibSQLConversationStore) LoadContext(ctx context.Context, conversationID string, k int) ([]ports.Turn, error) {
	query := `
		SELECT turn_data FROM conversation_turns
		WHERE conversation_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, conversationID, k)
	if err != nil {
		return nil, fmt.Errorf("failed to query turns: %w", err)
	}
	defer rows.Close()

	var turns []ports.Turn
	for rows.Next() {
		var turnJSON string
		if err := rows.Scan(&turnJSON); err != nil {
			return nil, fmt.Errorf("failed to scan turn: %w", err)
		}

		var turn ports.Turn
		if err := json.Unmarshal([]byte(turnJSON), &turn); err != nil {
			return nil, fmt.Errorf("failed to unmarshal turn: %w", err)
		}

		turns = append(turns, turn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating turns: %w", err)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}

	return turns, nil
}

// AppendToolArtifact appends tool execution results to the conversation.
func (s *LibSQLConversationStore) AppendToolArtifact(ctx context.Context, conversationID, name string, payload []byte) error {
	// Create a tool turn
	toolTurn := ports.Turn{
		Role:      "tool",
		Content:   fmt.Sprintf("Tool %s executed: %s", name, string(payload)),
		CreatedAt: time.Now(),
	}

	return s.SaveTurn(ctx, conversationID, toolTurn)
}

// Ensure LibSQLConversationStore implements the ConversationStore interface.
var _ ports.ConversationStore = (*LibSQLConversationStore)(nil)
