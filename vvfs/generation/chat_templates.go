package generation

import (
	"text/template"
)

// Chat templates for different models
// Based on model specifications and Hugot's existing templates

const GemmaTemplate = `<start_of_turn>user
{{.Content}}<end_of_turn>
<start_of_turn>model
{{if .AddGenerationPrompt}}{{end}}`

const LFM2Template = `{{range .Messages}}{{if eq .Role "system"}}<|im_start|>system
{{.Content}}<|im_end|>
{{else if eq .Role "user"}}<|im_start|>user
{{.Content}}<|im_end|>
{{else if eq .Role "assistant"}}<|im_start|>assistant
{{.Content}}<|im_end|>
{{end}}{{end}}{{if .AddGenerationPrompt}}<|im_start|>assistant
{{else}}<|im_end|>{{end}}`

// GetChatTemplate returns the appropriate chat template for a model
func GetChatTemplate(modelName string) *template.Template {
	var templateStr string

	switch {
	case contains(modelName, "gemma"):
		templateStr = GemmaTemplate
	case contains(modelName, "lfm2"):
		templateStr = LFM2Template
	default:
		// Default to LFM2 template for compatibility
		templateStr = LFM2Template
	}

	tmpl, err := template.New("chat").Parse(templateStr)
	if err != nil {
		// Fallback to simple template
		fallback := `<|im_start|>user
{{.Content}}<|im_end|>
<|im_start|>assistant
{{if .AddGenerationPrompt}}{{end}}`
		tmpl, _ = template.New("chat").Parse(fallback)
	}

	return tmpl
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr))
}
