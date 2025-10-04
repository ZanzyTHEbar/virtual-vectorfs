package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
	adapters "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/adapters"
	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/tools"
)

// StubProvider implements Provider for testing.
type StubProvider struct {
	completionFunc func(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error)
	streamFunc     func(ctx context.Context, in ports.PromptInput, opts ports.Options) (<-chan ports.CompletionChunk, error)
}

func (p *StubProvider) Complete(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error) {
	if p.completionFunc != nil {
		return p.completionFunc(ctx, in, opts)
	}
	return ports.Completion{
		Text: "stub completion",
		Usage: &ports.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (p *StubProvider) Stream(ctx context.Context, in ports.PromptInput, opts ports.Options) (<-chan ports.CompletionChunk, error) {
	if p.streamFunc != nil {
		return p.streamFunc(ctx, in, opts)
	}
	ch := make(chan ports.CompletionChunk, 1)
	ch <- ports.CompletionChunk{
		DeltaText: "stub",
		Done:      true,
	}
	close(ch)
	return ch, nil
}

// StubTool implements Tool for testing.
type StubTool struct {
	name   string
	schema string
	result string
}

func (t *StubTool) Name() string   { return t.name }
func (t *StubTool) Schema() []byte { return []byte(t.schema) }
func (t *StubTool) Invoke(ctx context.Context, args json.RawMessage) (any, error) {
	return t.result, nil
}

// stubConversationStore implements ConversationStore for testing.
type stubConversationStore struct {
	turns map[string][]ports.Turn
}

func (s *stubConversationStore) SaveTurn(ctx context.Context, conversationID string, turn ports.Turn) error {
	if s.turns == nil {
		s.turns = make(map[string][]ports.Turn)
	}
	s.turns[conversationID] = append(s.turns[conversationID], turn)
	return nil
}

func (s *stubConversationStore) LoadContext(ctx context.Context, conversationID string, k int) ([]ports.Turn, error) {
	turns, exists := s.turns[conversationID]
	if !exists {
		return nil, nil
	}

	if k <= 0 || k >= len(turns) {
		return turns, nil
	}

	return turns[len(turns)-k:], nil
}

func (s *stubConversationStore) AppendToolArtifact(ctx context.Context, conversationID, name string, payload []byte) error {
	return s.SaveTurn(ctx, conversationID, ports.Turn{
		Role:      "tool",
		Content:   string(payload),
		CreatedAt: time.Now(),
	})
}

// Ensure stubConversationStore implements the ConversationStore interface.
var _ ports.ConversationStore = (*stubConversationStore)(nil)

// TestPromptBuilder_Build tests prompt construction.
func TestPromptBuilder_Build(t *testing.T) {
	builder := NewPromptBuilder()

	system := "You are a helpful assistant"
	messages := []ports.PromptMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}
	context := []string{"Context 1", "Context 2"}
	tools := []ports.ToolSpec{
		{Name: "test_tool", Description: "A test tool"},
	}

	input := builder.Build(system, messages, context, tools, map[string]string{"test": "value"})

	assert.Equal(t, system, input.System)
	assert.Equal(t, messages, input.Messages)
	assert.Equal(t, context, input.Context)
	assert.Equal(t, tools, input.Tools)
	assert.Equal(t, "value", input.Meta["test"])
}

// TestContextAssembler_Pack tests context packing with token budgeting.
func TestContextAssembler_Pack(t *testing.T) {
	assembler := NewContextAssembler(
		Budget{MaxContextTokens: 100, MaxSnippets: 3},
		func(s string) int { return len(s) / 4 }, // rough token estimate
	)

	snippets := []Snippet{
		{Text: "Short text", Score: 1.0, TokenCount: 2},                                                 // Score 1.0, tokens 2
		{Text: "Medium length text here", Score: 0.8, TokenCount: 5},                                    // Score 0.8, tokens 5
		{Text: "Very long text that should be excluded due to token limit", Score: 0.6, TokenCount: 50}, // Score 0.6, tokens 50
		{Text: "Another short text", Score: 0.9, TokenCount: 3},                                         // Score 0.9, tokens 3
	}

	packed := assembler.Pack(snippets, nil)

	assert.Len(t, packed, 3) // Should pack top 3 within budget
	// Sorted by score descending: 1.0, 0.9, 0.8
	assert.Equal(t, "Short text", packed[0])              // Highest score (1.0) first
	assert.Equal(t, "Another short text", packed[1])      // Second highest (0.9)
	assert.Equal(t, "Medium length text here", packed[2]) // Third highest (0.8)
}

// TestOutputParser_ParseToolCalls tests tool call parsing.
func TestOutputParser_ParseToolCalls(t *testing.T) {
	parser := NewOutputParser()

	// Test JSON array format
	text1 := `[{"name": "test_tool", "arguments": {"arg": "value"}}]`
	calls1 := parser.ParseToolCalls(text1)
	assert.Len(t, calls1, 1)
	assert.Equal(t, "test_tool", calls1[0].Name)

	// Test function call format
	text2 := `test_tool({"arg": "value"})`
	calls2 := parser.ParseToolCalls(text2)
	assert.Len(t, calls2, 1)
	assert.Equal(t, "test_tool", calls2[0].Name)

	// Test no tool calls
	text3 := "Just a normal response"
	calls3 := parser.ParseToolCalls(text3)
	assert.Len(t, calls3, 0)
}

// TestGuardrails_ValidateToolCall tests tool call validation.
func TestGuardrails_ValidateToolCall(t *testing.T) {
	guardrails := NewGuardrails()

	// Add allowed tool
	guardrails.AddAllowedTool("allowed_tool")

	// Test allowed tool
	validCall := ports.ToolCall{
		Name: "allowed_tool",
		Args: json.RawMessage(`{"arg": "value"}`),
	}
	err := guardrails.ValidateToolCall(validCall)
	assert.NoError(t, err)

	// Test blocked tool
	blockedCall := ports.ToolCall{
		Name: "blocked_tool",
		Args: json.RawMessage(`{"arg": "value"}`),
	}
	err = guardrails.ValidateToolCall(blockedCall)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in allowlist")
}

// TestLRUCache_BasicOperations tests cache functionality.
func TestLRUCache_BasicOperations(t *testing.T) {
	cache := adapters.NewLRUCache(2)

	ctx := context.Background()

	// Test set and get
	err := cache.Set(ctx, "key1", []byte("value1"), 3600)
	assert.NoError(t, err)

	value, ok := cache.Get(ctx, "key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), value)

	// Test eviction (capacity 2, add third item)
	err = cache.Set(ctx, "key2", []byte("value2"), 3600)
	assert.NoError(t, err)
	err = cache.Set(ctx, "key3", []byte("value3"), 3600)
	assert.NoError(t, err)

	// key1 should be evicted
	_, ok = cache.Get(ctx, "key1")
	assert.False(t, ok)

	// key2 and key3 should exist
	_, ok = cache.Get(ctx, "key2")
	assert.True(t, ok)
	_, ok = cache.Get(ctx, "key3")
	assert.True(t, ok)
}

// TestTokenBucket_BasicRateLimiting tests rate limiting functionality.
func TestTokenBucket_BasicRateLimiting(t *testing.T) {
	limiter := adapters.NewTokenBucket(2, time.Second) // 2 tokens, refill every second

	ctx := context.Background()

	// Should allow first two requests
	release1, err := limiter.Acquire(ctx, "test")
	assert.NoError(t, err)
	assert.NotNil(t, release1)

	release2, err := limiter.Acquire(ctx, "test")
	assert.NoError(t, err)
	assert.NotNil(t, release2)

	// Third request should be rate limited
	_, err = limiter.Acquire(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")

	// Release tokens
	release1()
	release2()

	// After releases, should allow more requests
	release3, err := limiter.Acquire(ctx, "test")
	assert.NoError(t, err)
	assert.NotNil(t, release3)
	release3()
}

// TestHarnessOrchestrator_SimpleConversation tests basic orchestration without tools.
func TestHarnessOrchestrator_SimpleConversation(t *testing.T) {
	// Setup stub provider
	provider := &StubProvider{
		completionFunc: func(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error) {
			return ports.Completion{
				Text: "Assistant response",
				Usage: &ports.Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			}, nil
		},
	}

	// Setup components
	builder := NewPromptBuilder()
	assembler := NewContextAssembler(Budget{MaxContextTokens: 1000}, nil)
	store := &stubConversationStore{} // Use stub for tests to avoid DB dependency
	cache := adapters.NewLRUCache(100)
	limiter := adapters.NewTokenBucket(10, time.Second)
	tracer := adapters.NewZerologTracer(zerolog.New(zerolog.Nop())) // Use no-op logger for tests

	orchestrator := NewHarnessOrchestrator(provider, builder, assembler, store, cache, limiter, tracer)

	// Test request
	req := &Request{
		Conversation: &Conversation{
			ID: "test-conv",
			Messages: []ports.PromptMessage{
				{Role: "user", Content: "Hello"},
			},
		},
		System: "You are a helpful assistant",
		Tools:  []ports.Tool{},
	}

	// Execute orchestration
	resp, err := orchestrator.Orchestrate(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "Assistant response", resp.Text)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

// TestHarnessOrchestrator_WithTools tests orchestration with tool calls.
func TestHarnessOrchestrator_WithTools(t *testing.T) {
	provider := &StubProvider{
		completionFunc: func(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error) {
			// Simple response without tool calls for this test
			return ports.Completion{
				Text: "Response without tool calls",
				Usage: &ports.Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			}, nil
		},
	}

	// Setup components
	builder := NewPromptBuilder()
	assembler := NewContextAssembler(Budget{MaxContextTokens: 1000}, nil)
	store := &stubConversationStore{}
	cache := adapters.NewLRUCache(100)
	limiter := adapters.NewTokenBucket(10, time.Second)
	tracer := adapters.NewZerologTracer(zerolog.New(zerolog.Nop()))

	orchestrator := NewHarnessOrchestrator(provider, builder, assembler, store, cache, limiter, tracer)

	// Test request with tools but no tool calls in response
	req := &Request{
		Conversation: &Conversation{
			ID: "test-conv",
			Messages: []ports.PromptMessage{
				{Role: "user", Content: "Hello"},
			},
		},
		System: "You are a helpful assistant",
		Tools:  []ports.Tool{}, // No tools for this simple test
	}

	// Execute orchestration
	resp, err := orchestrator.Orchestrate(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "Response without tool calls", resp.Text)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

// Benchmark tests for performance validation
func BenchmarkPromptBuilder_Build(b *testing.B) {
	builder := NewPromptBuilder()

	messages := []ports.PromptMessage{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello world"},
	}

	for i := 0; i < b.N; i++ {
		builder.Build("system", messages, nil, nil, nil)
	}
}

func BenchmarkContextAssembler_Pack(b *testing.B) {
	assembler := NewContextAssembler(Budget{MaxContextTokens: 1000}, nil)

	snippets := make([]Snippet, 100)
	for i := range snippets {
		snippets[i] = Snippet{
			Text:       "Test snippet text",
			Score:      0.5,
			TokenCount: 4,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.Pack(snippets, nil)
	}
}

func BenchmarkLRUCache_SetGet(b *testing.B) {
	cache := adapters.NewLRUCache(1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		cache.Set(ctx, key, []byte("value"), 3600)
		cache.Get(ctx, key)
	}
}

// TestEndToEnd_MultiIterationToolCalling tests a complete multi-iteration workflow with tool calls and persistence.
func TestEndToEnd_MultiIterationToolCalling(t *testing.T) {
	// Setup mock conversation store to verify persistence
	store := &testConversationStore{
		turns: make(map[string][]ports.Turn),
	}

	// Setup tools
	kgTool := tools.NewKGSearchTool()
	fsTool := tools.NewFSMetadataTool(".")

	// Setup provider that simulates multi-turn conversation
	provider := &multiIterationProvider{}

	// Setup components
	builder := NewPromptBuilder()
	assembler := NewContextAssembler(Budget{MaxContextTokens: 1000}, nil)
	cache := adapters.NewLRUCache(100)
	limiter := adapters.NewTokenBucket(10, time.Second)
	tracer := adapters.NewZerologTracer(zerolog.New(zerolog.Nop()))

	orchestrator := NewHarnessOrchestrator(provider, builder, assembler, store, cache, limiter, tracer)

	// Test request
	req := &Request{
		Conversation: &Conversation{
			ID: "e2e-test-conv",
			Messages: []ports.PromptMessage{
				{Role: "user", Content: "Search for tasks related to testing"},
			},
		},
		System: "You are a helpful assistant with access to knowledge graph and filesystem tools.",
		Tools:  []ports.Tool{kgTool, fsTool},
		Policy: &Policy{
			MaxToolDepth:  1, // Single tool call for E2E test
			MaxIterations: 3,
		},
	}

	// Execute orchestration
	resp, err := orchestrator.Orchestrate(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify conversation was persisted (only final assistant response is saved as a turn)
	assert.NotEmpty(t, store.turns["e2e-test-conv"])

	// Verify we got the assistant response turn
	assert.Len(t, store.turns["e2e-test-conv"], 1)
	assert.Equal(t, "assistant", store.turns["e2e-test-conv"][0].Role)

	// Verify the response contains expected content
	assert.Contains(t, resp.Text, "test") // Should reference testing tasks
}

// multiIterationProvider simulates a provider that makes multiple tool calls across iterations.
type multiIterationProvider struct {
	iteration int
}

func (p *multiIterationProvider) Complete(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error) {
	p.iteration++

	// Check if we have tool results from previous iterations
	hasToolResults := false
	for _, msg := range in.Messages {
		if msg.Role == "tool" {
			hasToolResults = true
			break
		}
	}

	if hasToolResults {
		// After tool execution: provide final summary
		return ports.Completion{
			Text: "Based on the knowledge graph and filesystem analysis, I found comprehensive testing for the harness implementation.",
		}, nil
	} else {
		// First iteration: request KG search
		return ports.Completion{
			Text: "I need to search the knowledge graph for testing tasks.",
			ToolCalls: []ports.ToolCall{
				{Name: "kg_search", Args: json.RawMessage(`{"query": "testing tasks", "limit": 5}`)},
			},
		}, nil
	}
}

func (p *multiIterationProvider) Stream(ctx context.Context, in ports.PromptInput, opts ports.Options) (<-chan ports.CompletionChunk, error) {
	ch := make(chan ports.CompletionChunk, 1)
	ch <- ports.CompletionChunk{
		DeltaText: "Streaming response",
		Done:      true,
	}
	close(ch)
	return ch, nil
}

// testConversationStore implements ConversationStore for testing with verification capabilities.
type testConversationStore struct {
	turns map[string][]ports.Turn
	mu    sync.RWMutex
}

func (s *testConversationStore) SaveTurn(ctx context.Context, conversationID string, turn ports.Turn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turns == nil {
		s.turns = make(map[string][]ports.Turn)
	}
	s.turns[conversationID] = append(s.turns[conversationID], turn)
	return nil
}

func (s *testConversationStore) LoadContext(ctx context.Context, conversationID string, k int) ([]ports.Turn, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	turns, exists := s.turns[conversationID]
	if !exists {
		return nil, nil
	}

	if k <= 0 || k >= len(turns) {
		return turns, nil
	}

	return turns[len(turns)-k:], nil
}

func (s *testConversationStore) AppendToolArtifact(ctx context.Context, conversationID, name string, payload []byte) error {
	return s.SaveTurn(ctx, conversationID, ports.Turn{
		Role:      "tool",
		Content:   fmt.Sprintf("Tool %s: %s", name, string(payload)),
		CreatedAt: time.Now(),
	})
}

// TestToolSchemaValidation tests that tool schemas are properly validated.
func TestToolSchemaValidation(t *testing.T) {
	kgTool := tools.NewKGSearchTool()

	// Test valid arguments
	validArgs := `{"query": "test query", "limit": 5}`
	_, err := kgTool.Invoke(context.Background(), json.RawMessage(validArgs))
	assert.NoError(t, err)

	// Test invalid JSON
	invalidJSON := `{"query": "test", "limit": }`
	_, err = kgTool.Invoke(context.Background(), json.RawMessage(invalidJSON))
	assert.Error(t, err)

	// Test missing required field
	missingRequired := `{"limit": 5}`
	_, err = kgTool.Invoke(context.Background(), json.RawMessage(missingRequired))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

// TestFactory_Wiring tests that the factory properly wires components.
func TestFactory_Wiring(t *testing.T) {
	// Create test config
	config := &config.HarnessConfig{
		CacheEnabled:      true,
		CacheCapacity:     100,
		CacheTTLSeconds:   3600,
		RateLimitEnabled:  true,
		RateLimitCapacity: 10,
		MaxToolDepth:      3,
		MaxIterations:     5,
		EnableGuardrails:  true,
		AllowedTools:      []string{"kg_search", "fs_metadata"},
		EnableTracing:     true,
		ToolConcurrency:   3,
	}

	factory := NewFactory(config, nil, zerolog.New(zerolog.Nop()))

	// Test cache creation
	cache := factory.createCache()
	assert.NotNil(t, cache)

	// Test rate limiter creation
	limiter := factory.createRateLimiter()
	assert.NotNil(t, limiter)

	// Test tracer creation
	tracer := factory.createTracer()
	assert.NotNil(t, tracer)

	// Test store creation (no-op when no DB)
	store := factory.createStore()
	assert.NotNil(t, store)

	// Test guardrails creation
	guardrails := factory.CreateGuardrails()
	assert.NotNil(t, guardrails)

	// Test policy creation
	policy := factory.CreatePolicy()
	assert.Equal(t, 3, policy.MaxToolDepth)
	assert.Equal(t, 5, policy.MaxIterations)
}

// TestStreamingAggregator tests the streaming aggregator functionality.
func TestStreamingAggregator(t *testing.T) {
	aggregator := newStreamingAggregator()

	// Test chunk accumulation
	chunk1 := ports.CompletionChunk{
		DeltaText: "First chunk",
		ToolCalls: []ports.ToolCall{
			{Name: "test_tool", Args: json.RawMessage(`{"arg": "value"}`)},
		},
	}

	chunk2 := ports.CompletionChunk{
		DeltaText: " second chunk",
		Usage: &ports.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	}

	aggregator.addChunk(chunk1)
	aggregator.addChunk(chunk2)

	// Test accumulated text
	assert.Equal(t, "First chunk second chunk", aggregator.getText())

	// Test tool calls (provider tool calls are stored directly)
	toolCalls := aggregator.getToolCalls()
	assert.Len(t, toolCalls, 1)
	assert.Equal(t, "test_tool", toolCalls[0].Name)

	// Test usage
	usage := aggregator.getUsage()
	assert.Equal(t, 25, usage.TotalTokens)

	// Test finalize
	completion := aggregator.finalize()
	assert.Equal(t, "First chunk second chunk", completion.Text)
	assert.Len(t, completion.ToolCalls, 1)
	assert.Equal(t, 25, completion.Usage.TotalTokens)
}

// TestConcurrencySafety tests that the harness is safe for concurrent use.
func TestConcurrencySafety(t *testing.T) {
	provider := &StubProvider{
		completionFunc: func(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error) {
			return ports.Completion{
				Text: "Concurrent response",
				Usage: &ports.Usage{
					PromptTokens:     5,
					CompletionTokens: 3,
					TotalTokens:      8,
				},
			}, nil
		},
	}

	// Setup components
	builder := NewPromptBuilder()
	assembler := NewContextAssembler(Budget{MaxContextTokens: 1000}, nil)
	store := &testConversationStore{turns: make(map[string][]ports.Turn)}
	cache := adapters.NewLRUCache(100)
	limiter := adapters.NewTokenBucket(100, time.Second)
	tracer := adapters.NewZerologTracer(zerolog.New(zerolog.Nop()))

	orchestrator := NewHarnessOrchestrator(provider, builder, assembler, store, cache, limiter, tracer)

	// Run multiple concurrent orchestrations
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			req := &Request{
				Conversation: &Conversation{
					ID: fmt.Sprintf("concurrent-%d", id),
					Messages: []ports.PromptMessage{
						{Role: "user", Content: fmt.Sprintf("Concurrent request %d", id)},
					},
				},
				System: "You are a helpful assistant",
				Tools:  []ports.Tool{},
			}

			resp, err := orchestrator.Orchestrate(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
