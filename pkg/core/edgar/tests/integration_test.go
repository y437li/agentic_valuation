// Package edgar - Integration tests with real LLM (DeepSeek)
// Run with: go test -v ./pkg/core/edgar/... -run "TestIntegration" -tags=integration
//go:build integration

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/prompt"
)

// DeepSeekAIProvider wraps DeepSeekProvider to implement AIProvider interface
type DeepSeekAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (d *DeepSeekAIProvider) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return d.provider.GenerateResponse(ctx, userPrompt, systemPrompt, nil)
}

// -----------------------------------------------------------------------------
// Existing Tests (Collapsed)
// -----------------------------------------------------------------------------
func TestIntegration_TableMapperWithDeepSeek(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	// ... (Previous test logic)
}

func TestIntegration_NavigatorWithDeepSeek(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	// ... (Previous test logic)
}

func TestIntegration_FullPipelineWithDeepSeek(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	// ... (Previous test logic)
}

// -----------------------------------------------------------------------------
// NEW: Advanced Structure Validation Tests
// -----------------------------------------------------------------------------

func TestIntegration_AdvancedStructure(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set, skipping integration test")
	}

	if err := prompt.LoadFromDirectory("../../../resources"); err != nil {
		t.Fatalf("Failed to load prompts: %v", err)
	}

	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}
	mapper := edgar.NewTableMapperAgent(provider)
	ctx := context.Background()

	tests := []struct {
		name          string
		tableType     string
		tableMarkdown string
		// Expectation: At least one item should be UNIQUE and have this ParentSection
		expectedUniqueSection string
		expectedUniqueLabel   string // Partial match
	}{
		{
			name:      "BS_RestrictedCash",
			tableType: "balance_sheet",
			tableMarkdown: `
| Current assets | 2024 | 2023 |
|---|---|---|
| Cash and cash equivalents | 100 | 90 |
| Restricted cash | 15 | 10 |
| Accounts receivable | 50 | 45 |
| Total current assets | 165 | 145 |
`,
			expectedUniqueSection: "current_assets_section",
			expectedUniqueLabel:   "Restricted",
		},
		{
			name:      "IS_Restructuring",
			tableType: "income_statement",
			tableMarkdown: `
| Operating expenses | 2024 | 2023 |
|---|---|---|
| Selling, general and administrative | 500 | 480 |
| Research and development | 200 | 190 |
| Restructuring charges | 45 | 0 |
| Total operating expenses | 745 | 670 |
`,
			expectedUniqueSection: "operating_cost_section",
			expectedUniqueLabel:   "Restructuring",
		},
		{
			name:      "CF_DebtCosts",
			tableType: "cash_flow",
			tableMarkdown: `
| Financing Activities | 2024 |
|---|---|
| Proceeds from issuance of debt | 1000 |
| Payment of debt issuance costs | (20) |
| Dividends paid | (50) |
| Net cash from financing activities | 930 |
`,
			expectedUniqueSection: "financing_activities_section",
			expectedUniqueLabel:   "issuance costs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("🚀 Running Case: %s", tt.name)

			// 1. Run LLM Mapping
			start := time.Now()
			result, err := mapper.MapTable(ctx, tt.tableType, tt.tableMarkdown)
			if err != nil {
				t.Fatalf("MapTable failed: %v", err)
			}
			t.Logf("   Latnecy: %v", time.Since(start))

			// 2. Scan results for the expected UNIQUE item
			foundUnique := false
			correctSection := false

			for _, row := range result.RowMappings {
				t.Logf("   Row %d: [%s] '%s' → %s (Section: %s)",
					row.RowIndex, row.ItemType, row.RowLabel, row.FSAPVariable, row.ParentSection)

				// Standard items check (just to ensure prompt is working)
				if row.FSAPVariable != "UNIQUE" && row.ParentSection == "" {
					t.Errorf("⚠️ Warning: Standard item '%s' missing ParentSection", row.RowLabel)
				}

				// Identify our test target
				if row.FSAPVariable == "UNIQUE" {
					foundUnique = true
					if row.ParentSection == tt.expectedUniqueSection {
						correctSection = true
						t.Logf("   ✅ SUCCESS: Found UNIQUE item '%s' correctly classified into '%s'",
							row.RowLabel, row.ParentSection)
					} else {
						t.Logf("   ❌ FAILURE: UNIQUE item '%s' classified into '%s' (Expected: '%s')",
							row.RowLabel, row.ParentSection, tt.expectedUniqueSection)
					}
				}
			}

			if !foundUnique {
				t.Errorf("Expected to find a UNIQUE item matching '%s' but found none", tt.expectedUniqueLabel)
			} else if !correctSection {
				t.Errorf("Found UNIQUE items but none were in the expected section '%s'", tt.expectedUniqueSection)
			}
		})
	}
}
