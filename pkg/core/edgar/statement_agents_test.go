package edgar

import (
	"strings"
	"testing"
)

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
