# LLM Harness Integration Guide

## Overview

This guide walks through integrating the LLM Harness into your application, from basic setup to advanced usage patterns. Whether you're building a chatbot, AI assistant, or automated workflow system, this guide provides practical examples for common integration scenarios.

## Table of Contents

1. [Installation & Setup](#installation--setup)
2. [Basic Integration](#basic-integration)
3. [Provider Integration](#provider-integration)
4. [Tool Development](#tool-development)
5. [Conversation Management](#conversation-management)
6. [Streaming Responses](#streaming-responses)
7. [Advanced Patterns](#advanced-patterns)
8. [Migration from Generator](#migration-from-generator)
9. [Troubleshooting](#troubleshooting)

---

## Installation & Setup

### Prerequisites

```bash
# Required dependencies
go get github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness
go get github.com/xeipuuv/gojsonschema
go get github.com/rs/zerolog
```

### Configuration File

Create `config.yaml`:

```yaml
harness:
  # Caching configuration
  cache_enabled: true
  cache_capacity: 1000
  cache_ttl_seconds: 3600
  
  # Rate limiting
  rate_limit_enabled: true
  rate_limit_capacity: 10
  rate_limit_refill_rate: 1s
  
  # Policy enforcement
  max_tool_depth: 3
  max_iterations: 10
  max_output_size: 10000
  
  # Safety guardrails
  enable_guardrails: true
  blocked_words:
    - "password"
    - "secret"
    - "api_key"
    - "token"
    - "credential"
  allowed_tools: []  # Empty = allow all
  
  # Observability
  enable_tracing: true
  tool_concurrency: 5

# Database configuration for conversation storage
database:
  dsn: "file:conversations.db"
```

### Initialize Database

```go
import (
    "database/sql"
    _ "github.com/tursodatabase/go-libsql"
)

func setupDatabase() (*sql.DB, error) {
    db, err := sql.Open("libsql", "file:conversations.db")
    if err != nil {
        return nil, err
    }
    
    // Run migrations (conversation store table creation)
    // See: vvfs/generation/harness/adapters/store_libsql.go
    
    return db, nil
}
```

---

## Basic Integration

### Minimal Working Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/rs/zerolog"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

func main() {
    // 1. Load configuration
    cfg, err := config.LoadConfig("config.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    // 2. Setup dependencies
    db, err := setupDatabase()
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
    
    // 3. Create factory
    factory := harness.NewFactory(&cfg.Harness, db, logger)
    
    // 4. Build orchestrator
    orchestrator, err := factory.CreateOrchestrator()
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. Execute a simple conversation
    req := &harness.Request{
        ConversationID: "user-demo-001",
        System:         "You are a helpful AI assistant.",
        Messages: []ports.PromptMessage{
            {Role: "user", Content: "What is the capital of France?"},
        },
        Tools: []ports.Tool{}, // No tools for basic conversation
    }
    
    ctx := context.Background()
    resp, err := orchestrator.Orchestrate(ctx, req)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Response:", resp.FinalText)
}
```

---

## Provider Integration

### Creating a Custom Provider

The harness abstracts LLM inference behind the `Provider` interface. Here's how to integrate your LLM:

```go
package myprovider

import (
    "context"
    "encoding/json"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

type MyLLMProvider struct {
    apiKey string
    model  string
}

func NewMyLLMProvider(apiKey, model string) *MyLLMProvider {
    return &MyLLMProvider{
        apiKey: apiKey,
        model:  model,
    }
}

func (p *MyLLMProvider) Complete(ctx context.Context, in ports.PromptInput, opts ports.Options) (ports.Completion, error) {
    // Convert PromptInput to your API format
    apiRequest := convertToAPIRequest(in)
    
    // Call your LLM API
    apiResponse, err := callMyLLMAPI(ctx, p.apiKey, p.model, apiRequest)
    if err != nil {
        return ports.Completion{}, err
    }
    
    // Convert response to harness format
    return ports.Completion{
        Text:      apiResponse.Text,
        ToolCalls: parseToolCalls(apiResponse),
        Usage: &ports.Usage{
            PromptTokens:     apiResponse.Usage.PromptTokens,
            CompletionTokens: apiResponse.Usage.CompletionTokens,
            TotalTokens:      apiResponse.Usage.TotalTokens,
        },
    }, nil
}

func (p *MyLLMProvider) Stream(ctx context.Context, in ports.PromptInput, opts ports.Options) (<-chan ports.CompletionChunk, error) {
    ch := make(chan ports.CompletionChunk, 10)
    
    go func() {
        defer close(ch)
        
        // Stream from your LLM API
        stream, err := callMyLLMStreamingAPI(ctx, p.apiKey, p.model, convertToAPIRequest(in))
        if err != nil {
            return
        }
        
        for chunk := range stream {
            ch <- ports.CompletionChunk{
                DeltaText: chunk.Text,
                ToolCalls: parseToolCalls(chunk),
                Usage:     chunk.Usage,
            }
        }
    }()
    
    return ch, nil
}
```

### Using Your Provider

```go
// Create your provider
provider := myprovider.NewMyLLMProvider(apiKey, "gpt-4")

// Create orchestrator manually (instead of factory)
orchestrator := harness.NewHarnessOrchestrator(
    provider,
    cache,
    rateLimiter,
    tracer,
    store,
    guardrails,
    policy,
)
```

---

## Tool Development

### Example: Database Query Tool

```go
package tools

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

const DatabaseQuerySchema = `{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "SQL SELECT query to execute"
    },
    "limit": {
      "type": "integer",
      "description": "Maximum rows to return",
      "default": 10,
      "minimum": 1,
      "maximum": 100
    }
  },
  "required": ["query"]
}`

type DatabaseQueryTool struct {
    db *sql.DB
}

func NewDatabaseQueryTool(db *sql.DB) *DatabaseQueryTool {
    return &DatabaseQueryTool{db: db}
}

func (t *DatabaseQueryTool) Name() string {
    return "database_query"
}

func (t *DatabaseQueryTool) Schema() []byte {
    return []byte(DatabaseQuerySchema)
}

func (t *DatabaseQueryTool) Invoke(ctx context.Context, args json.RawMessage) (any, error) {
    var params struct {
        Query string `json:"query"`
        Limit int    `json:"limit"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }
    
    // Validate query is SELECT only (security)
    if !isSelectQuery(params.Query) {
        return nil, fmt.Errorf("only SELECT queries are allowed")
    }
    
    // Apply default limit
    if params.Limit == 0 {
        params.Limit = 10
    }
    
    // Execute query with timeout
    queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    rows, err := t.db.QueryContext(queryCtx, params.Query)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()
    
    // Convert rows to result
    results := make([]map[string]any, 0, params.Limit)
    cols, _ := rows.Columns()
    
    for rows.Next() && len(results) < params.Limit {
        row := make(map[string]any)
        values := make([]any, len(cols))
        valuePtrs := make([]any, len(cols))
        
        for i := range cols {
            valuePtrs[i] = &values[i]
        }
        
        if err := rows.Scan(valuePtrs...); err != nil {
            continue
        }
        
        for i, col := range cols {
            row[col] = values[i]
        }
        
        results = append(results, row)
    }
    
    return map[string]any{
        "rows":  results,
        "count": len(results),
    }, nil
}

func isSelectQuery(query string) bool {
    // Simple validation - production should use SQL parser
    trimmed := strings.TrimSpace(strings.ToUpper(query))
    return strings.HasPrefix(trimmed, "SELECT")
}
```

### Example: HTTP API Tool

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const HTTPRequestSchema = `{
  "type": "object",
  "properties": {
    "url": {
      "type": "string",
      "format": "uri",
      "description": "URL to fetch"
    },
    "method": {
      "type": "string",
      "enum": ["GET", "POST"],
      "default": "GET",
      "description": "HTTP method"
    },
    "headers": {
      "type": "object",
      "description": "Additional headers",
      "additionalProperties": {"type": "string"}
    }
  },
  "required": ["url"]
}`

type HTTPRequestTool struct {
    client      *http.Client
    allowedHosts []string // Whitelist for security
}

func NewHTTPRequestTool(allowedHosts []string) *HTTPRequestTool {
    return &HTTPRequestTool{
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
        allowedHosts: allowedHosts,
    }
}

func (t *HTTPRequestTool) Name() string {
    return "http_request"
}

func (t *HTTPRequestTool) Schema() []byte {
    return []byte(HTTPRequestSchema)
}

func (t *HTTPRequestTool) Invoke(ctx context.Context, args json.RawMessage) (any, error) {
    var params struct {
        URL     string            `json:"url"`
        Method  string            `json:"method"`
        Headers map[string]string `json:"headers"`
    }
    
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid arguments: %w", err)
    }
    
    // Default method
    if params.Method == "" {
        params.Method = "GET"
    }
    
    // Security: validate host is allowed
    if !t.isAllowedHost(params.URL) {
        return nil, fmt.Errorf("host not in allowlist")
    }
    
    // Create request
    req, err := http.NewRequestWithContext(ctx, params.Method, params.URL, nil)
    if err != nil {
        return nil, err
    }
    
    // Add headers
    for k, v := range params.Headers {
        req.Header.Set(k, v)
    }
    
    // Execute
    resp, err := t.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Read response (limit size)
    body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB max
    if err != nil {
        return nil, err
    }
    
    return map[string]any{
        "status_code": resp.StatusCode,
        "headers":     resp.Header,
        "body":        string(body),
    }, nil
}

func (t *HTTPRequestTool) isAllowedHost(url string) bool {
    // Implementation left as exercise
    return true
}
```

---

## Conversation Management

### Multi-Turn Conversations

```go
func runConversation(orchestrator *harness.HarnessOrchestrator, userID string) {
    conversationID := fmt.Sprintf("user-%s-session-%d", userID, time.Now().Unix())
    
    // Turn 1
    resp1, _ := orchestrator.Orchestrate(context.Background(), &harness.Request{
        ConversationID: conversationID,
        System:         "You are a helpful assistant.",
        Messages: []ports.PromptMessage{
            {Role: "user", Content: "What's the weather in Paris?"},
        },
        Tools: []ports.Tool{weatherTool},
    })
    
    fmt.Println("AI:", resp1.FinalText)
    
    // Turn 2 - context is automatically loaded from store
    resp2, _ := orchestrator.Orchestrate(context.Background(), &harness.Request{
        ConversationID: conversationID,
        Messages: []ports.PromptMessage{
            {Role: "user", Content: "What about London?"},
        },
        Tools: []ports.Tool{weatherTool},
    })
    
    fmt.Println("AI:", resp2.FinalText)
}
```

### Loading Previous Context

The orchestrator automatically loads recent context from the `ConversationStore`. To manually manage:

```go
// Load last 10 turns
store := adapters.NewLibSQLConversationStore(db)
turns, err := store.LoadContext(ctx, conversationID, 10)

// Optionally filter or transform before passing to orchestrator
messages := turnsToMessages(turns)
```

---

## Streaming Responses

### Basic Streaming

```go
func handleStreamingRequest(orchestrator *harness.HarnessOrchestrator) {
    req := &harness.Request{
        ConversationID: "stream-demo",
        System:         "You are a helpful assistant.",
        Messages: []ports.PromptMessage{
            {Role: "user", Content: "Explain quantum computing"},
        },
    }
    
    ctx := context.Background()
    respCh, errCh := orchestrator.StreamOrchestrate(ctx, req)
    
    for resp := range respCh {
        // Print incremental text
        fmt.Print(resp.FinalText)
        
        // Handle tool calls
        if len(resp.ToolCalls) > 0 {
            fmt.Println("\n[Tools triggered:", len(resp.ToolCalls), "]")
        }
    }
    
    if err := <-errCh; err != nil {
        log.Fatal(err)
    }
}
```

### Web Server Streaming (Server-Sent Events)

```go
func handleSSEStream(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        return
    }
    
    // Parse request
    var req harness.Request
    json.NewDecoder(r.Body).Decode(&req)
    
    // Stream response
    respCh, errCh := orchestrator.StreamOrchestrate(r.Context(), &req)
    
    for resp := range respCh {
        data, _ := json.Marshal(resp)
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush()
    }
    
    if err := <-errCh; err != nil {
        data, _ := json.Marshal(map[string]string{"error": err.Error()})
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush()
    }
}
```

---

## Advanced Patterns

### Custom Policy Enforcement

```go
// Create custom policy
customPolicy := &harness.Policy{
    MaxToolDepth:   5,  // Deep tool chains
    MaxIterations:  15, // More iterations
    MaxOutputSize:  50000, // Larger outputs
    ToolTimeout:    60 * time.Second, // Longer tool execution
}

// Override policy in request
orchestrator.Orchestrate(ctx, &harness.Request{
    // ... other fields ...
    Policy: customPolicy, // Override default
})
```

### Dynamic Tool Selection

```go
func selectToolsForIntent(intent string) []ports.Tool {
    toolRegistry := map[string]ports.Tool{
        "weather":  weatherTool,
        "database": dbTool,
        "search":   searchTool,
        "calendar": calendarTool,
    }
    
    // Use intent classification or keywords
    tools := []ports.Tool{}
    
    if strings.Contains(intent, "weather") || strings.Contains(intent, "temperature") {
        tools = append(tools, toolRegistry["weather"])
    }
    
    if strings.Contains(intent, "schedule") || strings.Contains(intent, "meeting") {
        tools = append(tools, toolRegistry["calendar"])
    }
    
    // Default: provide search
    if len(tools) == 0 {
        tools = append(tools, toolRegistry["search"])
    }
    
    return tools
}
```

### Conversation Summarization

```go
func summarizeConversation(store ports.ConversationStore, convID string) (string, error) {
    // Load full conversation
    turns, err := store.LoadContext(context.Background(), convID, 100)
    if err != nil {
        return "", err
    }
    
    // Build summary prompt
    transcript := ""
    for _, turn := range turns {
        transcript += fmt.Sprintf("%s: %s\n", turn.Role, turn.Content)
    }
    
    // Use orchestrator to summarize
    summaryReq := &harness.Request{
        ConversationID: convID + "-summary",
        System:         "Summarize the following conversation in 3 sentences:",
        Messages: []ports.PromptMessage{
            {Role: "user", Content: transcript},
        },
    }
    
    resp, err := orchestrator.Orchestrate(context.Background(), summaryReq)
    if err != nil {
        return "", err
    }
    
    return resp.FinalText, nil
}
```

---

## Migration from Generator

If you're using the existing `Generator` interface, use the bridge:

```go
import "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation"

// Old code
oldGenerator := generation.NewHugotGenerator(/* ... */)

// New code - use bridge
orchestrator, _ := factory.CreateOrchestrator()
newGenerator := generation.NewHarnessGenerator(orchestrator, "conversation-id")

// Same interface works
resp, err := newGenerator.Generate(ctx, &generation.GenerationRequest{
    Messages: []generation.Message{
        {Role: "user", Content: "Hello"},
    },
})
```

---

## Troubleshooting

### Common Issues

#### 1. Tool Not Being Called

**Symptoms**: LLM doesn't invoke available tools

**Solutions**:

- Ensure tool schema is valid JSON Schema
- Check tool name doesn't contain special characters
- Verify `allowed_tools` config includes your tool
- Add explicit tool instructions to system prompt

```go
// Explicit tool prompt
req.System = "You have access to a 'weather' tool. Use it when asked about weather."
```

#### 2. Rate Limit Exceeded

**Symptoms**: `rate limit exceeded` errors

**Solutions**:

- Increase `rate_limit_capacity` in config
- Decrease `rate_limit_refill_rate`
- Use different keys for different users

```go
// Per-user rate limiting
factory.createRateLimiter().Acquire(ctx, userID)
```

#### 3. Conversation Context Lost

**Symptoms**: AI doesn't remember previous turns

**Solutions**:

- Verify same `ConversationID` used across turns
- Check database connection is valid
- Ensure `store.SaveTurn` is called after each response
- Increase context window in `LoadContext(k)`

#### 4. Streaming Responses Incomplete

**Symptoms**: Streaming stops mid-response

**Solutions**:

- Check context isn't cancelled prematurely
- Verify provider streaming implementation
- Ensure channel isn't closed early
- Check for network timeouts

### Debug Mode

Enable verbose logging:

```go
logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).With().Timestamp().Logger()
factory := harness.NewFactory(&cfg.Harness, db, logger)
```

Trace events will be logged:

```
{"level":"debug","span":"orchestrate_start","conversation_id":"user-123"}
{"level":"debug","span":"provider_call","tokens":1234}
{"level":"debug","span":"tool_execution","tool":"weather","duration_ms":45}
{"level":"debug","span":"orchestrate_complete","iterations":2}
```

### Performance Profiling

```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// Access profiler at http://localhost:6060/debug/pprof/
```

---

## Best Practices Summary

1. **Configuration**: Use config files for production, hardcoded values for testing
2. **Error Handling**: Always check errors and provide context
3. **Context Propagation**: Pass `context.Context` through all layers
4. **Tool Design**: Validate inputs, handle timeouts, return structured data
5. **Security**: Validate all tool inputs, use allowlists, sanitize outputs
6. **Testing**: Write unit tests for custom tools and providers
7. **Observability**: Enable tracing in production
8. **Resource Management**: Close database connections, cancel contexts
9. **Graceful Degradation**: Handle partial failures without crashing
10. **Documentation**: Document custom tools and providers thoroughly

---

## Next Steps

- Review the [README](README.md) for architecture details
- Explore example tools in `vvfs/generation/harness/tools/`
- Check test suite for usage patterns: `vvfs/generation/harness/harness_test.go`
- Implement custom providers and tools for your use case
- Deploy with proper configuration and monitoring

For production deployments, consider Phase 3 enhancements (circuit breakers, Prometheus metrics, etc.) outlined in the main README.
