package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

// Conversation represents the current state of a conversation.
type Conversation struct {
	ID       string
	Messages []ports.PromptMessage
}

// Request configures the orchestration run.
type Request struct {
	Conversation *Conversation
	System       string
	Context      []string
	Tools        []ports.Tool
	Policy       *Policy
}

// Policy controls orchestration behavior.
type Policy struct {
	MaxToolDepth      int           // max recursive tool calls
	MaxIterations     int           // safeguard against infinite loops
	ToolTimeout       time.Duration // per-tool timeout
	RequireJSONOutput bool          // force JSON mode
	Deterministic     bool          // seed for reproducible results
	RetryCount        int           // provider call retries
	RetryBackoff      time.Duration // base delay between retries
}

// DefaultPolicy returns sensible defaults.
func DefaultPolicy() *Policy {
	return &Policy{
		MaxToolDepth:      3,
		MaxIterations:     10,
		ToolTimeout:       30 * time.Second,
		RequireJSONOutput: false,
		Deterministic:     false,
		RetryCount:        2,
		RetryBackoff:      100 * time.Millisecond,
	}
}

// Response is the final output of the orchestrator.
type Response struct {
	Text      string
	ToolCalls []ports.ToolCall
	Usage     *ports.Usage
}

// HarnessOrchestrator coordinates the full tool-calling loop.
type HarnessOrchestrator struct {
	provider  ports.Provider
	builder   *PromptBuilder
	assembler *ContextAssembler
	store     ports.ConversationStore
	cache     ports.Cache
	limiter   ports.RateLimiter
	tracer    ports.Tracer
}

// NewHarnessOrchestrator creates a new orchestrator with dependencies.
func NewHarnessOrchestrator(
	provider ports.Provider,
	builder *PromptBuilder,
	assembler *ContextAssembler,
	store ports.ConversationStore,
	cache ports.Cache,
	limiter ports.RateLimiter,
	tracer ports.Tracer,
) *HarnessOrchestrator {
	return &HarnessOrchestrator{
		provider:  provider,
		builder:   builder,
		assembler: assembler,
		store:     store,
		cache:     cache,
		limiter:   limiter,
		tracer:    tracer,
	}
}

// Orchestrate runs the full tool-calling loop to completion.
func (o *HarnessOrchestrator) Orchestrate(ctx context.Context, req *Request) (*Response, error) {
	if req.Policy == nil {
		req.Policy = DefaultPolicy()
	}

	// Acquire rate limit permit
	release, err := o.limiter.Acquire(ctx, "orchestrate")
	if err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	defer release()

	// Start tracing span
	ctx, finish := o.tracer.StartSpan(ctx, "orchestrate", map[string]any{
		"conversation_id": req.Conversation.ID,
		"tool_count":      len(req.Tools),
	})
	defer finish(nil)

	// Try cache first
	cacheKey := o.buildCacheKey(req)
	if cached, ok := o.cache.Get(ctx, cacheKey); ok {
		o.tracer.Event(ctx, "cache_hit", map[string]any{"key": cacheKey})
		return o.parseCachedResponse(cached)
	}

	// Build initial prompt
	toolSpecs := o.buildToolSpecs(req.Tools)
	prompt := o.builder.Build(req.System, req.Conversation.Messages, req.Context, toolSpecs, map[string]string{
		"conversation_id": req.Conversation.ID,
		"tool_count":      fmt.Sprintf("%d", len(req.Tools)),
	})

	// Run orchestration loop
	result, err := o.runLoop(ctx, req, prompt)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if resultBytes, err := json.Marshal(result); err == nil {
		o.cache.Set(ctx, cacheKey, resultBytes, 3600) // 1 hour TTL
	}

	// Persist final turn
	if err := o.store.SaveTurn(ctx, req.Conversation.ID, ports.Turn{
		Role:      "assistant",
		Content:   result.Text,
		CreatedAt: time.Now(),
	}); err != nil {
		// Log but don't fail
		o.tracer.Event(ctx, "store_error", map[string]any{"error": err.Error()})
	}

	return result, nil
}

