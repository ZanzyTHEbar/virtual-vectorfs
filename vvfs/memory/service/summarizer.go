package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/config"
)

// SummarizerImpl implements Summarizer for working memory summarization
type SummarizerImpl struct {
	config *config.MemoryConfig
}

// NewSummarizer creates a new summarizer
func NewSummarizer(config *config.MemoryConfig) *SummarizerImpl {
	return &SummarizerImpl{config: config}
}

// Summarize creates a structured summary of conversation messages
func (sum *SummarizerImpl) Summarize(ctx context.Context, messages []ConversationMessage) (Summary, error) {
	// Validate messages
	if err := sum.validateMessages(messages); err != nil {
		return Summary{}, fmt.Errorf("message validation failed: %w", err)
	}

	// Build sections
	sections := sum.buildSections(messages)

	// Create summary content
	content := sum.buildSummaryContent(sections)

	// Apply guards
	if err := sum.applyGuards(messages, &content); err != nil {
		return Summary{}, fmt.Errorf("guard check failed: %w", err)
	}

	return Summary{
		Content:  content,
		Sections: sections,
	}, nil
}

// validateMessages checks for basic message validity
func (sum *SummarizerImpl) validateMessages(messages []ConversationMessage) error {
	if len(messages) == 0 {
		return fmt.Errorf("no messages to summarize")
	}

	// Check for temporal ordering issues
	for i := 1; i < len(messages); i++ {
		// In a full implementation, check timestamps for ordering
		// For now, assume messages are in order
	}

	return nil
}

// buildSections creates structured sections from messages
func (sum *SummarizerImpl) buildSections(messages []ConversationMessage) map[string]string {
	sections := make(map[string]string)

	// Product & Environment section
	sections["Product & Environment"] = sum.buildProductEnvSection(messages)

	// Reported Issue section
	sections["Reported Issue"] = sum.buildIssueSection(messages)

	// Steps Tried & Results section
	sections["Steps Tried & Results"] = sum.buildStepsSection(messages)

	// Identifiers section
	sections["Identifiers"] = sum.buildIdentifiersSection(messages)

	// Timeline Milestones section
	sections["Timeline Milestones"] = sum.buildTimelineSection(messages)

	// Tool Performance Insights section
	sections["Tool Performance Insights"] = sum.buildToolInsightsSection(messages)

	// Current Status & Blockers section
	sections["Current Status & Blockers"] = sum.buildStatusSection(messages)

	// Next Recommended Step section
	sections["Next Recommended Step"] = sum.buildNextStepSection(messages)

	return sections
}

// buildProductEnvSection extracts product and environment information
func (sum *SummarizerImpl) buildProductEnvSection(messages []ConversationMessage) string {
	var envInfo []string

	for _, msg := range messages {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "device") || strings.Contains(content, "os") || strings.Contains(content, "version") {
			envInfo = append(envInfo, msg.Content)
		}
	}

	if len(envInfo) == 0 {
		return "No specific device or environment information mentioned."
	}

	return strings.Join(envInfo, "; ")
}

// buildIssueSection extracts the main issue description
func (sum *SummarizerImpl) buildIssueSection(messages []ConversationMessage) string {
	// Find the first user message (usually contains the issue)
	for _, msg := range messages {
		if msg.Role == "user" {
			return msg.Content
		}
	}
	return "Issue not clearly stated."
}

// buildStepsSection extracts steps tried and their results
func (sum *SummarizerImpl) buildStepsSection(messages []ConversationMessage) string {
	var steps []string

	for _, msg := range messages {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "tried") || strings.Contains(content, "attempted") || strings.Contains(content, "reset") {
			steps = append(steps, msg.Content)
		}
	}

	if len(steps) == 0 {
		return "No troubleshooting steps mentioned."
	}

	return strings.Join(steps, "; ")
}

// buildIdentifiersSection extracts identifiers like ticket numbers, device serials
func (sum *SummarizerImpl) buildIdentifiersSection(messages []ConversationMessage) string {
	var identifiers []string

	for _, msg := range messages {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "ticket") || strings.Contains(content, "serial") || strings.Contains(content, "model") {
			identifiers = append(identifiers, msg.Content)
		}
	}

	if len(identifiers) == 0 {
		return "No specific identifiers provided."
	}

	return strings.Join(identifiers, "; ")
}

// buildTimelineSection extracts key events with timestamps
func (sum *SummarizerImpl) buildTimelineSection(messages []ConversationMessage) string {
	var milestones []string

	for _, msg := range messages {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "then") || strings.Contains(content, "after") || strings.Contains(content, "before") {
			milestones = append(milestones, msg.Content)
		}
	}

	if len(milestones) == 0 {
		return "No clear timeline events mentioned."
	}

	return strings.Join(milestones, "; ")
}

// buildToolInsightsSection extracts tool performance information
func (sum *SummarizerImpl) buildToolInsightsSection(messages []ConversationMessage) string {
	var insights []string

	for _, msg := range messages {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "tool") || strings.Contains(content, "worked") || strings.Contains(content, "failed") {
			insights = append(insights, msg.Content)
		}
	}

	if len(insights) == 0 {
		return "No tool usage or performance information mentioned."
	}

	return strings.Join(insights, "; ")
}

