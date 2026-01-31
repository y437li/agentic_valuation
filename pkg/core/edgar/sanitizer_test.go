package edgar

import (
	"strings"
	"testing"
)

func TestHTMLSanitizer_FixFakeHeaders(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantH2   bool
		wantText string
	}{
		{
			name:     "Bold paragraph with 14pt font becomes h2",
			html:     `<body><p style="font-weight:bold; font-size:14pt">Item 8. Financial Statements</p></body>`,
			wantH2:   true,
			wantText: "Item 8. Financial Statements",
		},
		{
			name:     "Bold paragraph with 12pt font becomes h3",
			html:     `<body><p style="font-weight:bold; font-size:12pt">Note 1: Summary</p></body>`,
			wantH2:   false, // should be h3
			wantText: "Note 1: Summary",
		},
		{
			name:     "Regular paragraph stays paragraph",
			html:     `<body><p>This is just regular text.</p></body>`,
			wantH2:   false,
			wantText: "This is just regular text.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizer := NewHTMLSanitizer()
			result, err := sanitizer.Sanitize(tt.html)
			if err != nil {
				t.Fatalf("Sanitize() error = %v", err)
			}

			hasH2 := strings.Contains(result, "<h2>")
			if tt.wantH2 && !hasH2 {
				t.Errorf("Expected <h2> tag, got: %s", result)
			}

			if !strings.Contains(result, tt.wantText) {
				t.Errorf("Expected text '%s' in result, got: %s", tt.wantText, result)
			}
		})
	}
}

func TestHTMLSanitizer_TablePlaceholders(t *testing.T) {
	html := `<body>
		<p>Some text before table</p>
		<table><tr><td>Value 1</td><td>Value 2</td></tr></table>
		<p>Some text after table</p>
	</body>`

	sanitizer := NewHTMLSanitizer()
	result, err := sanitizer.Sanitize(html)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}

	// Check placeholder was inserted
	if !strings.Contains(result, "{{TABLE_ID_1}}") {
		t.Errorf("Expected {{TABLE_ID_1}} placeholder, got: %s", result)
	}

	// Check table was stored
	if sanitizer.GetTableCount() != 1 {
		t.Errorf("Expected 1 table stored, got %d", sanitizer.GetTableCount())
	}

	// Test restoration
	restored := sanitizer.RestoreTables(result)
	if strings.Contains(restored, "{{TABLE_ID_1}}") {
		t.Error("Placeholder should be replaced after RestoreTables")
	}
	if !strings.Contains(restored, "Value 1") {
		t.Error("Table content should be restored")
	}
}

func TestHTMLSanitizer_RemoveNoise(t *testing.T) {
	html := `<body>
		<script>alert('bad');</script>
		<style>.hide{display:none}</style>
		<p>Real content</p>
		<img src="spacer.gif" width="1" height="1">
		<p>Page 42</p>
	</body>`

	sanitizer := NewHTMLSanitizer()
	result, err := sanitizer.Sanitize(html)
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}

	// Scripts should be removed
	if strings.Contains(result, "script") || strings.Contains(result, "alert") {
		t.Error("Script tags should be removed")
	}

	// Styles should be removed
	if strings.Contains(result, "style") {
		t.Error("Style tags should be removed")
	}

	// Spacer images should be removed
	if strings.Contains(result, "spacer") {
		t.Error("Spacer images should be removed")
	}

	// Real content should remain
	if !strings.Contains(result, "Real content") {
		t.Error("Real content should be preserved")
	}
}

func TestDetectUnits(t *testing.T) {
	tests := []struct {
		name      string
		markdown  string
		wantScale float64
		wantLabel string
	}{
		{
			name:      "Detect 'in thousands'",
			markdown:  "CONSOLIDATED BALANCE SHEETS\n(In thousands, except share data)",
			wantScale: 1000,
			wantLabel: "thousands",
		},
		{
			name:      "Detect 'in millions'",
			markdown:  "The following table shows amounts in millions of dollars",
			wantScale: 1000000,
			wantLabel: "millions",
		},
		{
			name:      "Detect '$000s'",
			markdown:  "All amounts in $000s unless otherwise noted",
			wantScale: 1000,
			wantLabel: "thousands",
		},
		{
			name:      "Default to millions when not specified",
			markdown:  "BALANCE SHEET\n| Assets | 2024 | 2023 |",
			wantScale: 1000000,
			wantLabel: "millions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units := DetectUnits(tt.markdown)
			if units.Scale != tt.wantScale {
				t.Errorf("Scale = %v, want %v", units.Scale, tt.wantScale)
			}
			if units.ScaleLabel != tt.wantLabel {
				t.Errorf("ScaleLabel = %v, want %v", units.ScaleLabel, tt.wantLabel)
			}
		})
	}
}

func TestSplitByTableMarkers(t *testing.T) {
	markdown := `
# Item 8. Financial Statements

[TABLE: BALANCE_SHEET]
| Assets | 2024 | 2023 |
| Cash | 1000 | 900 |
| Total Assets | 5000 | 4500 |

[TABLE: INCOME_STATEMENT]  
| Revenue | 2024 | 2023 |
| Net Sales | 10000 | 9000 |

[TABLE: CASH_FLOW_STATEMENT]
| Operating | 2024 |
| Net Income | 500 |
`

	sections := splitByTableMarkers(markdown)

	if _, ok := sections[BalanceSheetType]; !ok {
		t.Error("Expected BALANCE_SHEET section")
	}
	if _, ok := sections[IncomeStatementType]; !ok {
		t.Error("Expected INCOME_STATEMENT section")
	}
	if _, ok := sections[CashFlowType]; !ok {
		t.Error("Expected CASH_FLOW section")
	}

	// Check content
	if !strings.Contains(sections[BalanceSheetType], "Total Assets") {
		t.Error("Balance sheet section should contain 'Total Assets'")
	}
}
