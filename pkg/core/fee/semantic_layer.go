// Package fee - Semantic Layer for constrained LLM field mapping
package fee

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	// Added for parallel execution
	"agentic_valuation/pkg/core/edgar"
)

// =============================================================================
// SEMANTIC LAYER - Constrained LLM Field Mapping (Phase 4)
// =============================================================================

// LLMProvider interface for semantic extraction
type LLMProvider interface {
	Query(ctx context.Context, prompt string) (string, error)
}

// SemanticExtractor uses LLM for field mapping but with constrained choices
type SemanticExtractor struct {
	provider LLMProvider
}

// NewSemanticExtractor creates a new semantic extractor
func NewSemanticExtractor(provider LLMProvider) *SemanticExtractor {
	return &SemanticExtractor{provider: provider}
}

// FieldMappingRequest contains candidates for LLM to choose from
type FieldMappingRequest struct {
	FSAPVariable string             `json:"fsap_variable"` // e.g., "cash_and_equivalents"
	Candidates   []MappingCandidate `json:"candidates"`    // Possible row matches
	TargetYear   int                `json:"target_year"`   // Which year's data to use
}

// MappingCandidate represents a possible row match
type MappingCandidate struct {
	RowIndex   int             `json:"row_index"`
	RowLabel   string          `json:"row_label"`  // e.g., "Cash and cash equivalents"
	Values     map[int]float64 `json:"values"`     // year -> value
	Confidence float64         `json:"confidence"` // From deterministic match
}

// FieldMappingResponse is the LLM's choice
type FieldMappingResponse struct {
	FSAPVariable   string  `json:"fsap_variable"`
	SelectedRow    int     `json:"selected_row"`    // Index of chosen candidate
	ConfidenceNote string  `json:"confidence_note"` // LLM's reasoning
	Value          float64 `json:"value"`
	Year           int     `json:"year"`
}

// MapFieldsConstrained uses LLM to map ambiguous fields from candidates
func (se *SemanticExtractor) MapFieldsConstrained(
	ctx context.Context,
	table *ParsedTable,
	targetYear int,
	unmappedVars []string,
) (map[string]*FieldMappingResponse, error) {
	if se.provider == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}

	// Build the prompt with constrained choices
	prompt := se.buildConstrainedPrompt(table, targetYear, unmappedVars)

	// Call LLM
	response, err := se.provider.Query(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM query failed: %w", err)
	}

	// Parse response
	return se.parseResponse(response)
}

// buildConstrainedPrompt creates a prompt that limits LLM choices
func (se *SemanticExtractor) buildConstrainedPrompt(
	table *ParsedTable,
	targetYear int,
	unmappedVars []string,
) string {
	var sb strings.Builder

	sb.WriteString(`You are a financial analyst mapping line items to standardized FSAP variables.

I have extracted a financial table with the following row labels. For each FSAP variable I need,
tell me which row (by index) best matches it, or return -1 if no good match exists.

## Table Info
`)
	sb.WriteString(fmt.Sprintf("Type: %s\n", table.Type))
	sb.WriteString(fmt.Sprintf("Title: %s\n", table.Title))
	sb.WriteString(fmt.Sprintf("Target Year: %d\n", targetYear))

	// List available columns
	sb.WriteString("\n## Available Columns (Years)\n")
	for _, col := range table.Columns {
		if col.Year > 0 {
			marker := ""
			if col.Year == targetYear {
				marker = " [TARGET]"
			}
			sb.WriteString(fmt.Sprintf("- Column %d: %s (Year %d)%s\n", col.Index, col.Label, col.Year, marker))
		}
	}

	// List rows with their values
	sb.WriteString("\n## Rows (choose from these)\n")
	sb.WriteString("Index | Label | Values\n")
	sb.WriteString("------|-------|--------\n")
	for i, row := range table.Rows {
		if row.IsHeader {
			continue // Skip header rows
		}
		values := []string{}
		for _, v := range row.Values {
			if v.Value != nil {
				values = append(values, fmt.Sprintf("%.0f", *v.Value))
			} else {
				values = append(values, "-")
			}
		}
		sb.WriteString(fmt.Sprintf("%d | %s | %s\n", i, row.Label, strings.Join(values, ", ")))
	}

	// List variables to map
	sb.WriteString("\n## Variables to Map\n")
	for _, v := range unmappedVars {
		sb.WriteString(fmt.Sprintf("- %s\n", v))
	}

	sb.WriteString(`

## Output Format (JSON only, no markdown)
{
  "mappings": [
    {"fsap_variable": "variable_name", "selected_row": 0, "confidence_note": "exact match", "year": 2024, "value": 12345},
    {"fsap_variable": "variable_name", "selected_row": -1, "confidence_note": "no match found", "year": 0, "value": 0}
  ]
}

IMPORTANT:
- Only choose from the rows listed above
- Use the value from the TARGET YEAR column
- If multiple rows could match, choose the most specific one
- Return -1 for selected_row if no good match exists
`)

	return sb.String()
}

