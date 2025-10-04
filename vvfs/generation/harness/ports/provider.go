package harnessports

import (
	"context"
)

// PromptMessage represents a single chat message used to build prompts.
type PromptMessage struct {
	Role    string // "system", "developer", "user", "assistant"
	Content string
}

// PromptInput aggregates everything the provider needs to produce a completion.
type PromptInput struct {
	System   string            // high-level system/developer instructions
	Messages []PromptMessage   // ordered chat history (already windowed)
	Context  []string          // retrieved RAG/context snippets
	Tools    []ToolSpec        // tool declarations available to the model
	Meta     map[string]string // lightweight metadata for tracing/caching keys
}

// Options controls sampling, limits, determinism, and tool preferences.
type Options struct {
	MaxNewTokens int
	Temperature  float32
	TopP         float32
	MinP         float32
	Seed         int
	Stop         []string
	// ToolChoice: "auto" | "none" | specific tool name (if the provider supports it)
	ToolChoice string
	// TimeoutMs applies to the provider call only (not overall harness deadline)
	TimeoutMs int
}

// Usage captures token accounting for cost/telemetry.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Completion is the provider's non-streaming response.
type Completion struct {
	Text      string
	ToolCalls []ToolCall
	Raw       any    // raw provider payload for debugging/telemetry
	Usage     *Usage // optional usage information
}

// CompletionChunk is the provider's streaming delta.
type CompletionChunk struct {
	DeltaText string
	ToolCalls []ToolCall
	Done      bool
	Usage     *Usage // on final chunk when available
}

// Provider is the abstraction for all LLM backends (inference hidden behind this port).
type Provider interface {
	Complete(ctx context.Context, in PromptInput, opts Options) (Completion, error)
	Stream(ctx context.Context, in PromptInput, opts Options) (<-chan CompletionChunk, error)
}
