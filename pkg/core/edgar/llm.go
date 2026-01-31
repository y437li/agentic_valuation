package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AIProvider interface for LLM interaction
type AIProvider interface {
	Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
}

// LLMAnalyzer handles LLM-based analysis of filing text
type LLMAnalyzer struct {
	provider AIProvider
}

// NewLLMAnalyzer creates a new analyzer
func NewLLMAnalyzer(provider AIProvider) *LLMAnalyzer {
	return &LLMAnalyzer{provider: provider}
}

// System Prompt for Financial Analyst
const SystemPrompt = `You are an expert Financial Analyst and Auditor (CPA). 
Your task is to analyze excerpts from SEC 10-K filings (specifically the Notes to Financial Statements) to identify and extract line-item breakdowns that justify reclassifying generic aggregated items into specific FSAP variables.

You must strictly adhere to the following JSON schema for your output:
[
  {
    "fsap_variable": "string (e.g., other_current_assets_1)",
    "reclassification_type": "BREAKDOWN", 
    "value": number (in millions),
    "reasoning": "string (quote direct evidence)",
    "note_evidence": {
      "note_number": "string (e.g., Note 5)",
      "note_title": "string",
      "quote": "string"
    }
  }
]

Rules:
1. Only extract data that is explicitly stated in the text.
2. Value must be consistent with the context (millions vs thousands).
3. If no relevant data is found, return an empty array [].
`

// AnalyzeNotes for reclassification evidence
func (a *LLMAnalyzer) AnalyzeNotes(ctx context.Context, notesText string, unmappedVars []string) ([]Reclassification, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Truncate notes text if too long (simple approach for now)
	// In production, should use RAG or smart chunking
	maxLen := 15000
	if len(notesText) > maxLen {
		notesText = notesText[:maxLen] + "... [truncated]"
	}

	userPrompt := fmt.Sprintf(`
Target Variables to find breakdowns for: %s

Notes Text Excerpt:
%s

Identify any specific line items that should be mapped to "other_*" variables or provide breakdown for generic items.
Return ONLY valid JSON.
`, strings.Join(unmappedVars, ", "), notesText)

	resp, err := a.provider.Generate(ctx, SystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	// Basic cleanup for markdown code blocks
	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	var reclassifications []Reclassification
	if err := json.Unmarshal([]byte(cleanJson), &reclassifications); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %v\nResponse: %s", err, resp)
	}

	return reclassifications, nil
}

// AnalyzeWithFacts sends both Notes text and parsed XBRL facts to LLM for comprehensive data alignment
// Only non-null facts are included to minimize token usage
func (a *LLMAnalyzer) AnalyzeWithFacts(ctx context.Context, notesText string, facts []XBRLFact, unmappedVars []string) ([]Reclassification, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Build compact facts summary (only non-null values)
	var factsSummary strings.Builder
	factsSummary.WriteString("Parsed XBRL Data (non-null only):\n")
	factCount := 0
	for _, fact := range facts {
		if fact.NumericVal != 0 { // Only include non-zero values (use NumericVal which is float64)
			factsSummary.WriteString(fmt.Sprintf("- %s: %.2f (%s)\n", fact.Tag, fact.NumericVal, fact.ContextRef))
			factCount++
			if factCount >= 100 { // Limit to 100 facts to control token usage
				factsSummary.WriteString("... [more facts truncated]\n")
				break
			}
		}
	}

	// Truncate notes text
	maxLen := 10000 // Smaller since we're also sending facts
	if len(notesText) > maxLen {
		notesText = notesText[:maxLen] + "... [truncated]"
	}

	// Enhanced system prompt for full data alignment
	enhancedSystem := `You are an expert Financial Analyst and Auditor (CPA).
Your task is to analyze SEC 10-K filings and map XBRL data to FSAP (Financial Statement Analysis Platform) variables.

You have access to:
1. Parsed XBRL facts from the filing (structured data)
2. Notes to Financial Statements (for detailed breakdowns)

Your job is to:
1. Identify which XBRL tags map to which FSAP variables
2. Find breakdowns for "other_*" placeholder variables
3. Identify any reclassifications needed based on Notes disclosures

Return a JSON array following this schema:
[
  {
    "fsap_variable": "string (e.g., other_current_assets_1)",
    "xbrl_tag": "string (the XBRL tag to use)",
    "value": number (in millions USD),
    "label": "string (human-readable name)",
    "reclassification_type": "DIRECT | BREAKDOWN | COMBINED",
    "reasoning": "string (brief justification)",
    "note_evidence": {
      "note_number": "Note X",
      "quote": "relevant quote if applicable"
    }
  }
]

Rules:
1. Only use data explicitly present in the facts or notes
2. Values in millions USD (divide raw values by 1,000,000 if needed)
3. If no additional mappings found, return []
`

	userPrompt := fmt.Sprintf(`
Target FSAP Variables needing data: %s

%s

Notes Text Excerpt:
%s

Analyze the above and identify any additional line items that should be mapped to the target variables.
Return ONLY valid JSON array.
`, strings.Join(unmappedVars, ", "), factsSummary.String(), notesText)

	resp, err := a.provider.Generate(ctx, enhancedSystem, userPrompt)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	var reclassifications []Reclassification
	if err := json.Unmarshal([]byte(cleanJson), &reclassifications); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %v\nResponse: %s", err, resp)
	}

	return reclassifications, nil
}

