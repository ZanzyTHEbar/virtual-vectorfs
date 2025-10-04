package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/adapters"
	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
	"github.com/rs/zerolog"
)

// HarnessGenerator bridges the existing Generator interface to use HarnessOrchestrator.
// This provides backward compatibility while leveraging the new harness capabilities.
type HarnessGenerator struct {
	orchestrator   *harness.HarnessOrchestrator
	conversationID string
}

// NewHarnessGenerator creates a new generator that uses the harness under the hood.
func NewHarnessGenerator(orchestrator *harness.HarnessOrchestrator, conversationID string) *HarnessGenerator {
	return &HarnessGenerator{
		orchestrator:   orchestrator,
		conversationID: conversationID,
	}
}

// Generate implements the Generator interface using the harness.
func (g *HarnessGenerator) Generate(ctx context.Context, req *GenerationRequest) (*GenerationResponse, error) {
	// Convert GenerationRequest to harness Request
	harnessReq := &harness.Request{
		Conversation: &harness.Conversation{
			ID:       g.conversationID,
			Messages: g.convertMessages(req.Messages),
		},
		System:  "", // System message should be part of conversation messages
		Context: nil,
		Tools:   nil, // Tools not part of the original Generator interface
		Policy:  harness.DefaultPolicy(),
	}

	// Execute orchestration
	resp, err := g.orchestrator.Orchestrate(ctx, harnessReq)
	if err != nil {
		return nil, fmt.Errorf("harness orchestration failed: %w", err)
	}

	// Convert harness Response back to GenerationResponse
	return &GenerationResponse{
		Text:     resp.Text,
		Messages: g.convertBackToMessages(req.Messages, resp.Text),
		Usage:    g.convertUsage(resp.Usage),
	}, nil
}

// StreamGenerate implements streaming using the harness.
func (g *HarnessGenerator) StreamGenerate(ctx context.Context, req *GenerationRequest) (<-chan *GenerationResponse, error) {
	// Convert request
	harnessReq := &harness.Request{
		Conversation: &harness.Conversation{
			ID:       g.conversationID,
			Messages: g.convertMessages(req.Messages),
		},
		System:  "",
		Context: nil,
		Tools:   nil,
		Policy:  harness.DefaultPolicy(),
	}

	// Get streaming channel
	respCh, errCh := g.orchestrator.StreamOrchestrate(ctx, harnessReq)

	resultCh := make(chan *GenerationResponse, 1)

	go func() {
		defer close(resultCh)

		select {
		case resp := <-respCh:
			if resp != nil {
				resultCh <- &GenerationResponse{
					Text:     resp.Text,
					Messages: g.convertBackToMessages(req.Messages, resp.Text),
					Usage:    g.convertUsage(resp.Usage),
				}
			}
		case err := <-errCh:
			// For now, we can't easily return errors from the channel
			// FIXME: In a real implementation, we might want to use a different pattern
			if err != nil {
				// Log the error or handle it appropriately
				fmt.Printf("Streaming error: %v\n", err)
			}
		}
	}()

	return resultCh, nil
}

