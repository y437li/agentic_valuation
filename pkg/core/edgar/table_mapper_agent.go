// Package edgar - Table Mapper Agent for Line Item → FSAP mapping
package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agentic_valuation/pkg/core/prompt"
)

// =============================================================================
// TABLE MAPPER AGENT - Maps table line items to FSAP variables
// =============================================================================

// YearColumn represents a detected year column in the table
type YearColumn struct {
	Year        int `json:"year"`
	ColumnIndex int `json:"column_index"`
}

// ItemType classifies the row as regular item, subtotal, or total
type ItemType string

const (
	ItemTypeItem     ItemType = "ITEM"     // Regular line item (e.g., "Cash and equivalents")
	ItemTypeSubtotal ItemType = "SUBTOTAL" // Section subtotal (e.g., "Total current assets")
	ItemTypeTotal    ItemType = "TOTAL"    // Statement total (e.g., "Total assets")
)

// RowMapping represents mapping from table row to FSAP variable
type RowMapping struct {
	RowIndex      int      `json:"row_index"`
	RowLabel      string   `json:"row_label"`
	FSAPVariable  string   `json:"fsap_variable"`
	Confidence    float64  `json:"confidence"`
	ItemType      ItemType `json:"item_type,omitempty"`      // ITEM, SUBTOTAL, TOTAL
	MarkdownLine  int      `json:"markdown_line,omitempty"`  // Line number in source markdown (for jump-to-source)
	ParentSection string   `json:"parent_section,omitempty"` // Classification from LLM (e.g., "Current Assets")
}

// LineItemMapping represents the complete mapping for a table
type LineItemMapping struct {
	TableType   string       `json:"table_type"` // "balance_sheet", "income_statement", "cash_flow"
	YearColumns []YearColumn `json:"year_columns"`
	RowMappings []RowMapping `json:"row_mappings"` // All mappings (backward compatible)
	// Separated for cross-validation
	Items     []RowMapping `json:"items,omitempty"`     // Regular line items only
	Subtotals []RowMapping `json:"subtotals,omitempty"` // Section subtotals
	Totals    []RowMapping `json:"totals,omitempty"`    // Statement totals (for validation)
}

// TableMapperAgent uses LLM to map table line items to FSAP variables
type TableMapperAgent struct {
	provider AIProvider
}

// NewTableMapperAgent creates a new table mapper agent
func NewTableMapperAgent(provider AIProvider) *TableMapperAgent {
	return &TableMapperAgent{provider: provider}
}

// MapTable analyzes a table and returns line item mappings
func (a *TableMapperAgent) MapTable(ctx context.Context, tableType string, tableMarkdown string) (*LineItemMapping, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	systemPrompt, userPrompt := a.buildMappingPrompt(tableType, tableMarkdown)

	response, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM query failed: %w", err)
	}

	return a.parseResponse(tableType, response)
}

// buildMappingPrompt creates the prompts for table mapping
// Tries to load from prompt library first, falls back to hardcoded if not found
func (a *TableMapperAgent) buildMappingPrompt(tableType string, tableMarkdown string) (string, string) {
	fsapVars := a.getFSAPVariables(tableType)

	// Try to load prompt from JSON file
	promptID := fmt.Sprintf("extraction.table_mapper.%s", tableType)
	if pt, err := prompt.Get().GetPrompt(promptID); err == nil {
		// Use prompt from library
		ctx := prompt.NewContext().
			Set("TableMarkdown", tableMarkdown)
		userPrompt, _ := prompt.RenderUserPrompt(pt, ctx)
		return pt.SystemPrompt, userPrompt
	}

	// Fallback to hardcoded prompt
	systemPrompt := "You are a financial data analyst. Map table line items to standard FSAP variables."

	userPrompt := fmt.Sprintf(`Map line items in this %s table to FSAP variables.

TABLE (with line numbers):
%s

FSAP VARIABLES TO MAP:
%s

Output JSON:
{
  "year_columns": [{"year": 2024, "column_index": 1}],
  "row_mappings": [
    {
      "row_index": 0,
      "row_label": "Cash and equivalents",
      "fsap_variable": "cash_and_equivalents",
      "confidence": 1.0,
      "item_type": "ITEM",
      "markdown_line": 45
    }
  ]
}

Rules:
- row_index is 0-based from first data row (not header)
- Only map rows that clearly match an FSAP variable
- Confidence: 1.0 = exact, 0.8-0.99 = likely, <0.8 = uncertain
- item_type: "ITEM" for regular line items, "SUBTOTAL" for section totals (e.g., "Total current assets"), "TOTAL" for statement totals (e.g., "Total assets")
- markdown_line: Provide the line number where this row appears in the source markdown (for jump-to-source)`, tableType, tableMarkdown, strings.Join(fsapVars, "\n"))

	return systemPrompt, userPrompt
}

