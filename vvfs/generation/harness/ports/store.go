package harnessports

import (
	"context"
	"time"
)

// Turn represents a conversational exchange.
type Turn struct {
	Role      string    // "user" | "assistant" | "system" | "tool"
	Content   string    // text or JSON string (for tool outputs)
	CreatedAt time.Time // server-side timestamp
}

// ConversationStore persists conversation context and tool artifacts.
type ConversationStore interface {
	SaveTurn(ctx context.Context, conversationID string, turn Turn) error
	LoadContext(ctx context.Context, conversationID string, k int) ([]Turn, error) // last-k turns
	AppendToolArtifact(ctx context.Context, conversationID, name string, payload []byte) error
}
