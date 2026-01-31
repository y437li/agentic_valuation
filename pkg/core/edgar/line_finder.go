package edgar

import (
	"regexp"
	"strings"
)

// FindLineNumber searches for a label in markdown content and returns the line number (1-indexed).
// Returns 0 if not found. Uses case-insensitive matching.
// IMPROVED: Prioritizes table rows (lines starting with |) over paragraph text.
// Priority order: 1) Table row with label+numbers, 2) Table row with label, 3) Any row with label+numbers, 4) First match
func FindLineNumber(markdown string, label string) int {
	if label == "" || markdown == "" {
		return 0
	}

	lines := strings.Split(markdown, "\n")
	// Normalize the search label
	searchLabel := strings.TrimSpace(strings.ToLower(label))

	// Regex to detect currency/numeric values (e.g., "12,345", "$1,234", "(1,234)", "1234")
	numericPattern := regexp.MustCompile(`[\d,$()]+\d{2,}`)

	var (
		tableWithNumeric int = 0 // Priority 1: Table row + numbers
		tableMatch       int = 0 // Priority 2: Table row (any)
		numericMatch     int = 0 // Priority 3: Line with numbers (non-table)
		firstMatch       int = 0 // Priority 4: First occurrence
	)

	for i, line := range lines {
		normalizedLine := strings.ToLower(line)
		if !strings.Contains(normalizedLine, searchLabel) {
			continue
		}

		lineNum := i + 1
		trimmedLine := strings.TrimSpace(line)
		isTableRow := strings.HasPrefix(trimmedLine, "|")
		hasNumbers := numericPattern.MatchString(line)

		// Priority 1: Table row with numeric values (best match)
		if isTableRow && hasNumbers && tableWithNumeric == 0 {
			tableWithNumeric = lineNum
		}

		// Priority 2: Table row without numbers
		if isTableRow && tableMatch == 0 {
			tableMatch = lineNum
		}

		// Priority 3: Non-table line with numbers
		if !isTableRow && hasNumbers && numericMatch == 0 {
			numericMatch = lineNum
		}

		// Priority 4: First match as fallback
		if firstMatch == 0 {
			firstMatch = lineNum
		}
	}

	// Return in priority order
	if tableWithNumeric > 0 {
		return tableWithNumeric
	}
	if tableMatch > 0 {
		return tableMatch
	}
	if numericMatch > 0 {
		return numericMatch
	}
	return firstMatch
}