// getFSAPVariables returns the list of FSAP variables for a table type
func (a *TableMapperAgent) getFSAPVariables(tableType string) []string {
	switch tableType {
	case "balance_sheet":
		return []string{
			"cash_and_equivalents - Cash and cash equivalents",
			"short_term_investments - Short-term investments, marketable securities",
			"accounts_receivable_net - Accounts receivable (net of allowance)",
			"inventories - Inventories",
			"prepaid_expenses - Prepaid expenses",
			"other_current_assets - Other current assets",
			"total_current_assets - Total current assets",
			"ppe_net - Property, plant and equipment (net)",
			"goodwill - Goodwill",
			"intangibles - Intangible assets",
			"other_noncurrent_assets - Other non-current assets",
			"total_assets - Total assets",
			"accounts_payable - Accounts payable",
			"accrued_expenses - Accrued expenses",
			"short_term_debt - Short-term debt, current portion of long-term debt",
			"other_current_liabilities - Other current liabilities",
			"total_current_liabilities - Total current liabilities",
			"long_term_debt - Long-term debt",
			"other_noncurrent_liabilities - Other non-current liabilities",
			"total_liabilities - Total liabilities",
			"common_stock - Common stock and APIC",
			"retained_earnings - Retained earnings (deficit)",
			"treasury_stock - Treasury stock",
			"aoci - Accumulated other comprehensive income (loss)",
			"noncontrolling_interests - Noncontrolling interests (minority interests)",
			"total_equity - Total stockholders' equity (including NCI if applicable)",
		}
	case "income_statement":
		return []string{
			"revenues - Total revenues, net sales",
			"cost_of_goods_sold - Cost of goods sold, cost of revenues",
			"gross_profit - Gross profit",
			"sga_expenses - Selling, general and administrative expenses",
			"rd_expenses - Research and development expenses",
			"operating_income - Operating income",
			"interest_expense - Interest expense",
			"interest_income - Interest income",
			"other_income_expense - Other income/expense",
			"income_before_tax - Income before income taxes",
			"income_tax_expense - Income tax expense (benefit)",
			"net_income - Net income",
			"eps_basic - Basic earnings per share",
			"eps_diluted - Diluted earnings per share",
		}
	case "cash_flow":
		return []string{
			"net_income - Net income",
			"depreciation_amortization - Depreciation and amortization",
			"stock_based_compensation - Stock-based compensation",
			"change_in_working_capital - Changes in working capital",
			"operating_cash_flow - Net cash from operating activities",
			"capex - Capital expenditures",
			"acquisitions - Acquisitions",
			"investing_cash_flow - Net cash from investing activities",
			"debt_issuance - Proceeds from debt",
			"debt_repayment - Repayment of debt",
			"stock_repurchase - Stock repurchases",
			"dividends - Dividends paid",
			"financing_cash_flow - Net cash from financing activities",
			"net_change_in_cash - Net change in cash",
		}
	default:
		return []string{}
	}
}

// parseResponse extracts LineItemMapping from LLM response
func (a *TableMapperAgent) parseResponse(tableType string, response string) (*LineItemMapping, error) {
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}
	jsonStr := response[jsonStart : jsonEnd+1]

	var result LineItemMapping
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	result.TableType = tableType

	// Separate mappings by ItemType for cross-validation
	for _, rm := range result.RowMappings {
		switch rm.ItemType {
		case ItemTypeSubtotal:
			result.Subtotals = append(result.Subtotals, rm)
		case ItemTypeTotal:
			result.Totals = append(result.Totals, rm)
		default:
			// Default to ITEM if not specified
			if rm.ItemType == "" {
				rm.ItemType = ItemTypeItem
			}
			result.Items = append(result.Items, rm)
		}
	}

	return &result, nil
}
