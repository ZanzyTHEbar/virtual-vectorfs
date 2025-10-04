package harnessports

import (
	"context"
	"encoding/json"
)

// ToolSpec describes a callable tool exposed to the model.
type ToolSpec struct {
	Name        string // unique logical name
	Description string // concise doc for model selection
	JSONSchema  []byte // JSON schema for args (Draft 2020-12 recommended)
}

// ToolCall represents a model-invoked function with JSON arguments.
type ToolCall struct {
	Name string
	Args json.RawMessage
}

// Tool defines the runtime that executes a tool call.
type Tool interface {
	Name() string
	Schema() []byte
	Invoke(ctx context.Context, args json.RawMessage) (any, error)
}