// StreamOrchestrate provides streaming orchestration with early tool-call emission.
func (o *HarnessOrchestrator) StreamOrchestrate(ctx context.Context, req *Request) (<-chan *Response, <-chan error) {
	respCh := make(chan *Response, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(respCh)
		defer close(errCh)

		// Run orchestration loop with streaming aggregator
		aggregator := newStreamingAggregator()

		currentPrompt := o.buildInitialPrompt(req)
		iteration := 0
		depth := 0

		for {
			iteration++
			if iteration > req.Policy.MaxIterations {
				errCh <- fmt.Errorf("max iterations exceeded: %d", req.Policy.MaxIterations)
				return
			}

			// Build provider options
			opts := ports.Options{
				MaxNewTokens: 1024,
				Temperature:  0.7,
				TopP:         0.9,
				Seed:         0,
			}
			if req.Policy.Deterministic && iteration == 1 {
				opts.Seed = 42
			}

			// Call provider with streaming
			streamCh, err := o.provider.Stream(ctx, currentPrompt, opts)
			if err != nil {
				errCh <- fmt.Errorf("provider stream failed: %w", err)
				return
			}

			// Process stream chunks
			o.processStream(ctx, streamCh, aggregator)

			// Check for tool calls in aggregated content
			toolCalls := aggregator.getToolCalls()
			if len(toolCalls) > 0 {
				// Emit early tool calls for immediate execution
				respCh <- &Response{
					Text:      aggregator.getText(),
					ToolCalls: toolCalls,
					Usage:     aggregator.getUsage(),
				}

				// Validate tool depth only if we're going to execute tools
				if depth >= req.Policy.MaxToolDepth {
					errCh <- fmt.Errorf("max tool depth exceeded: %d", req.Policy.MaxToolDepth)
					return
				}
				depth++

				// Execute tools
				toolResults, err := o.executeTools(ctx, req.Tools, toolCalls)
				if err != nil {
					errCh <- fmt.Errorf("tool execution failed: %w", err)
					return
				}

				// Append to conversation and continue loop
				req.Conversation.Messages = append(req.Conversation.Messages,
					ports.PromptMessage{Role: "assistant", Content: aggregator.getText()},
				)
				for _, result := range toolResults {
					req.Conversation.Messages = append(req.Conversation.Messages,
						ports.PromptMessage{Role: "tool", Content: result},
					)
				}

				// Rebuild prompt for next iteration
				currentPrompt = o.builder.Build(req.System, req.Conversation.Messages, req.Context, o.buildToolSpecs(req.Tools), nil)
				continue
			}

			// No tool calls - final response
			respCh <- &Response{
				Text:      aggregator.getText(),
				ToolCalls: nil,
				Usage:     aggregator.getUsage(),
			}
			break
		}
	}()

	return respCh, errCh
}

// processStream aggregates streaming chunks into a completion.
func (o *HarnessOrchestrator) processStream(ctx context.Context, streamCh <-chan ports.CompletionChunk, aggregator *streamingAggregator) ports.Completion {
	for chunk := range streamCh {
		aggregator.addChunk(chunk)

		// Check if we have early tool calls to emit
		if toolCalls := aggregator.getEarlyToolCalls(); len(toolCalls) > 0 {
			// This would be handled by the streaming aggregator to emit early
			// For now, we'll continue accumulating until the end
		}
	}

	return aggregator.finalize()
}

// buildInitialPrompt builds the initial prompt for orchestration.
func (o *HarnessOrchestrator) buildInitialPrompt(req *Request) ports.PromptInput {
	toolSpecs := o.buildToolSpecs(req.Tools)
	return o.builder.Build(req.System, req.Conversation.Messages, req.Context, toolSpecs, map[string]string{
		"conversation_id": req.Conversation.ID,
		"tool_count":      fmt.Sprintf("%d", len(req.Tools)),
	})
}

// streamingAggregator accumulates streaming chunks and detects early tool calls.
type streamingAggregator struct {
	text          strings.Builder
	toolCalls     []ports.ToolCall
	usage         *ports.Usage
	parser        *OutputParser
	earlyCalls    []ports.ToolCall
	partialBuffer strings.Builder // Buffer for incomplete JSON that might span chunks
}

func newStreamingAggregator() *streamingAggregator {
	return &streamingAggregator{
		parser: NewOutputParser(),
	}
}

