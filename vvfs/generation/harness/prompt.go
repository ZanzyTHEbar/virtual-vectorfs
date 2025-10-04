package harness

import (
	"strings"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

// PromptBuilder assembles model-ready inputs from system text, messages, and tools.
type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder { return &PromptBuilder{} }

// Build flattens system + chat messages into a Provider PromptInput.
func (b *PromptBuilder) Build(system string, messages []ports.PromptMessage, contextSnippets []string, toolSpecs []ports.ToolSpec, meta map[string]string) ports.PromptInput {
	// Normalize newlines and trim whitespace to reduce prompt diffs for caching
	norm := func(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")) }

	for i := range messages {
		messages[i].Content = norm(messages[i].Content)
	}
	for i := range contextSnippets {
		contextSnippets[i] = norm(contextSnippets[i])
	}

	return ports.PromptInput{
		System:   norm(system),
		Messages: messages,
		Context:  contextSnippets,
		Tools:    toolSpecs,
		Meta:     meta,
	}
}