// ParallelFullTableExtraction uses the v2.0 architecture to extract each statement.
// Uses Navigator → Mapper → GoExtractor pipeline for improved accuracy.
func (a *LLMAnalyzer) ParallelFullTableExtraction(ctx context.Context, item8Markdown string, meta *FilingMetadata) (*FSAPDataResponse, error) {
	v2 := NewV2Extractor(a.provider)
	return v2.Extract(ctx, item8Markdown, meta)
}

// PopulateSourcePositions searches for Label in markdown and fills MarkdownLine in Provenance.
// This is deterministic - no LLM hallucination possible.
// Uses the Label field (original 10-K item name) for search.
func PopulateSourcePositions(resp *FSAPDataResponse, markdown string) {
	if len(markdown) == 0 || resp == nil {
		return
	}

	// Helper to find line number for a label
	findLine := func(label string) int {
		return FindLineNumber(markdown, label)
	}

	// Helper to populate provenance for a single FSAPValue using Label
	populateValue := func(v *FSAPValue) {
		if v == nil || v.Label == "" {
			return
		}
		line := findLine(v.Label)
		if line > 0 {
			if v.Provenance == nil {
				v.Provenance = &SourceTrace{}
			}
			v.Provenance.MarkdownLine = line
			v.Provenance.RowLabel = v.Label // Keep in sync
		}
	}

	// Process Balance Sheet fields (CurrentAssets is embedded struct, not pointer)
	bs := &resp.BalanceSheet
	populateValue(bs.CurrentAssets.CashAndEquivalents)
	populateValue(bs.CurrentAssets.ShortTermInvestments)
	populateValue(bs.CurrentAssets.AccountsReceivableNet)
	populateValue(bs.CurrentAssets.Inventories)
	populateValue(bs.CurrentAssets.OtherCurrentAssets)

	populateValue(bs.NoncurrentAssets.LongTermInvestments)
	populateValue(bs.NoncurrentAssets.PPENet)
	populateValue(bs.NoncurrentAssets.Goodwill)
	populateValue(bs.NoncurrentAssets.Intangibles)

	populateValue(bs.CurrentLiabilities.AccountsPayable)
	populateValue(bs.CurrentLiabilities.AccruedLiabilities)
	populateValue(bs.CurrentLiabilities.NotesPayableShortTermDebt)

	populateValue(bs.NoncurrentLiabilities.LongTermDebt)
	populateValue(bs.NoncurrentLiabilities.DeferredTaxLiabilities)

	populateValue(bs.Equity.CommonStockAPIC)
	populateValue(bs.Equity.RetainedEarningsDeficit)
	populateValue(bs.Equity.TreasuryStock)

	// Process Income Statement sections
	is := &resp.IncomeStatement

	// Process Income Statement sections
	if is.GrossProfitSection != nil {
		populateValue(is.GrossProfitSection.Revenues)
		populateValue(is.GrossProfitSection.CostOfGoodsSold)
		populateValue(is.GrossProfitSection.GrossProfit)
	}
	if is.OperatingCostSection != nil {
		populateValue(is.OperatingCostSection.SGAExpenses)
		populateValue(is.OperatingCostSection.RDExpenses)
		populateValue(is.OperatingCostSection.OperatingIncome)
	}

	// Process Cash Flow Statement
	cf := &resp.CashFlowStatement
	populateValue(cf.NetIncomeStart)
	populateValue(cf.DepreciationAmortization)
	populateValue(cf.Capex)
	populateValue(cf.DebtIssuanceRetirementNet)
	populateValue(cf.ShareRepurchases)
	populateValue(cf.Dividends)
}