func (a *streamingAggregator) addChunk(chunk ports.CompletionChunk) {
	// Accumulate text
	a.text.WriteString(chunk.DeltaText)

	// Also accumulate in partial buffer for JSON parsing
	a.partialBuffer.WriteString(chunk.DeltaText)

	// Use provider tool calls if available, otherwise parse from text
	if len(chunk.ToolCalls) > 0 {
		// Accumulate tool calls from provider
		a.toolCalls = append(a.toolCalls, chunk.ToolCalls...)
		// Store the latest batch for early emission
		if len(a.earlyCalls) == 0 {
			a.earlyCalls = chunk.ToolCalls
		}
	} else {
		// Try to parse tool calls from both buffers
		if calls := a.parser.ParseToolCalls(a.text.String()); len(calls) > 0 {
			a.toolCalls = append(a.toolCalls, calls...)
			if len(a.earlyCalls) == 0 {
				a.earlyCalls = calls
			}
		} else if calls := a.parser.ParseToolCalls(a.partialBuffer.String()); len(calls) > 0 {
			a.toolCalls = append(a.toolCalls, calls...)
			if len(a.earlyCalls) == 0 {
				a.earlyCalls = calls
			}
		}
	}

	// Update usage if provided (take the latest usage info)
	if chunk.Usage != nil {
		a.usage = chunk.Usage
	}
}

func (a *streamingAggregator) getText() string {
	return a.text.String()
}

func (a *streamingAggregator) getToolCalls() []ports.ToolCall {
	return a.toolCalls
}

func (a *streamingAggregator) getUsage() *ports.Usage {
	return a.usage
}

func (a *streamingAggregator) getEarlyToolCalls() []ports.ToolCall {
	calls := a.earlyCalls
	a.earlyCalls = nil // Clear after getting
	return calls
}

func (a *streamingAggregator) finalize() ports.Completion {
	// Final parse of accumulated text for any missed tool calls
	if len(a.toolCalls) == 0 {
		// Try parsing from main text buffer first
		if calls := a.parser.ParseToolCalls(a.text.String()); len(calls) > 0 {
			a.toolCalls = append(a.toolCalls, calls...)
		}

		// Also try parsing from partial buffer which might contain complete JSON
		if calls := a.parser.ParseToolCalls(a.partialBuffer.String()); len(calls) > 0 {
			a.toolCalls = append(a.toolCalls, calls...)
		}
	}

	return ports.Completion{
		Text:      a.text.String(),
		ToolCalls: a.toolCalls,
		Usage:     a.usage,
	}
}

// runLoop executes the tool-calling loop until completion.
func (o *HarnessOrchestrator) runLoop(ctx context.Context, req *Request, prompt ports.PromptInput) (*Response, error) {
	currentPrompt := prompt
	iteration := 0
	depth := 0

	for {
		iteration++
		if iteration > req.Policy.MaxIterations {
			return nil, fmt.Errorf("max iterations exceeded: %d", req.Policy.MaxIterations)
		}

		// Build provider options
		opts := ports.Options{
			MaxNewTokens: 1024,
			Temperature:  0.7,
			TopP:         0.9,
			Seed:         0, // will be set if deterministic
		}
		if req.Policy.Deterministic && iteration == 1 {
			opts.Seed = 42
		}

		// Call provider
		ctx, spanFinish := o.tracer.StartSpan(ctx, "provider_call", map[string]any{
			"iteration": iteration,
			"depth":     depth,
		})
		completion, err := o.provider.Complete(ctx, currentPrompt, opts)
		spanFinish(err)

		if err != nil {
			return nil, fmt.Errorf("provider call failed: %w", err)
		}

		// Merge tool calls from provider and parsed text
		providerToolCalls := completion.ToolCalls
		parsedToolCalls := o.parseToolCalls(completion.Text)

		// Use provider tool calls if available, otherwise fall back to parsed
		toolCalls := providerToolCalls
		if len(toolCalls) == 0 {
			toolCalls = parsedToolCalls
		}

		// Check stop conditions
		if len(toolCalls) == 0 {
			// No more tool calls - final response
			return &Response{
				Text:      completion.Text,
				ToolCalls: nil,
				Usage:     completion.Usage,
			}, nil
		}

		// Validate tool depth only if we're going to execute tools
		if depth >= req.Policy.MaxToolDepth {
			return nil, fmt.Errorf("max tool depth exceeded: %d", req.Policy.MaxToolDepth)
		}
		depth++

		// Execute tools and append results
		toolResults, err := o.executeTools(ctx, req.Tools, toolCalls)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		// Append tool results to conversation
		req.Conversation.Messages = append(req.Conversation.Messages,
			ports.PromptMessage{Role: "assistant", Content: completion.Text},
		)
		for _, result := range toolResults {
			req.Conversation.Messages = append(req.Conversation.Messages,
				ports.PromptMessage{Role: "tool", Content: result},
			)
		}

		// Rebuild prompt for next iteration
		currentPrompt = o.builder.Build(req.System, req.Conversation.Messages, req.Context, o.buildToolSpecs(req.Tools), nil)
	}
}

