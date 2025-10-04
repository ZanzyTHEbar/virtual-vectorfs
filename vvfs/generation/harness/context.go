package harness

import (
	"context"
	"sort"
	"strings"
)

// ContextSource abstracts retrieval of candidate context snippets (e.g., RAG, KG, filesystem).
type ContextSource interface {
	Search(ctx context.Context, query string, limit int) ([]Snippet, error)
}

// Snippet is a retrievable chunk with a score and token estimate.
type Snippet struct {
	Text       string
	Score      float32 // higher is better
	TokenCount int
	Source     string // optional provenance
}

// Budget specifies maximum tokens allocated to context packing.
type Budget struct {
	MaxContextTokens int // hard cap for context snippets
	MaxSnippets      int // safety bound on number of chunks
}

// ContextAssembler selects and packs context snippets within a token budget.
type ContextAssembler struct {
	defaultBudget Budget
	// TokenEstimator should be a fast heuristic; we avoid binding to a specific tokenizer here.
	TokenEstimator func(s string) int
}

func NewContextAssembler(b Budget, est func(s string) int) *ContextAssembler {
	if est == nil {
		est = func(s string) int { // rough heuristic: ~4 chars per token
			l := len(s)
			if l == 0 {
				return 0
			}
			return (l + 3) / 4
		}
	}
	return &ContextAssembler{defaultBudget: b, TokenEstimator: est}
}

// Pack sorts snippets by score desc and packs up to budget, normalizing text.
func (a *ContextAssembler) Pack(snippets []Snippet, b *Budget) []string {
	if b == nil {
		b = &a.defaultBudget
	}
	if len(snippets) == 0 || b.MaxContextTokens <= 0 || b.MaxSnippets <= 0 {
		return nil
	}

	// Sort by score desc
	sort.Slice(snippets, func(i, j int) bool { return snippets[i].Score > snippets[j].Score })

	remaining := b.MaxContextTokens
	count := 0
	packed := make([]string, 0, min(len(snippets), b.MaxSnippets))

	norm := func(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")) }

	for _, sn := range snippets {
		if count >= b.MaxSnippets {
			break
		}
		if sn.TokenCount <= 0 {
			sn.TokenCount = a.TokenEstimator(sn.Text)
		}
		if sn.TokenCount > remaining {
			continue
		}
		packed = append(packed, norm(sn.Text))
		remaining -= sn.TokenCount
		count++
		if remaining <= 0 {
			break
		}
	}

	return packed
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