// parseResponse extracts structured mappings from LLM response
func (se *SemanticExtractor) parseResponse(response string) (map[string]*FieldMappingResponse, error) {
	// Extract JSON from response (may have markdown wrapper)
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}
	jsonStr := response[jsonStart : jsonEnd+1]

	var parsed struct {
		Mappings []FieldMappingResponse `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	result := make(map[string]*FieldMappingResponse)
	for i := range parsed.Mappings {
		m := &parsed.Mappings[i]
		if m.SelectedRow >= 0 {
			result[m.FSAPVariable] = m
		}
	}

	return result, nil
}

// =============================================================================
// YEAR COLUMN SELECTOR - Choose correct column for target year
// =============================================================================

// ColumnSelector helps select the right column for a target year
type ColumnSelector struct{}

// SelectColumn finds the best column for a target year
func (cs *ColumnSelector) SelectColumn(columns []ColumnHeader, targetYear int) *ColumnHeader {
	// Exact match first
	for i := range columns {
		if columns[i].Year == targetYear {
			return &columns[i]
		}
	}

	// If no exact match, return latest
	return GetLatestYearColumn(columns)
}

// =============================================================================
// EXTRACTION ORCHESTRATOR - Coordinates deterministic + semantic extraction
// =============================================================================

// ExtractionOrchestrator combines deterministic and LLM-based extraction
type ExtractionOrchestrator struct {
	docParser     *DocumentParser
	fsapMapper    *FSAPMapper
	semantic      *SemanticExtractor
	strategyAgent *edgar.StrategyAgent
	capitalAgent  *edgar.CapitalAllocationAgent
	segmentAgent  *edgar.SegmentAgent
	riskAgent     *edgar.RiskAgent
	aiProvider    edgar.AIProvider // Store for orchestrator use
	// v2.0 Agents - LLM Navigator + Go Extractor pattern
	navigatorAgent   *edgar.NavigatorAgent
	tableMapperAgent *edgar.TableMapperAgent
	goExtractor      *edgar.GoExtractor
}

// NewExtractionOrchestrator creates a new orchestrator
func NewExtractionOrchestrator(provider LLMProvider, aiProvider edgar.AIProvider) *ExtractionOrchestrator {
	return &ExtractionOrchestrator{
		docParser:     NewDocumentParser(),
		fsapMapper:    NewFSAPMapper(),
		semantic:      NewSemanticExtractor(provider),
		strategyAgent: edgar.NewStrategyAgent(aiProvider),
		capitalAgent:  edgar.NewCapitalAllocationAgent(aiProvider),
		segmentAgent:  edgar.NewSegmentAgent(aiProvider),
		riskAgent:     edgar.NewRiskAgent(aiProvider),
		aiProvider:    aiProvider,
		// v2.0 Agents
		navigatorAgent:   edgar.NewNavigatorAgent(aiProvider),
		tableMapperAgent: edgar.NewTableMapperAgent(aiProvider),
		goExtractor:      edgar.NewGoExtractor(),
	}
}

// ExtractToFSAP performs full extraction from HTML to FSAP format
func (eo *ExtractionOrchestrator) ExtractToFSAP(
	ctx context.Context,
	html string,
	metadata DocumentMetadata,
	targetYear int,
) (*edgar.FSAPDataResponse, error) {
	// Step 1: Parse document structure
	docIndex, err := eo.docParser.ParseDocument(html, metadata)
	if err != nil {
		return nil, fmt.Errorf("document parsing failed: %w", err)
	}

	// Step 2: If no target year specified, use latest
	if targetYear == 0 && len(docIndex.AvailableYears) > 0 {
		targetYear = docIndex.AvailableYears[0] // Already sorted descending
	}

	// Step 3: Find the main financial tables
	var bsTable, isTable, cfTable *ParsedTable
	for i := range docIndex.Tables {
		t := &docIndex.Tables[i]
		// Prefer consolidated tables
		switch t.Type {
		case TableTypeBalanceSheet:
			if bsTable == nil || (t.IsConsolidated && !bsTable.IsConsolidated) {
				bsTable = t
			}
		case TableTypeIncomeStatement:
			if isTable == nil || (t.IsConsolidated && !isTable.IsConsolidated) {
				isTable = t
			}
		case TableTypeCashFlow:
			if cfTable == nil || (t.IsConsolidated && !cfTable.IsConsolidated) {
				cfTable = t
			}
		}
	}

	// Step 4: Build FSAP response
	resp := &edgar.FSAPDataResponse{
		Company:        metadata.CompanyName,
		CIK:            metadata.CIK,
		FiscalYear:     targetYear,
		FiscalPeriod:   "FY",
		SourceDocument: fmt.Sprintf("10-K Accession %s", metadata.AccessionNumber),
	}

	// Step 5: Extract Balance Sheet values
	if bsTable != nil {
		eo.extractBalanceSheet(bsTable, targetYear, resp)
	}

	// Step 6: Extract Income Statement values
	if isTable != nil {
		eo.extractIncomeStatement(isTable, targetYear, resp)
	}

	// Step 7: Extract Cash Flow values
	if cfTable != nil {
		eo.extractCashFlow(cfTable, targetYear, resp)
	}

	// Step 8: Count mapped variables
	resp.Metadata.VariablesMapped = countMappedValues(resp)

	return resp, nil
}

// extractBalanceSheet extracts balance sheet values
func (eo *ExtractionOrchestrator) extractBalanceSheet(table *ParsedTable, targetYear int, resp *edgar.FSAPDataResponse) {
	colSel := &ColumnSelector{}
	targetCol := colSel.SelectColumn(table.Columns, targetYear)
	if targetCol == nil {
		return
	}

	for _, row := range table.Rows {
		if row.IsHeader {
			continue
		}

		// Try deterministic mapping
		candidates := eo.fsapMapper.MapRowToFSAP(row.Label, TableTypeBalanceSheet)
		if len(candidates) == 0 {
			continue
		}

		// Get value for target year
		var value *float64
		for _, cv := range row.Values {
			if cv.ColumnIndex == targetCol.Index-1 && cv.Value != nil {
				value = cv.Value
				break
			}
		}

		if value == nil {
			continue
		}

		// Map to FSAP field based on best candidate
		bestCandidate := candidates[0]
		fsapValue := &edgar.FSAPValue{
			Value:       value,
			Label:       row.Label,
			SourcePath:  fmt.Sprintf("%s > Row %d", table.Title, row.Index),
			MappingType: "DETERMINISTIC",
			Provenance: &edgar.SourceTrace{
				SectionTitle: table.Title,
				TableID:      table.ID,
				RowLabel:     row.Label,
				ColumnLabel:  targetCol.Label,
				ExtractedBy:  "DETERMINISTIC",
			},
		}

		// Assign to appropriate FSAP field
		switch bestCandidate.FSAPVariable {
		case "cash_and_equivalents":
			resp.BalanceSheet.CurrentAssets.CashAndEquivalents = fsapValue
		case "short_term_investments":
			resp.BalanceSheet.CurrentAssets.ShortTermInvestments = fsapValue
		case "accounts_receivable_net":
			resp.BalanceSheet.CurrentAssets.AccountsReceivableNet = fsapValue
		case "inventories":
			resp.BalanceSheet.CurrentAssets.Inventories = fsapValue
		case "ppe_net":
			resp.BalanceSheet.NoncurrentAssets.PPENet = fsapValue
		case "goodwill":
			resp.BalanceSheet.NoncurrentAssets.Goodwill = fsapValue
		case "intangibles":
			resp.BalanceSheet.NoncurrentAssets.Intangibles = fsapValue
		case "accounts_payable":
			resp.BalanceSheet.CurrentLiabilities.AccountsPayable = fsapValue
		case "long_term_debt":
			resp.BalanceSheet.NoncurrentLiabilities.LongTermDebt = fsapValue
		case "common_stock":
			resp.BalanceSheet.Equity.CommonStockAPIC = fsapValue
		case "retained_earnings":
			resp.BalanceSheet.Equity.RetainedEarningsDeficit = fsapValue
		}
	}
}

// extractIncomeStatement extracts income statement values
func (eo *ExtractionOrchestrator) extractIncomeStatement(table *ParsedTable, targetYear int, resp *edgar.FSAPDataResponse) {
	colSel := &ColumnSelector{}
	targetCol := colSel.SelectColumn(table.Columns, targetYear)
	if targetCol == nil {
		return
	}

	for _, row := range table.Rows {
		if row.IsHeader {
			continue
		}

		candidates := eo.fsapMapper.MapRowToFSAP(row.Label, TableTypeIncomeStatement)
		if len(candidates) == 0 {
			continue
		}

		var value *float64
		for _, cv := range row.Values {
			if cv.ColumnIndex == targetCol.Index-1 && cv.Value != nil {
				value = cv.Value
				break
			}
		}

		if value == nil {
			continue
		}

		bestCandidate := candidates[0]
		fsapValue := &edgar.FSAPValue{
			Value:       value,
			Label:       row.Label,
			SourcePath:  fmt.Sprintf("%s > Row %d", table.Title, row.Index),
			MappingType: "DETERMINISTIC",
		}

		switch bestCandidate.FSAPVariable {
		case "revenues":
			resp.IncomeStatement.GrossProfitSection.Revenues = fsapValue
		case "cost_of_goods_sold":
			resp.IncomeStatement.GrossProfitSection.CostOfGoodsSold = fsapValue
		case "net_income":
			resp.IncomeStatement.NetIncomeSection.NetIncomeToCommon = fsapValue
		}
	}
}

// extractCashFlow extracts cash flow values
func (eo *ExtractionOrchestrator) extractCashFlow(table *ParsedTable, targetYear int, resp *edgar.FSAPDataResponse) {
	colSel := &ColumnSelector{}
	targetCol := colSel.SelectColumn(table.Columns, targetYear)
	if targetCol == nil {
		return
	}

	for _, row := range table.Rows {
		if row.IsHeader {
			continue
		}

		candidates := eo.fsapMapper.MapRowToFSAP(row.Label, TableTypeCashFlow)
		if len(candidates) == 0 {
			continue
		}

		var value *float64
		for _, cv := range row.Values {
			if cv.ColumnIndex == targetCol.Index-1 && cv.Value != nil {
				value = cv.Value
				break
			}
		}

		if value == nil {
			continue
		}

		bestCandidate := candidates[0]
		fsapValue := &edgar.FSAPValue{
			Value:       value,
			Label:       row.Label,
			SourcePath:  fmt.Sprintf("%s > Row %d", table.Title, row.Index),
			MappingType: "DETERMINISTIC",
		}

		switch bestCandidate.FSAPVariable {
		case "depreciation_amortization":
			resp.CashFlowStatement.DepreciationAmortization = fsapValue
		case "capex":
			resp.CashFlowStatement.Capex = fsapValue
		}
	}
}

// countMappedValues counts non-nil FSAP values
func countMappedValues(resp *edgar.FSAPDataResponse) int {
	count := 0
	// Simple heuristic - count non-nil balance sheet fields
	if resp.BalanceSheet.CurrentAssets.CashAndEquivalents != nil {
		count++
	}
	if resp.BalanceSheet.CurrentAssets.AccountsReceivableNet != nil {
		count++
	}
	if resp.BalanceSheet.CurrentAssets.Inventories != nil {
		count++
	}
	if resp.IncomeStatement.GrossProfitSection.Revenues != nil {
		count++
	}
	if resp.IncomeStatement.NetIncomeSection.NetIncomeToCommon != nil {
		count++
	}
	return count
}

// ComprehensiveReport combines FSAP data with qualitative insights
type ComprehensiveReport struct {
	Financials  *edgar.FSAPDataResponse    `json:"financials"`
	Qualitative *edgar.QualitativeInsights `json:"qualitative"`
}

// ExtractComprehensiveReport extraction financial and qualitative data in parallel
func (eo *ExtractionOrchestrator) ExtractComprehensiveReport(
	ctx context.Context,
	html string,
	metadata DocumentMetadata,
	targetYear int,
) (*ComprehensiveReport, error) {
	// 1. Extract Sections (Architecture Layer 2)
	// We use the Parser + LLMAnalyzer to discover and slice the document
	parser := edgar.NewParser()
	llmAnalyzer := edgar.NewLLMAnalyzer(eo.aiProvider)

	fullText, err := parser.ExtractWithLLMAgent(ctx, html, llmAnalyzer)
	if err != nil {
		return nil, fmt.Errorf("section extraction failed: %w", err)
	}

	// Helper to split the tagged text into a map
	sections := splitSections(fullText)

	// 2. Parallel Execution (The WaitGroup)
	var wg sync.WaitGroup
	var mu sync.Mutex

	report := &ComprehensiveReport{
		Qualitative: &edgar.QualitativeInsights{},
	}

	errors := make([]string, 0)

	// Track A: Financials (Existing deterministic/hybrid flow)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// We pass the original HTML to preserve the existing verified flow
		fsapResp, err := eo.ExtractToFSAP(ctx, html, metadata, targetYear)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errors = append(errors, fmt.Sprintf("FSAP Error: %v", err))
		} else {
			report.Financials = fsapResp
		}
	}()

	// Track B: Strategy (Input: MD&A)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if mda, ok := sections["MDA"]; ok {
			res, err := eo.strategyAgent.Analyze(ctx, mda)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Sprintf("Strategy Error: %v", err))
			} else {
				report.Qualitative.Strategy = *res
			}
		}
	}()

	// Track C: Capital Allocation (Input: MD&A)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Capital allocation often needs MD&A context
		if mda, ok := sections["MDA"]; ok {
			res, err := eo.capitalAgent.Analyze(ctx, mda)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Sprintf("Capital Error: %v", err))
			} else {
				report.Qualitative.CapitalAllocation = *res
			}
		}
	}()

	// Track D: Segments (Input: Business + MD&A)
	wg.Add(1)
	go func() {
		defer wg.Done()
		business := sections["BUSINESS"]
		mda := sections["MDA"]
		if business != "" || mda != "" {
			res, err := eo.segmentAgent.Analyze(ctx, business, mda)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Sprintf("Segment Error: %v", err))
			} else {
				report.Qualitative.Segments = *res
			}
		}
	}()

	// Track E: Risk (Input: Item 1A + 7A + 3 + 9A)
	wg.Add(1)
	go func() {
		defer wg.Done()

		var riskTextBuilder strings.Builder
		if rf := sections["RISK_FACTORS"]; rf != "" {
			riskTextBuilder.WriteString("\n=== ITEM 1A: RISK FACTORS ===\n")
			riskTextBuilder.WriteString(rf)
		}
		if mr := sections["MARKET_RISK"]; mr != "" {
			riskTextBuilder.WriteString("\n=== ITEM 7A: MARKET RISK ===\n")
			riskTextBuilder.WriteString(mr)
		}
		if lp := sections["LEGAL_PROCEEDINGS"]; lp != "" {
			riskTextBuilder.WriteString("\n=== ITEM 3: LEGAL PROCEEDINGS ===\n")
			riskTextBuilder.WriteString(lp)
		}
		if ctrl := sections["CONTROLS"]; ctrl != "" {
			riskTextBuilder.WriteString("\n=== ITEM 9A: CONTROLS ===\n")
			riskTextBuilder.WriteString(ctrl)
		}

		riskText := riskTextBuilder.String()
		if riskText != "" {
			res, err := eo.riskAgent.Analyze(ctx, riskText)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Sprintf("Risk Error: %v", err))
			} else {
				report.Qualitative.Risks = *res
			}
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		// Log errors but return partial result if possible, or error if critical
		// For now, if FSAP failed, it's critical.
		if report.Financials == nil {
			return nil, fmt.Errorf("critical failure in financial extraction: %v", strings.Join(errors, "; "))
		}
		// Qualitative errors are non-critical
	}

	return report, nil
}

// splitSections parses the output of ExtractWithLLMAgent into a map
func splitSections(text string) map[string]string {
	result := make(map[string]string)

	// Markers we look for
	// Markers we look for
	markers := []string{"[TABLE: BUSINESS]", "[TABLE: RISK_FACTORS]", "[TABLE: MARKET_RISK]", "[TABLE: LEGAL_PROCEEDINGS]", "[TABLE: CONTROLS]", "[TABLE: MDA]", "[TABLE: BALANCE_SHEET]", "[TABLE: INCOME_STATEMENT]", "[TABLE: CASH_FLOW]", "[TABLE: NOTES]"}

	// Simple split strategy: find marker, take text until next marker
	// We iterate through the text

	// Hacky but effective: replace known markers with unique split token
	tempText := text
	for _, m := range markers {
		// Remove "[TABLE: " and "]" to get key
		key := strings.TrimSuffix(strings.TrimPrefix(m, "[TABLE: "), "]")
		// Replace marker with special token
		tempText = strings.ReplaceAll(tempText, m, "|||SPLIT|||"+key+"|||CONTENT|||")
	}

	parts := strings.Split(tempText, "|||SPLIT|||")
	for _, part := range parts {
		if part == "" {
			continue
		}
		if idx := strings.Index(part, "|||CONTENT|||"); idx >= 0 {
			key := part[:idx]
			content := part[idx+len("|||CONTENT|||"):]
			result[key] = strings.TrimSpace(content)
		}
	}

	return result
}
