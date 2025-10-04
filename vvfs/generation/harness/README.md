# LLM Harness — Production-Ready Orchestration System

## Overview

The LLM Harness is a production-grade orchestration system for managing complex LLM interactions with tool-calling, streaming, caching, rate limiting, and safety guardrails. It abstracts LLM inference behind clean interfaces, enabling flexibility and testability while providing enterprise-ready reliability.

## Architecture

The harness follows **hexagonal architecture** principles with clear separation between:

- **Ports (Interfaces)**: Define contracts for providers, tools, storage, caching, etc.
- **Core Logic**: Orchestration, prompt building, context assembly, output parsing
- **Adapters**: Concrete implementations (LRU cache, token bucket limiter, LibSQL store, etc.)
- **Factory**: Configuration-driven component instantiation

```
┌─────────────────────────────────────────────────────────┐
│                   Client Application                     │
└───────────────────────┬─────────────────────────────────┘
                        │
                        ▼
            ┌───────────────────────┐
            │  HarnessOrchestrator  │ ◄── Core Logic
            └───────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
   ┌────────┐    ┌──────────┐    ┌─────────┐
   │Provider│    │  Tools   │    │ Memory  │
   │ (Port) │    │ (Port)   │    │ (Port)  │
   └────────┘    └──────────┘    └─────────┘
        │               │               │
        ▼               ▼               ▼
   ┌────────┐    ┌──────────┐    ┌─────────┐
   │OpenAI  │    │KGSearch  │    │ LibSQL  │
   │Claude  │    │FSMetadata│    │ Store   │
   │Ollama  │    │...       │    │         │
   └────────┘    └──────────┘    └─────────┘
```

## Key Features

### 🚀 Core Capabilities

- **Multi-Turn Tool Calling**: Supports iterative LLM-tool loops with configurable depth/iteration limits
- **Streaming Support**: Real-time response processing with early tool-call detection
- **Context Management**: Token-aware context assembly with budget enforcement
- **Prompt Engineering**: Structured prompt building with system messages, context, and tool specs
- **Output Parsing**: Intelligent extraction of tool calls and JSON from LLM responses

### 🛡️ Safety & Reliability

- **Guardrails**: Input validation, output filtering, JSON schema compliance
- **Rate Limiting**: Token bucket algorithm prevents API overuse
- **Caching**: LRU cache with TTL for response reuse
- **Policy Enforcement**: Configurable limits on tool depth, iterations, output size
- **Error Handling**: Graceful degradation and comprehensive error propagation

### 📊 Observability

- **Tracing**: OpenTelemetry-compatible structured logging
- **Conversation Persistence**: Store and retrieve multi-turn conversations
- **Metrics**: Performance tracking via tracer events
- **Explainability**: Return component scores and execution metadata

### ⚡ Performance

- **Concurrent Tool Execution**: Parallel tool invocation with semaphore control
- **Efficient Caching**: Hash-based cache keys for deterministic lookup
- **Zero-Allocation Paths**: Optimized hot paths (PromptBuilder: 41ns/op)
- **Memory Safety**: No leaks, proper resource cleanup

## Quick Start

### Installation

```go
import (
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/tools"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)
```

### Basic Usage

```go
// 1. Load configuration
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}

// 2. Create factory
factory := harness.NewFactory(&cfg.Harness, db, logger)

// 3. Create orchestrator
orchestrator, err := factory.CreateOrchestrator()
if err != nil {
    log.Fatal(err)
}

// 4. Add tools
kgTool := tools.NewKGSearchTool()
fsTool := tools.NewFSMetadataTool("/base/path")

// 5. Execute request
req := &harness.Request{
    ConversationID: "user-123",
    System:         "You are a helpful assistant.",
    Messages: []ports.PromptMessage{
        {Role: "user", Content: "Find information about Go concurrency"},
    },
    Tools: []ports.Tool{kgTool, fsTool},
}

resp, err := orchestrator.Orchestrate(ctx, req)
if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.FinalText)
```

### Streaming Usage

```go
respCh, errCh := orchestrator.StreamOrchestrate(ctx, req)

for resp := range respCh {
    // Process incremental response
    fmt.Print(resp.FinalText)
    
    // Check for early tool calls
    if len(resp.ToolCalls) > 0 {
        fmt.Println("\nTools triggered:", resp.ToolCalls)
    }
}

if err := <-errCh; err != nil {
    log.Fatal(err)
}
```

