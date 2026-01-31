package edgar

import (
	"strings"
)

// DetectScaleFactor analyzes the table markdown/header to find unit multipliers.
// Returns factor (e.g. 1000000.0) and unit name (e.g. "millions").
func (e *GoExtractor) DetectScaleFactor(text string) (float64, string) {
	text = strings.ToLower(text)

	// Check for millions
	if strings.Contains(text, "millions") || strings.Contains(text, "million") {
		return 1000000.0, "millions"
	}

	// Check for thousands
	if strings.Contains(text, "thousands") || strings.Contains(text, "thousand") || strings.Contains(text, "000s") {
		return 1000.0, "thousands"
	}

	// Check for billions (rare but possible)
	if strings.Contains(text, "billions") || strings.Contains(text, "billion") {
		return 1000000000.0, "billions"
	}

	return 1.0, ""
}