// executeTools runs all tool calls in parallel with timeout.
func (o *HarnessOrchestrator) executeTools(ctx context.Context, tools []ports.Tool, calls []ports.ToolCall) ([]string, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	// Build tool map for lookup
	toolMap := make(map[string]ports.Tool)
	for _, tool := range tools {
		toolMap[tool.Name()] = tool
	}

	type result struct {
		content string
		err     error
	}

	results := make([]result, len(calls))
	sem := make(chan struct{}, 5) // limit concurrency

	// Start all goroutines
	for i, call := range calls {
		go func(idx int, tc ports.ToolCall) {
			sem <- struct{}{} // acquire semaphore

			tool, exists := toolMap[tc.Name]
			if !exists {
				results[idx] = result{content: "", err: fmt.Errorf("unknown tool: %s", tc.Name)}
				<-sem // release semaphore
				return
			}

			toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			output, err := tool.Invoke(toolCtx, tc.Args)
			if err != nil {
				results[idx] = result{content: "", err: fmt.Errorf("tool %s failed: %w", tc.Name, err)}
				<-sem // release semaphore
				return
			}

			// Convert output to string
			var content string
			if str, ok := output.(string); ok {
				content = str
			} else {
				jsonBytes, err := json.Marshal(output)
				if err != nil {
					content = fmt.Sprintf("Error marshaling tool output: %v", err)
					results[idx] = result{content: content, err: fmt.Errorf("tool %s output marshaling failed: %w", tc.Name, err)}
					<-sem // release semaphore
					return
				} else {
					content = string(jsonBytes)
				}
			}

			results[idx] = result{content: content, err: nil}
			<-sem // release semaphore
		}(i, call)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(calls); i++ {
		sem <- struct{}{} // block until all goroutines release their semaphores
	}

	// Collect results - handle partial failures gracefully
	var outputs []string
	var errors []error

	for _, res := range results {
		if res.err != nil {
			errors = append(errors, res.err)
		} else {
			outputs = append(outputs, res.content)
		}
	}

	// If any tools failed, return the first error but include successful results in context
	if len(errors) > 0 {
		return outputs, fmt.Errorf("some tools failed (returned %d successful results): %w", len(outputs), errors[0])
	}

	return outputs, nil
}

// buildToolSpecs converts tools to provider-expected specs.
func (o *HarnessOrchestrator) buildToolSpecs(tools []ports.Tool) []ports.ToolSpec {
	specs := make([]ports.ToolSpec, len(tools))
	for i, tool := range tools {
		specs[i] = ports.ToolSpec{
			Name:        tool.Name(),
			Description: "", // would need tool to provide this
			JSONSchema:  tool.Schema(),
		}
	}
	return specs
}

// buildCacheKey creates a deterministic key for caching.
func (o *HarnessOrchestrator) buildCacheKey(req *Request) string {
	// Create a more robust cache key that includes all relevant components
	// Use a simple hash-like approach to avoid extremely long keys
	key := fmt.Sprintf("conv:%s|sys:%s|ctx:%s|tools:%d",
		req.Conversation.ID,
		o.hashString(req.System),
		o.hashString(strings.Join(req.Context, "|")),
		len(req.Tools))

	if req.Policy != nil {
		key += fmt.Sprintf("|policy:%d:%d", req.Policy.MaxToolDepth, req.Policy.MaxIterations)
	}

	return key
}

// hashString creates a simple hash of a string for cache key generation.
func (o *HarnessOrchestrator) hashString(s string) string {
	// Simple djb2 hash for deterministic but shorter keys
	hash := uint32(5381)
	for _, r := range s {
		hash = ((hash << 5) + hash) + uint32(r)
	}
	return fmt.Sprintf("%x", hash)
}

// parseCachedResponse reconstructs a Response from cached bytes.
func (o *HarnessOrchestrator) parseCachedResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse cached response: %w", err)
	}
	return &resp, nil
}