// countLLMVariables counts non-null values in the FSAPDataResponse
func countLLMVariables(resp *FSAPDataResponse) (mapped, unmapped int) {
	// Helper to count non-nil values in a map
	countSection := func(data interface{}) (m, u int) {
		jsonBytes, _ := json.Marshal(data)
		var rawMap map[string]json.RawMessage
		json.Unmarshal(jsonBytes, &rawMap)

		for key, val := range rawMap {
			if key == "_reported_for_validation" || key == "calculated_total" {
				continue
			}
			// Check if it's a nested object with "value" field
			var item struct {
				Value *float64 `json:"value"`
			}
			if err := json.Unmarshal(val, &item); err == nil {
				if item.Value != nil && *item.Value != 0 {
					m++
				} else {
					u++
				}
			} else {
				// Might be a nested section, recurse
				var nested map[string]json.RawMessage
				if json.Unmarshal(val, &nested) == nil {
					for _, nv := range nested {
						if json.Unmarshal(nv, &item) == nil {
							if item.Value != nil && *item.Value != 0 {
								m++
							} else {
								u++
							}
						}
					}
				}
			}
		}
		return
	}

	// Count all sections
	m1, u1 := countSection(resp.BalanceSheet)
	m2, u2 := countSection(resp.IncomeStatement)
	m3, u3 := countSection(resp.CashFlowStatement)
	m4, u4 := countSection(resp.SupplementalData)

	return m1 + m2 + m3 + m4, u1 + u2 + u3 + u4
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TOCAgentResult contains LLM-identified statement locations
type TOCAgentResult struct {
	Business         *TOCItem `json:"business"`          // Item 1
	RiskFactors      *TOCItem `json:"risk_factors"`      // Item 1A
	MarketRisk       *TOCItem `json:"market_risk"`       // Item 7A
	LegalProceedings *TOCItem `json:"legal_proceedings"` // Item 3
	Controls         *TOCItem `json:"controls"`          // Item 9A
	MDA              *TOCItem `json:"mda"`               // Item 7
	BalanceSheet     *TOCItem `json:"balance_sheet"`
	IncomeStatement  *TOCItem `json:"income_statement"`
	CashFlow         *TOCItem `json:"cash_flow"`
	Notes            *TOCItem `json:"notes"`
}

// TOCItem represents a single TOC entry identified by LLM
type TOCItem struct {
	Title  string `json:"title"`
	Page   int    `json:"page,omitempty"`
	Anchor string `json:"anchor,omitempty"`
}

// AnalyzeTOC uses LLM to identify financial statement entries in TOC
func (a *LLMAnalyzer) AnalyzeTOC(ctx context.Context, tocMarkdown string) (*TOCAgentResult, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Limit TOC size for LLM
	maxLen := 20000
	if len(tocMarkdown) > maxLen {
		tocMarkdown = tocMarkdown[:maxLen]
	}

	systemPrompt := `You are a Senior Financial Analyst. 
Your job is to identify the specific sections in a 10-K filing that are required to perform specific analysis tasks.
Do not rely solely on standard Item numbers (like "Item 1A"), as they may vary. Look for the content description.`

	userPrompt := fmt.Sprintf(`Given the Table of Contents (below), identify the start page and title for the sections needed for these 4 Analysis Tasks:

TASK 1: Structural Risk Analysis
- Need: The primary section listing risk factors and uncertainties.
- Target: Often "Item 1A. Risk Factors", but could be "Principal Risks".

TASK 2: Quantitative Market Risk (Hidden Risks)
- Need: Interest rate sensitivity, FX exposure.
- Target: "Item 7A. Quantitative and Qualitative Disclosures about Market Risk".

TASK 3: Legal & Regulatory Risks
- Need: Material pending litigation.
- Target: "Item 3. Legal Proceedings".

TASK 4: Internal Control Risks
- Need: Weaknesses in financial reporting controls.
- Target: "Item 9A. Controls and Procedures".

TASK 5: Strategic Analysis & Management Sentiment
- Need: Management's narrative on performance, outlook, and liquidity.
- Target: "Item 7. Management's Discussion and Analysis" (MD&A).

TASK 6: Business & Segment Modeling
- Need: Description of business operations and reporting segments.
- Target: "Item 1. Business".

TASK 7: Financial Statement Extraction
- Need: Balance Sheet, Income Statement, Cash Flows, and Notes.
- Target: "Item 8. Financial Statements".

For each mapped section, extract the exact Title, Page Number, and Anchor Link (if available).

TOC:
%s

Return JSON (map task to section):
{
  "business": { "title": "...", "page": ... },       // For Task 6
  "risk_factors": { "title": "...", "page": ... },   // For Task 1
  "market_risk": { "title": "...", "page": ... },    // For Task 2
  "legal_proceedings": { "title": "...", "page": ... }, // For Task 3
  "controls": { "title": "...", "page": ... },       // For Task 4
  "mda": { "title": "...", "page": ... },            // For Task 5
  "balance_sheet": { "title": "...", "page": ... },  // For Task 7
  "income_statement": { "title": "...", "page": ... },
  "cash_flow": { "title": "...", "page": ... },
  "notes": { "title": "...", "page": ... }
}`, tocMarkdown)

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	// Find JSON object
	start := strings.Index(cleanJson, "{")
	end := strings.LastIndex(cleanJson, "}")
	if start >= 0 && end > start {
		cleanJson = cleanJson[start : end+1]
	}

	var result TOCAgentResult
	if err := json.Unmarshal([]byte(cleanJson), &result); err != nil {
		return nil, fmt.Errorf("failed to parse TOC LLM response: %v", err)
	}

	return &result, nil
}