// convertMessages converts from the old Message format to harness PromptMessage.
func (g *HarnessGenerator) convertMessages(messages []Message) []ports.PromptMessage {
	result := make([]ports.PromptMessage, len(messages))
	for i, msg := range messages {
		result[i] = ports.PromptMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return result
}

// convertBackToMessages reconstructs the conversation with the new response.
func (g *HarnessGenerator) convertBackToMessages(original []Message, responseText string) []Message {
	result := make([]Message, len(original)+1)
	copy(result, original)

	// Add the assistant response
	result[len(original)] = Message{
		Role:    "assistant",
		Content: responseText,
	}

	return result
}

// convertUsage converts from harness Usage to the expected format.
func (g *HarnessGenerator) convertUsage(usage *ports.Usage) *Usage {
	if usage == nil {
		return nil
	}
	return &Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

// HarnessFactory creates a harness-based generator with default components.
type HarnessFactory struct {
	orchestrator *harness.HarnessOrchestrator
}

// NewHarnessFactory creates a factory for harness-based generators.
func NewHarnessFactory(orchestrator *harness.HarnessOrchestrator) *HarnessFactory {
	return &HarnessFactory{
		orchestrator: orchestrator,
	}
}

// CreateGenerator creates a new harness-based generator for a conversation.
func (f *HarnessFactory) CreateGenerator(conversationID string) Generator {
	return NewHarnessGenerator(f.orchestrator, conversationID)
}

// DefaultHarnessFactory creates a factory with default harness components.
func DefaultHarnessFactory() (*HarnessFactory, error) {
	// Create default components
	builder := harness.NewPromptBuilder()
	assembler := harness.NewContextAssembler(
		harness.Budget{MaxContextTokens: 4000, MaxSnippets: 10},
		nil, // FIXME: Use default token estimator
	)
	store := &stubConversationStoreForBridge{} // Use stub for bridge to avoid DB dependency
	cache := adapters.NewLRUCache(1000)
	limiter := adapters.NewTokenBucket(100, 0)                      // No rate limiting for now
	tracer := adapters.NewZerologTracer(zerolog.New(zerolog.Nop())) // Use no-op logger for bridge

	// FIXME: Create stub provider for compatibility
	provider := &stubProviderForBridge{}

	orchestrator := harness.NewHarnessOrchestrator(
		provider,
		builder,
		assembler,
		store,
		cache,
		limiter,
		tracer,
	)

	return NewHarnessFactory(orchestrator), nil
}

// stubProviderForBridge provides a minimal provider implementation for the bridge.
// FIXME: In a real implementation, this would use actual LLM providers.
type stubProviderForBridge struct{}

func (p *stubProviderForBridge) Complete(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error) {
	// FIXME: This is a stub - in real usage, this would call an actual LLM provider
	return ports.Completion{
		Text: "This is a bridge response - LLM provider not configured",
		Usage: &ports.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (p *stubProviderForBridge) Stream(ctx context.Context, in ports.PromptInput, opts ports.Options) (<-chan ports.CompletionChunk, error) {
	ch := make(chan ports.CompletionChunk, 1)
	ch <- ports.CompletionChunk{
		DeltaText: "This is a bridge response - LLM provider not configured",
		Done:      true,
	}
	close(ch)
	return ch, nil
}

// stubConversationStoreForBridge implements ConversationStore for the bridge.
type stubConversationStoreForBridge struct {
	turns map[string][]ports.Turn
}

func (s *stubConversationStoreForBridge) SaveTurn(ctx context.Context, conversationID string, turn ports.Turn) error {
	if s.turns == nil {
		s.turns = make(map[string][]ports.Turn)
	}
	s.turns[conversationID] = append(s.turns[conversationID], turn)
	return nil
}

func (s *stubConversationStoreForBridge) LoadContext(ctx context.Context, conversationID string, k int) ([]ports.Turn, error) {
	turns, exists := s.turns[conversationID]
	if !exists {
		return nil, nil
	}

	if k <= 0 || k >= len(turns) {
		return turns, nil
	}

	return turns[len(turns)-k:], nil
}

func (s *stubConversationStoreForBridge) AppendToolArtifact(ctx context.Context, conversationID, name string, payload []byte) error {
	return s.SaveTurn(ctx, conversationID, ports.Turn{
		Role:      "tool",
		Content:   string(payload),
		CreatedAt: time.Now(),
	})
}

// Ensure stubConversationStoreForBridge implements the ConversationStore interface.
var _ ports.ConversationStore = (*stubConversationStoreForBridge)(nil)

// Ensure stubProviderForBridge implements the Provider interface.
var _ ports.Provider = (*stubProviderForBridge)(nil)
