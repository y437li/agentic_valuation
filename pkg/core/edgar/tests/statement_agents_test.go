package tests

import (
	"agentic_valuation/pkg/core/edgar"
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
			units := edgar.DetectUnits(tt.markdown)
			if units.Scale != tt.wantScale {
				t.Errorf("Scale = %v, want %v", units.Scale, tt.wantScale)
			}
			if units.ScaleLabel != tt.wantLabel {
				t.Errorf("ScaleLabel = %v, want %v", units.ScaleLabel, tt.wantLabel)
			}
		})
	}
}

// NOTE: TestSplitByTableMarkers removed - splitByTableMarkers was part of v1.0 legacy code
// V2.0 architecture uses NavigatorAgent for statement location detection
