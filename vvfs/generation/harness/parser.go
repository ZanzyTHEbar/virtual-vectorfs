package harness

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
)

// OutputParser handles extracting structured data from model responses.
type OutputParser struct {
	// Regex patterns for different tool call formats
	toolCallPatterns []*regexp.Regexp
}

// NewOutputParser creates a parser with default patterns for common tool call formats.
func NewOutputParser() *OutputParser {
	return &OutputParser{
		toolCallPatterns: []*regexp.Regexp{
			// JSON array format: [{"name": "tool", "arguments": {...}}]
			regexp.MustCompile(`\[\s*\{\s*"name"\s*:\s*"([^"]+)"\s*,\s*"arguments"\s*:\s*(\{.*?\})\s*\}\s*\]`),
			// Function call format: tool_name({"arg": "value"})
			regexp.MustCompile(`(\w+)\s*\(\s*(\{.*?\})\s*\)`),
			// OpenAI format: {"tool_calls": [{"function": {"name": "tool", "arguments": "..."}}]}
			regexp.MustCompile(`"tool_calls"\s*:\s*\[\s*\{\s*"function"\s*:\s*\{\s*"name"\s*:\s*"([^"]+)"\s*,\s*"arguments"\s*:\s*"(\{.*?\})"\s*\}\s*\}\s*\]`),
		},
	}
}

// ParseToolCalls extracts tool calls from a model response text.
func (p *OutputParser) ParseToolCalls(text string) []ports.ToolCall {
	var calls []ports.ToolCall

	// Try each pattern
	for _, pattern := range p.toolCallPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				name := strings.TrimSpace(match[1])
				argsStr := strings.TrimSpace(match[2])

				var args json.RawMessage
				if json.Valid([]byte(argsStr)) {
					args = json.RawMessage(argsStr)
				} else {
					// Try to fix common JSON issues
					argsStr = p.fixJSON(argsStr)
					if json.Valid([]byte(argsStr)) {
						args = json.RawMessage(argsStr)
					} else {
						// Skip invalid JSON
						continue
					}
				}

				calls = append(calls, ports.ToolCall{
					Name: name,
					Args: args,
				})
			}
		}
	}

	return calls
}

// ParseJSONOutput attempts to extract JSON from text when JSON mode is required.
func (p *OutputParser) ParseJSONOutput(text string) (json.RawMessage, error) {
	// Look for JSON objects or arrays in the text
	jsonPattern := regexp.MustCompile(`(\{.*\}|\[.*\])`)
	match := jsonPattern.FindString(text)

	if match == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Clean up the JSON
	cleaned := p.fixJSON(match)
	if !json.Valid([]byte(cleaned)) {
		return nil, fmt.Errorf("invalid JSON in response")
	}

	return json.RawMessage(cleaned), nil
}

// fixJSON attempts to fix common JSON formatting issues.
func (p *OutputParser) fixJSON(jsonStr string) string {
	// Remove trailing commas before closing braces/brackets
	jsonStr = regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(jsonStr, "$1")

	// Fix unquoted keys (basic heuristic)
	jsonStr = regexp.MustCompile(`([{,]\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*:`).ReplaceAllString(jsonStr, `$1"$2":`)

	// Fix single quotes to double quotes
	jsonStr = strings.ReplaceAll(jsonStr, "'", "\"")

	return jsonStr
}

// ValidateToolCall checks if a tool call is well-formed.
func (p *OutputParser) ValidateToolCall(call ports.ToolCall) error {
	if call.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if !json.Valid(call.Args) {
		return fmt.Errorf("tool arguments are not valid JSON")
	}

	return nil
}

// ValidateJSONOutput validates that JSON output matches expected schema (if provided).
func (p *OutputParser) ValidateJSONOutput(data json.RawMessage, schema []byte) error {
	if len(schema) == 0 {
		return nil // no schema to validate against
	}

	// Basic validation - in production, use a proper JSON schema validator
	if !json.Valid(data) {
		return fmt.Errorf("output is not valid JSON")
	}

	return nil
}

// parseToolCalls is a method on HarnessOrchestrator that delegates to OutputParser.
func (o *HarnessOrchestrator) parseToolCalls(text string) []ports.ToolCall {
	parser := NewOutputParser()
	return parser.ParseToolCalls(text)
}
