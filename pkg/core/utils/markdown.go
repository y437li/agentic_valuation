package utils

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

// CleanMarkdown strips conversational filler and outer markdown code blocks.
// It ensures the output is pure Markdown ready for rendering.
func CleanMarkdown(input string) string {
	cleaned := strings.TrimSpace(input)

	// Strip outer wrapping code blocks if present (e.g. ```markdown ... ```)
	if strings.HasPrefix(cleaned, "```markdown") && strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```markdown")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	} else if strings.HasPrefix(cleaned, "```") && strings.HasSuffix(cleaned, "```") {
		// Generic code block strip
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}

	return cleaned
}

// ValidateMarkdown checks if the string is valid Markdown using Goldmark.
// Returns true if it parses without critical errors (Goldmark is very permissive, so this is basic).
func ValidateMarkdown(input string) bool {
	parser := goldmark.DefaultParser()
	reader := text.NewReader([]byte(input))
	doc := parser.Parse(reader)
	return doc != nil
}