## Configuration

The harness is configured via `HarnessConfig` in your `config.yaml`:

```yaml
harness:
  # Caching
  cache_enabled: true
  cache_capacity: 1000
  cache_ttl_seconds: 3600
  
  # Rate Limiting
  rate_limit_enabled: true
  rate_limit_capacity: 10
  rate_limit_refill_rate: 1s
  
  # Policy
  max_tool_depth: 3
  max_iterations: 10
  max_output_size: 10000
  
  # Guardrails
  enable_guardrails: true
  blocked_words: ["password", "secret", "key", "token"]
  allowed_tools: []  # Empty means allow all
  
  # Observability
  enable_tracing: true
  tool_concurrency: 5
```

## Component Reference

### Ports (Interfaces)

#### Provider

Abstracts LLM inference:

```go
type Provider interface {
    Complete(ctx context.Context, in PromptInput, opts Options) (Completion, error)
    Stream(ctx context.Context, in PromptInput, opts Options) (<-chan CompletionChunk, error)
}
```

#### Tool

External function invocation:

```go
type Tool interface {
    Name() string
    Schema() []byte  // JSON Schema
    Invoke(ctx context.Context, args json.RawMessage) (any, error)
}
```

#### ConversationStore

Persist conversation history:

```go
type ConversationStore interface {
    SaveTurn(ctx context.Context, convID string, turn Turn) error
    LoadContext(ctx context.Context, convID string, k int) ([]Turn, error)
}
```

#### Cache

Response caching:

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool)
    Set(ctx context.Context, key string, value []byte, ttlSeconds int) error
    Delete(ctx context.Context, key string) error
}
```

#### RateLimiter

API rate control:

```go
type RateLimiter interface {
    Acquire(ctx context.Context, key string) (release func(), err error)
}
```

#### Tracer

Observability:

```go
type Tracer interface {
    StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, func(err error))
    Event(ctx context.Context, name string, attrs map[string]any)
}
```

### Core Components

#### HarnessOrchestrator

Main orchestration engine:

- Manages tool-calling loops
- Handles streaming and non-streaming execution
- Enforces policies and guardrails
- Coordinates caching and rate limiting

#### PromptBuilder

Constructs LLM prompts:

- Assembles system messages, context, and tools
- Formats prompts for provider consumption
- Supports metadata injection

#### ContextAssembler

Token-aware context management:

- Budget-based snippet selection
- Priority-based ranking
- Token estimation

#### OutputParser

Extracts structured data from LLM responses:

- Tool call parsing (multiple formats)
- JSON extraction and validation
- JSON schema compliance checking

#### Guardrails

Safety and validation:

- Tool allowlisting
- Output filtering (blocked words)
- JSON schema validation (`gojsonschema`)
- Input sanitization

### Adapters

#### LRU Cache (`adapters/cache_lru.go`)

In-memory cache with TTL support.

#### Token Bucket Rate Limiter (`adapters/ratelimit_tb.go`)

Per-key rate limiting with configurable capacity and refill rate.

#### Zerolog Tracer (`adapters/trace_zerolog.go`)

Structured logging with span tracking.

#### LibSQL Conversation Store (`adapters/store_libsql.go`)

SQLite-backed conversation persistence.

## Tool Development

### Creating a Custom Tool

```go
package mytools

