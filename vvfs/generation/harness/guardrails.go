package harness

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	ports "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/generation/harness/ports"
	"github.com/xeipuuv/gojsonschema"
)

// Guardrails enforces safety, validation, and policy compliance.
type Guardrails struct {
	allowlist     map[string]bool  // allowed tool names
	blockedWords  []string         // words that should not appear in output
	outputFilters []*regexp.Regexp // regex patterns for filtering output
	jsonValidator *JSONValidator   // for schema validation
}

// NewGuardrails creates guardrails with default safety settings.
func NewGuardrails() *Guardrails {
	return &Guardrails{
		allowlist: make(map[string]bool),
		blockedWords: []string{
			"password", "secret", "key", "token", "credential",
		},
		outputFilters: []*regexp.Regexp{
			regexp.MustCompile(`(?i)password[:=]\s*\S+`),
			regexp.MustCompile(`(?i)api[_-]?key[:=]\s*\S+`),
			regexp.MustCompile(`(?i)secret[:=]\s*\S+`),
		},
		jsonValidator: NewJSONValidator(),
	}
}

// AddAllowedTool adds a tool to the allowlist.
func (g *Guardrails) AddAllowedTool(name string) {
	g.allowlist[name] = true
}

// RemoveAllowedTool removes a tool from the allowlist.
func (g *Guardrails) RemoveAllowedTool(name string) {
	delete(g.allowlist, name)
}

// ValidateToolCall checks if a tool call is allowed and well-formed.
func (g *Guardrails) ValidateToolCall(call ports.ToolCall) error {
	// Check allowlist
	if !g.allowlist[call.Name] {
		return fmt.Errorf("tool %s is not in allowlist", call.Name)
	}

	// Basic validation
	if call.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if !json.Valid(call.Args) {
		return fmt.Errorf("tool arguments are not valid JSON")
	}

	// Check for blocked content in arguments
	argsStr := string(call.Args)
	for _, word := range g.blockedWords {
		if strings.Contains(strings.ToLower(argsStr), word) {
			return fmt.Errorf("tool arguments contain blocked content: %s", word)
		}
	}

	return nil
}

// ValidateOutput checks if output passes safety filters.
func (g *Guardrails) ValidateOutput(output string) error {
	// Check for blocked words
	lowerOutput := strings.ToLower(output)
	for _, word := range g.blockedWords {
		if strings.Contains(lowerOutput, word) {
			return fmt.Errorf("output contains blocked word: %s", word)
		}
	}

	// Check regex filters
	for _, filter := range g.outputFilters {
		if filter.MatchString(output) {
			return fmt.Errorf("output matches blocked pattern")
		}
	}

	return nil
}

// ValidateJSONOutput validates JSON output against a schema if provided.
func (g *Guardrails) ValidateJSONOutput(data json.RawMessage, schema []byte) error {
	return g.jsonValidator.Validate(data, schema)
}

// SanitizeOutput removes or masks sensitive information from output.
func (g *Guardrails) SanitizeOutput(output string) string {
	sanitized := output

	// Apply regex filters to mask sensitive data
	for _, filter := range g.outputFilters {
		sanitized = filter.ReplaceAllString(sanitized, "[REDACTED]")
	}

	return sanitized
}

// JSONValidator handles JSON schema validation.
type JSONValidator struct{}

// NewJSONValidator creates a new JSON validator.
func NewJSONValidator() *JSONValidator {
	return &JSONValidator{}
}

// Validate checks if JSON data conforms to a schema.
func (v *JSONValidator) Validate(data json.RawMessage, schema []byte) error {
	if len(schema) == 0 {
		return nil // no schema to validate against
	}

	// First check basic JSON validity
	if !json.Valid(data) {
		return fmt.Errorf("data is not valid JSON")
	}

	// Parse schema
	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewBytesLoader(data)

	// Validate against schema
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		return fmt.Errorf("schema validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// PolicyValidator enforces orchestration policies.
type PolicyValidator struct {
	maxToolDepth  int
	maxIterations int
	maxOutputSize int
}

// NewPolicyValidator creates a policy validator with defaults.
func NewPolicyValidator() *PolicyValidator {
	return &PolicyValidator{
		maxToolDepth:  3,
		maxIterations: 10,
		maxOutputSize: 10000, // 10KB max output
	}
}

// ValidateDepth checks if tool depth is within limits.
func (v *PolicyValidator) ValidateDepth(currentDepth int) error {
	if currentDepth > v.maxToolDepth {
		return fmt.Errorf("tool depth %d exceeds maximum %d", currentDepth, v.maxToolDepth)
	}
	return nil
}

// ValidateIteration checks if iteration count is within limits.
func (v *PolicyValidator) ValidateIteration(currentIteration int) error {
	if currentIteration > v.maxIterations {
		return fmt.Errorf("iteration %d exceeds maximum %d", currentIteration, v.maxIterations)
	}
	return nil
}

// ValidateOutputSize checks if output size is within limits.
func (v *PolicyValidator) ValidateOutputSize(output string) error {
	if len(output) > v.maxOutputSize {
		return fmt.Errorf("output size %d exceeds maximum %d", len(output), v.maxOutputSize)
	}
	return nil
}