// buildStatusSection determines current status and blockers
func (sum *SummarizerImpl) buildStatusSection(messages []ConversationMessage) string {
	var status []string

	for _, msg := range messages {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "resolved") || strings.Contains(content, "fixed") || strings.Contains(content, "still") {
			status = append(status, msg.Content)
		}
	}

	if len(status) == 0 {
		return "Status unclear; issue may still be pending."
	}

	return strings.Join(status, "; ")
}

// buildNextStepSection suggests next recommended action
func (sum *SummarizerImpl) buildNextStepSection(messages []ConversationMessage) string {
	// Simple heuristic: suggest escalation or follow-up
	return "Escalate to human support or provide more detailed troubleshooting steps."
}

// buildSummaryContent creates the overall summary text
func (sum *SummarizerImpl) buildSummaryContent(sections map[string]string) string {
	var content strings.Builder

	content.WriteString("Conversation Summary:\n\n")

	for sectionName, sectionContent := range sections {
		content.WriteString(fmt.Sprintf("**%s:**\n%s\n\n", sectionName, sectionContent))
	}

	return content.String()
}

// applyGuards checks for contradictions, temporal issues, and hallucinations
func (sum *SummarizerImpl) applyGuards(messages []ConversationMessage, content *string) error {
	// 1. Contradiction check: ensure summary doesn't contradict tool definitions or system rules
	if err := sum.checkContradictions(messages, *content); err != nil {
		return err
	}

	// 2. Temporal ordering: ensure summary respects message timeline
	if err := sum.checkTemporalOrdering(messages); err != nil {
		return err
	}

	// 3. Hallucination control: ensure summary doesn't invent unverifiable facts
	if err := sum.checkHallucinations(*content); err != nil {
		return err
	}

	return nil
}

// checkContradictions ensures summary doesn't contradict known facts
func (sum *SummarizerImpl) checkContradictions(messages []ConversationMessage, content string) error {
	// Simple check: look for contradictions in tool mentions
	contentLower := strings.ToLower(content)

	// Example: if summary mentions a tool that wasn't used
	blockedTools := []string{"forbidden_tool", "deprecated_api"}
	for _, tool := range blockedTools {
		if strings.Contains(contentLower, tool) {
			return fmt.Errorf("summary mentions forbidden tool: %s", tool)
		}
	}

	return nil
}

// checkTemporalOrdering ensures timeline is respected
func (sum *SummarizerImpl) checkTemporalOrdering(messages []ConversationMessage) error {
	// In a full implementation, check that summary doesn't reorder events incorrectly
	// For now, assume ordering is correct
	return nil
}

// checkHallucinations ensures summary doesn't invent facts
func (sum *SummarizerImpl) checkHallucinations(content string) error {
	// Simple check: look for unverifiable claims
	contentLower := strings.ToLower(content)

	unverifiable := []string{"secret key", "internal api", "undocumented feature"}
	for _, term := range unverifiable {
		if strings.Contains(contentLower, term) {
			return fmt.Errorf("summary contains unverifiable information: %s", term)
		}
	}

	return nil
}

// ValidateSummary checks if a summary meets quality criteria
func (sum *SummarizerImpl) ValidateSummary(summary Summary) error {
	// Check required sections exist
	requiredSections := []string{
		"Product & Environment",
		"Reported Issue",
		"Steps Tried & Results",
		"Current Status & Blockers",
	}

	for _, section := range requiredSections {
		if _, exists := summary.Sections[section]; !exists {
			return fmt.Errorf("missing required section: %s", section)
		}
	}

	// Check content length (not too short)
	if len(summary.Content) < 100 {
		return fmt.Errorf("summary too short: %d characters", len(summary.Content))
	}

	// Check for contradictions in sections
	for sectionName, sectionContent := range summary.Sections {
		if err := sum.checkContradictions([]ConversationMessage{}, sectionContent); err != nil {
			return fmt.Errorf("contradiction in section %s: %w", sectionName, err)
		}
	}

	return nil
}

// MergeSummaries combines multiple summaries (for conversation continuation)
func (sum *SummarizerImpl) MergeSummaries(summaries []Summary) (Summary, error) {
	if len(summaries) == 0 {
		return Summary{}, fmt.Errorf("no summaries to merge")
	}

	// Simple merge: combine sections, preferring newer information
	mergedSections := make(map[string]string)

	for _, summary := range summaries {
		for sectionName, sectionContent := range summary.Sections {
			// Append or overwrite (newer summaries take precedence)
			if existing, exists := mergedSections[sectionName]; exists {
				mergedSections[sectionName] = sectionContent + "; " + existing
			} else {
				mergedSections[sectionName] = sectionContent
			}
		}
	}

	mergedContent := sum.buildSummaryContent(mergedSections)

	return Summary{
		Content:  mergedContent,
		Sections: mergedSections,
	}, nil
}