import (
    "context"
    "encoding/json"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

const MyToolSchema = `{
  "type": "object",
  "properties": {
    "query": {"type": "string", "description": "Search query"},
    "limit": {"type": "integer", "description": "Max results", "default": 10}
  },
  "required": ["query"]
}`

type MySearchTool struct {}

func NewMySearchTool() *MySearchTool {
    return &MySearchTool{}
}

func (t *MySearchTool) Name() string {
    return "my_search"
}

func (t *MySearchTool) Schema() []byte {
    return []byte(MyToolSchema)
}

func (t *MySearchTool) Invoke(ctx context.Context, args json.RawMessage) (any, error) {
    var params struct {
        Query string `json:"query"`
        Limit int    `json:"limit"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, err
    }
    
    // Implement search logic
    results := performSearch(params.Query, params.Limit)
    
    return results, nil
}
```

### Tool Best Practices

1. **Input Validation**: Always validate and sanitize inputs
2. **Error Handling**: Return meaningful errors with context
3. **Timeouts**: Respect context cancellation
4. **Security**: Prevent path traversal, injection attacks
5. **Schema**: Provide detailed JSON schemas for LLM guidance
6. **Idempotency**: Design tools to be safely retryable

## Testing

### Running Tests

```bash
# Run all tests
go test ./vvfs/generation/harness/... -v

# Run benchmarks
go test ./vvfs/generation/harness/... -bench=. -benchmem

# Check for race conditions
go test ./vvfs/generation/harness/... -race

# Vet for issues
go vet ./vvfs/generation/harness/...
```

### Test Coverage

The harness includes comprehensive test coverage:

- **Unit Tests**: All core components (13 tests)
- **Integration Tests**: End-to-end multi-turn scenarios
- **Benchmarks**: Performance validation
- **Concurrency Tests**: Thread-safety verification
- **Property Tests**: Parser robustness

**Current Metrics**:

- ✅ 100% test pass rate
- ✅ PromptBuilder: 41ns/op (zero allocations)
- ✅ ContextAssembler: 1.8ns/op (zero allocations)
- ✅ LRU Cache: 325ns/op (4 allocations)
- ✅ Zero race conditions detected

## Backward Compatibility

The harness provides a bridge for existing `Generator` interface users:

```go
import "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation"

// Use existing Generator interface
gen := generation.NewHarnessGenerator(orchestrator, "conversation-id")

resp, err := gen.Generate(ctx, &generation.GenerationRequest{
    Messages: []generation.Message{
        {Role: "user", Content: "Hello"},
    },
})
```

## Performance Tuning

### Cache Strategy

- **Capacity**: Set based on memory budget (default: 1000 entries)
- **TTL**: Balance freshness vs hit rate (default: 1 hour)
- **Key Design**: Deterministic hash-based keys prevent collisions

### Rate Limiting

- **Capacity**: Max tokens per time window (default: 10)
- **Refill Rate**: How fast tokens replenish (default: 1s)
- **Per-Key**: Separate limits for different users/conversations

### Tool Concurrency

- **Tool Concurrency**: Max parallel tool executions (default: 5)
- **Timeouts**: Individual tool timeout (default: 30s)
- **Semaphore**: Prevents resource exhaustion

### Context Budget

- **Max Tokens**: Set based on model limits
- **Snippet Priority**: Rank by relevance score
- **Overflow**: Older context truncated first

## Production Considerations

### Error Handling

The harness propagates errors with context:

- Tool failures don't break orchestration
- Partial results returned when possible
- Comprehensive error wrapping for debugging

### Security

- **Input Sanitization**: All tool inputs validated
- **Path Traversal**: Prevented in filesystem tools
- **Output Filtering**: Blocked words configurable
- **Schema Validation**: JSON payloads verified

### Observability

Enable tracing for production monitoring:

```go
factory := harness.NewFactory(&cfg.Harness, db, logger)
// Tracer automatically configured if enable_tracing: true
```

Trace events include:

- `orchestrate_start`, `orchestrate_complete`
- `provider_call`, `tool_execution`
- `cache_hit`, `cache_miss`
- `rate_limit_acquired`

### Graceful Degradation

The harness handles failures gracefully:

- **Provider Failures**: Return error, don't crash
- **Tool Failures**: Continue with partial results
- **Cache Failures**: Fall through to provider
- **Rate Limit**: Block until available

## Roadmap (Phase 3)

Potential future enhancements:

1. **Context Window Management**: Intelligent truncation and summarization
2. **Circuit Breaker**: Automatic provider failure detection
3. **Prometheus Metrics**: Production-grade observability
4. **Tool Chaining**: Dependency-aware execution
5. **Dynamic Tool Discovery**: Runtime tool registration
6. **Hot Reload**: Configuration updates without restart

## Contributing

The harness follows strict development practices:

- **Test-Driven Development**: All features tested first
- **Hexagonal Architecture**: Clean separation of concerns
- **Table-Driven Tests**: Comprehensive input coverage
- **Benchmarks**: Performance validation required
- **Documentation**: Inline comments and godoc

## License

Part of the Virtual Vector Filesystem (VVFS) project.

## Support

For issues, questions, or contributions, please refer to the main VVFS repository.
