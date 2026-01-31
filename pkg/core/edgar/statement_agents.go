package edgar

import (
	"agentic_valuation/pkg/core/prompt"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DetectedUnits holds scale information extracted from financial table headers
type DetectedUnits struct {
	Scale      float64 // 1000, 1000000, or 1
	ScaleLabel string  // "thousands", "millions", "dollars"
	ShareScale float64 // Separate scale for share counts (may differ from dollar amounts)
	Currency   string  // "USD", "EUR", etc.
}

// DetectUnits scans markdown for unit declarations in table headers.
// This replaces hardcoded "millions" assumptions in prompts.
// Returns detected scale factor and label.
func DetectUnits(markdown string) *DetectedUnits {
	result := &DetectedUnits{
		Scale:      1000000, // Default to millions if not detected
		ScaleLabel: "millions",
		ShareScale: 1000000,
		Currency:   "USD",
	}

	// Unit detection patterns (priority order - first match wins)
	patterns := []struct {
		regex      string
		scale      float64
		scaleLabel string
	}{
		// Explicit "in thousands" patterns
		{`(?i)in\s+thousands`, 1000, "thousands"},
		{`(?i)\(\s*in\s+thousands\s*\)`, 1000, "thousands"},
		{`(?i)\$\s*000s?`, 1000, "thousands"},
		{`(?i)amounts\s+in\s+thousands`, 1000, "thousands"},
		{`(?i)thousands\s+of\s+dollars`, 1000, "thousands"},

		// Explicit "in millions" patterns
		{`(?i)in\s+millions`, 1000000, "millions"},
		{`(?i)\(\s*in\s+millions\s*\)`, 1000000, "millions"},
		{`(?i)\$\s*MM`, 1000000, "millions"},
		{`(?i)\$M`, 1000000, "millions"},
		{`(?i)amounts\s+in\s+millions`, 1000000, "millions"},
		{`(?i)millions\s+of\s+dollars`, 1000000, "millions"},

		// Explicit "in billions" patterns
		{`(?i)in\s+billions`, 1000000000, "billions"},
		{`(?i)\(\s*in\s+billions\s*\)`, 1000000000, "billions"},

		// Raw dollars (no scale)
		{`(?i)amounts\s+in\s+dollars`, 1, "dollars"},
	}

	// Scan first 5000 chars (where unit declarations typically appear)
	scanRegion := markdown
	if len(scanRegion) > 5000 {
		scanRegion = scanRegion[:5000]
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.regex)
		if re.MatchString(scanRegion) {
			result.Scale = p.scale
			result.ScaleLabel = p.scaleLabel
			break
		}
	}

	// Check for separate share scale (e.g., "shares in thousands")
	sharePatterns := []struct {
		regex string
		scale float64
	}{
		{`(?i)shares?\s+in\s+thousands`, 1000},
		{`(?i)shares?\s+in\s+millions`, 1000000},
		{`(?i)except\s+(?:per\s+)?share`, 1}, // Indicates share data is in single units
	}

	for _, p := range sharePatterns {
		re := regexp.MustCompile(p.regex)
		if re.MatchString(scanRegion) {
			result.ShareScale = p.scale
			break
		}
	}

	return result
}

// StatementType represents different financial statement types
type StatementType string

const (
	BalanceSheetType    StatementType = "BALANCE_SHEET"
	IncomeStatementType StatementType = "INCOME_STATEMENT"
	CashFlowType        StatementType = "CASH_FLOW"
	SupplementalType    StatementType = "SUPPLEMENTAL"
	BusinessType        StatementType = "BUSINESS"
	RiskFactorsType     StatementType = "RISK_FACTORS"
	MDAType             StatementType = "MDA"
	NotesType           StatementType = "NOTES"
)

// StatementAgent represents a specialized extraction agent for a specific statement type
type StatementAgent struct {
	Name          string
	StatementType StatementType
	// SystemPrompt is generated dynamically based on detected units
}

// StatementAgentRegistry holds all specialized agents
var StatementAgentRegistry = map[StatementType]*StatementAgent{
	BalanceSheetType: {
		Name:          "BalanceSheetAgent",
		StatementType: BalanceSheetType,
	},
	IncomeStatementType: {
		Name:          "IncomeStatementAgent",
		StatementType: IncomeStatementType,
	},
	CashFlowType: {
		Name:          "CashFlowAgent",
		StatementType: CashFlowType,
	},
	SupplementalType: {
		Name:          "SupplementalAgent",
		StatementType: SupplementalType,
	},
}

// partialResult holds extraction result from one agent
type partialResult struct {
	StatementType StatementType
	Data          json.RawMessage
	Error         error
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// ParallelExtract runs specialized agents concurrently for each statement type.
// This provides:
// 1. Smaller context windows (lower token usage)
// 2. Parallel execution (faster total time)
// 3. Easier debugging of individual statement issues
func ParallelExtract(ctx context.Context, fullMarkdown string, provider AIProvider, meta *FilingMetadata) (*FSAPDataResponse, error) {
	if provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Step 1: Detect units from markdown headers
	units := DetectUnits(fullMarkdown)

	// Step 2: Split markdown by [TABLE: TYPE] markers
	sections := splitByTableMarkers(fullMarkdown)

	// === DEBUG: Log all sections found ===
	fmt.Println("\n========== PIPELINE DEBUG ==========")
	fmt.Printf("[STAGE 1] splitByTableMarkers found %d sections:\n", len(sections))
	for st, content := range sections {
		fmt.Printf("  - %s: %d chars (first 100: %s...)\n", st, len(content), truncateForLog(content, 100))
	}
	fmt.Println("=====================================")

	// Step 3: Run agents in parallel
	var wg sync.WaitGroup
	resultsChan := make(chan *partialResult, 4)

	for sectionType, sectionContent := range sections {
		if len(sectionContent) < 100 {
			fmt.Printf("[STAGE 2] SKIPPING %s (only %d chars, below 100 threshold)\n", sectionType, len(sectionContent))
			continue // Skip truly empty or trivial sections (lowered from 500)
		}

		wg.Add(1)
		go func(st StatementType, content string) {
			defer wg.Done()

			result := &partialResult{StatementType: st}

			// Generate statement-specific prompt
			prompt := generateStatementPrompt(st, units)

			fmt.Printf("[STAGE 2] Starting LLM extraction for %s (content length: %d chars)\n", st, len(content))

			// Extract using LLM
			data, err := extractStatement(ctx, provider, prompt, content, meta)
			if err != nil {
				fmt.Printf("[STAGE 2] ERROR extracting %s: %v\n", st, err)
				result.Error = err
			} else {
				fmt.Printf("[STAGE 2] SUCCESS extracting %s (JSON length: %d bytes)\n", st, len(data))
				fmt.Printf("[STAGE 2] Raw JSON preview for %s: %s\n", st, truncateForLog(string(data), 300))
				result.Data = data
			}

			resultsChan <- result
		}(sectionType, sectionContent)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
		fmt.Println("[ParallelExtract] All extraction goroutines completed.")
	}()

	// Step 3b: Run Qualitative Agents (Strategy, Capital, Risk, Segments)
	// These run in parallel with statement agents but collected differently
	qualitativeResults := &QualitativeInsights{}
	var qualWg sync.WaitGroup

	// Helper to safely run qualitative agent
	runQualitative := func(name string, fn func() error) {
		qualWg.Add(1)
		go func() {
			defer qualWg.Done()
			if err := fn(); err != nil {
				fmt.Printf("[ParallelExtract] %s failed: %v\n", name, err)
			} else {
				fmt.Printf("[ParallelExtract] %s completed successfully\n", name)
			}
		}()
	}

	// 1. Strategy Agent (Needs MDA)
	if mda, ok := sections[MDAType]; ok && len(mda) > 100 {
		runQualitative("StrategyAgent", func() error {
			agent := NewStrategyAgent(provider)
			analysis, err := agent.Analyze(ctx, mda)
			if err == nil {
				qualitativeResults.Strategy = *analysis
			}
			return err
		})
	}

	// 2. Capital Allocation Agent (Needs MDA)
	if mda, ok := sections[MDAType]; ok && len(mda) > 100 {
		runQualitative("CapitalAllocationAgent", func() error {
			agent := NewCapitalAllocationAgent(provider)
			analysis, err := agent.Analyze(ctx, mda)
			if err == nil {
				qualitativeResults.CapitalAllocation = *analysis
			}
			return err
		})
	}

	// 3. Risk Agent (Needs Risk Factors)
	if risks, ok := sections[RiskFactorsType]; ok && len(risks) > 100 {
		runQualitative("RiskAgent", func() error {
			agent := NewRiskAgent(provider)
			analysis, err := agent.Analyze(ctx, risks)
			if err == nil {
				qualitativeResults.Risks = *analysis
			}
			return err
		})
	}

	// 4. Segment Agent (Needs Business + MDA)
	// We combine them or use what is available
	business, hasBusiness := sections[BusinessType]
	mda, hasMDA := sections[MDAType]
	if (hasBusiness && len(business) > 100) || (hasMDA && len(mda) > 100) {
		runQualitative("SegmentAgent", func() error {
			agent := NewSegmentAgent(provider)
			analysis, err := agent.Analyze(ctx, business, mda)
			if err == nil {
				qualitativeResults.Segments = *analysis
			}
			return err
		})
	}

	// 5. Quantitative Segment Agent (Needs Notes)
	// This supersedes the qualitative agent if successful, as it extracts hard numbers
	if notes, ok := sections[NotesType]; ok && len(notes) > 500 {
		runQualitative("QuantitativeSegmentAgent", func() error {
			agent := NewQuantitativeSegmentAgent(provider)
			// Pass the notes section to extract quantitative segment data
			analysis, err := agent.AnalyzeSegments(ctx, notes)
			if err == nil && analysis != nil && len(analysis.Segments) > 0 {
				qualitativeResults.Segments = *analysis
				fmt.Printf("[ParallelExtract] Quantitative Segment Analysis replaced Qualitative (found %d segments)\n", len(analysis.Segments))
			}
			return err
		})
	}

	// Wait for qualitative agents
	// We wait here to ensure qualitative data is ready before returning
	// However, to keep it truly parallel with financial agents, we could wait at the end
	// But since we need to wait for statement agents to close the channel loop below,
	// we can wait for qualWg separately.

	// Step 4: Collect and merge results
	response := &FSAPDataResponse{
		Company:        meta.CompanyName,
		CIK:            meta.CIK,
		FiscalYear:     meta.FiscalYear,
		FiscalPeriod:   meta.FiscalPeriod,
		SourceDocument: meta.Form,
		FilingURL:      meta.FilingURL,
		Metadata: Metadata{
			LLMProvider: "Parallel Agents",
		},
		Qualitative: qualitativeResults,
	}

	// Ensure qualitative agents are done
	qualWg.Wait()

	response.RawJSON = make(map[string]interface{})

	fmt.Println("\n[STAGE 3] Collecting results from goroutines...")
	fmt.Println("\n[STAGE 3] Collecting results from goroutines...")
	resultsReceived := 0

	// Use a loop with select to handle context cancellation (timeout)
loop:
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled or timed out during result collection: %w", ctx.Err())
		case result, ok := <-resultsChan:
			if !ok {
				// Channel closed, all agents done
				break loop
			}
			resultsReceived++
			if result.Error != nil {
				fmt.Printf("[STAGE 3] Result %d: %s - ERROR: %v\n", resultsReceived, result.StatementType, result.Error)
				continue // Log error but continue with other results
			}

			fmt.Printf("[STAGE 3] Result %d: %s - SUCCESS (data len: %d)\n", resultsReceived, result.StatementType, len(result.Data))

			// Save raw JSON for debugging
			response.RawJSON[string(result.StatementType)] = result.Data

			// Merge partial result into response
			mergePartialResult(response, result)
			// fmt.Printf("[STAGE 3] After merge: BS.CurrentAssets.Cash = %v\n", response.BalanceSheet.CurrentAssets.CashAndEquivalents)
		}
	}
	fmt.Printf("[STAGE 3] Total results received: %d\n", resultsReceived)

	// ========== PASS 2: FILTER SUBTOTALS FROM ADDITIONAL_ITEMS ==========
	// This uses LLM to dynamically identify and remove subtotals
	fmt.Println("[Pass2] Starting subtotal filtering for all statements...")

	// === INCOME STATEMENT ===
	// Filter Operating Cost Section
	if response.IncomeStatement.OperatingCostSection != nil && len(response.IncomeStatement.OperatingCostSection.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2(ctx, provider, response.IncomeStatement.OperatingCostSection.AdditionalItems)
		response.IncomeStatement.OperatingCostSection.AdditionalItems = filtered
	}

	// Filter Gross Profit Section
	if response.IncomeStatement.GrossProfitSection != nil && len(response.IncomeStatement.GrossProfitSection.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2(ctx, provider, response.IncomeStatement.GrossProfitSection.AdditionalItems)
		response.IncomeStatement.GrossProfitSection.AdditionalItems = filtered
	}

	// Filter Non-Operating Section
	if response.IncomeStatement.NonOperatingSection != nil && len(response.IncomeStatement.NonOperatingSection.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2(ctx, provider, response.IncomeStatement.NonOperatingSection.AdditionalItems)
		response.IncomeStatement.NonOperatingSection.AdditionalItems = filtered
	}

	// === BALANCE SHEET ===
	// Filter Current Assets
	if len(response.BalanceSheet.CurrentAssets.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2FSAPValue(ctx, provider, response.BalanceSheet.CurrentAssets.AdditionalItems)
		response.BalanceSheet.CurrentAssets.AdditionalItems = filtered
	}
	// Filter Noncurrent Assets
	if len(response.BalanceSheet.NoncurrentAssets.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2FSAPValue(ctx, provider, response.BalanceSheet.NoncurrentAssets.AdditionalItems)
		response.BalanceSheet.NoncurrentAssets.AdditionalItems = filtered
	}
	// Filter Current Liabilities
	if len(response.BalanceSheet.CurrentLiabilities.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2FSAPValue(ctx, provider, response.BalanceSheet.CurrentLiabilities.AdditionalItems)
		response.BalanceSheet.CurrentLiabilities.AdditionalItems = filtered
	}
	// Filter Noncurrent Liabilities
	if len(response.BalanceSheet.NoncurrentLiabilities.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2FSAPValue(ctx, provider, response.BalanceSheet.NoncurrentLiabilities.AdditionalItems)
		response.BalanceSheet.NoncurrentLiabilities.AdditionalItems = filtered
	}
	// Filter Equity
	if len(response.BalanceSheet.Equity.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2FSAPValue(ctx, provider, response.BalanceSheet.Equity.AdditionalItems)
		response.BalanceSheet.Equity.AdditionalItems = filtered
	}

	// === CASH FLOW STATEMENT ===
	// Filter Operating Activities
	if response.CashFlowStatement.OperatingActivities != nil && len(response.CashFlowStatement.OperatingActivities.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2(ctx, provider, response.CashFlowStatement.OperatingActivities.AdditionalItems)
		response.CashFlowStatement.OperatingActivities.AdditionalItems = filtered
	}
	// Filter Investing Activities
	if response.CashFlowStatement.InvestingActivities != nil && len(response.CashFlowStatement.InvestingActivities.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2(ctx, provider, response.CashFlowStatement.InvestingActivities.AdditionalItems)
		response.CashFlowStatement.InvestingActivities.AdditionalItems = filtered
	}
	// Filter Financing Activities
	if response.CashFlowStatement.FinancingActivities != nil && len(response.CashFlowStatement.FinancingActivities.AdditionalItems) > 0 {
		filtered, _ := filterSubtotalsPass2(ctx, provider, response.CashFlowStatement.FinancingActivities.AdditionalItems)
		response.CashFlowStatement.FinancingActivities.AdditionalItems = filtered
	}

	fmt.Println("[Pass2] Subtotal filtering complete for all statements.")

	// Populate Value fields from Years map for backward compatibility
	populateValuesFromYears(response)

	// Count mapped variables
	mapped, unmapped := countLLMVariables(response)
	response.Metadata.VariablesMapped = mapped
	response.Metadata.VariablesUnmapped = unmapped

	return response, nil
}

// ExtractSingleStatement extracts only a single statement type for isolated testing.
// This allows testing one statement at a time (e.g., just Balance Sheet for Apple).
func ExtractSingleStatement(ctx context.Context, fullMarkdown string, provider AIProvider, meta *FilingMetadata, stmtType StatementType) (*FSAPDataResponse, error) {
	if provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Step 1: Detect units from markdown headers
	units := DetectUnits(fullMarkdown)

	// Step 2: Split markdown by [TABLE: TYPE] markers
	sections := splitByTableMarkers(fullMarkdown)

	// Find the requested section
	sectionContent, found := sections[stmtType]
	if !found || len(sectionContent) < 500 {
		// Fallback: If specific section not found, use full markdown
		// This passes the burden of finding the table to the LLM (higher latent cost, but prevents failure)
		fmt.Printf("[ExtractSingleStatement] Warning: Section %s not found via markers. Falling back to full markdown.\n", stmtType)
		sectionContent = fullMarkdown
		if len(sectionContent) > 100000 {
			// Truncate if too huge to avoid context overflow (first 100k chars usually contain financials)
			sectionContent = sectionContent[:100000]
		}
	}

	// Step 3: Generate statement-specific prompt
	prompt := generateStatementPrompt(stmtType, units)

	// Step 4: Extract using LLM
	data, err := extractStatement(ctx, provider, prompt, sectionContent, meta)
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Step 5: Build response with just the single statement
	response := &FSAPDataResponse{
		Company:        meta.CompanyName,
		CIK:            meta.CIK,
		FiscalYear:     meta.FiscalYear,
		FiscalPeriod:   meta.FiscalPeriod,
		SourceDocument: meta.Form,
		FilingURL:      meta.FilingURL,
		Metadata: Metadata{
			LLMProvider: fmt.Sprintf("Single Agent (%s)", stmtType),
		},
	}

	// Parse and assign to correct field
	result := &partialResult{StatementType: stmtType, Data: data}
	mergePartialResult(response, result)

	// Populate Value fields from Years map for backward compatibility
	populateValuesFromYears(response)

	// Count mapped variables
	mapped, unmapped := countLLMVariables(response)
	response.Metadata.VariablesMapped = mapped
	response.Metadata.VariablesUnmapped = unmapped

	return response, nil
}

// ParseStatementType converts a string to StatementType
func ParseStatementType(s string) (StatementType, bool) {
	switch strings.ToUpper(s) {
	case "BALANCE_SHEET", "BS":
		return BalanceSheetType, true
	case "INCOME_STATEMENT", "IS":
		return IncomeStatementType, true
	case "CASH_FLOW", "CF":
		return CashFlowType, true
	case "SUPPLEMENTAL", "SP":
		return SupplementalType, true
	default:
		return "", false
	}
}

// splitByTableMarkers splits markdown content by [TABLE: TYPE] annotations
func splitByTableMarkers(markdown string) map[StatementType]string {
	sections := make(map[StatementType]string)

	// Patterns to find table markers
	markers := []struct {
		pattern       string
		statementType StatementType
	}{
		{`\[TABLE:\s*BALANCE_SHEET\]`, BalanceSheetType},
		{`\[TABLE:\s*INCOME_STATEMENT\]`, IncomeStatementType},
		{`\[TABLE:\s*CASH_FLOW(?:_STATEMENT)?\]`, CashFlowType},
		{`\[TABLE:\s*SUPPLEMENTAL\]`, SupplementalType},
		{`\[TABLE:\s*BUSINESS\]`, BusinessType},
		{`\[TABLE:\s*RISK_FACTORS\]`, RiskFactorsType},
		{`\[TABLE:\s*MDA\]`, MDAType},
		{`\[TABLE:\s*NOTES\]`, NotesType},
	}

	for _, m := range markers {
		re := regexp.MustCompile(m.pattern)
		matches := re.FindAllStringIndex(markdown, -1)

		if len(matches) > 0 {
			// Use first match (main consolidated statement, not parent company)
			startPos := matches[0][0]

			// Find end position (next major section or 50KB limit)
			endPos := findSectionEnd(markdown, startPos)

			if endPos > startPos {
				sections[m.statementType] = markdown[startPos:endPos]
			}
		}
	}

	// If no specific sections found (or critical ones missing), try to extract based on content headers
	// This is a backup for when [TABLE: XYZ] markers failed but content exists
	if len(sections) < 3 {
		// Helper scanning function
		scanForSection := func(st StatementType, keywords []string) {
			if _, exists := sections[st]; exists {
				return
			}
			for _, kw := range keywords {
				// Prepare flexible keyword pattern (spaces -> \s+)
				safeKw := regexp.QuoteMeta(kw)
				safeKw = strings.ReplaceAll(safeKw, " ", `\s+`)

				// Look for header logic:
				// \n -> Start of line (or after newline)
				// [^\w\n]* -> Skip any non-word characters (pipes, bullets, stars, spaces) but NOT newlines
				// (?:Consolidated\s+)? -> Optional "Consolidated" prefix
				// <Keyword> -> The section name
				pattern := fmt.Sprintf(`(?i)\n[^\w\n]*(?:Consolidated\s+)?%s.*`, safeKw)
				re := regexp.MustCompile(pattern)
				if loc := re.FindStringIndex(markdown); loc != nil {
					// Extract a reasonable chunk (50KB or until next marker)
					start := loc[0]
					end := findSectionEnd(markdown, start)
					if end > start+100 {
						sections[st] = markdown[start:end]
						fmt.Printf("[SmartScan] Found %s via keyword '%s'\n", st, kw)
						return
					}
				}
			}
		}

		scanForSection(BalanceSheetType, []string{"Balance Sheets", "Financial Position", "Assets", "Liabilities and Equity"})
		scanForSection(IncomeStatementType, []string{"Statements of Operations", "Statements of Income", "Statements of Earnings", "Loss and Comprehensive Loss"})
		scanForSection(CashFlowType, []string{"Statements of Cash Flows", "Cash Flows"})
	}

	return sections
}

// findSectionEnd finds where a statement section ends
func findSectionEnd(markdown string, startPos int) int {
	remaining := markdown[startPos:]
	maxLen := 50000 // 50KB max per section

	if len(remaining) < maxLen {
		maxLen = len(remaining)
	}

	// End patterns
	endPatterns := []string{
		`\[TABLE:\s*[A-Z_]+\]`, // Next table marker
		`(?i)\n\s*Item\s+9`,    // Item 9
		`(?i)\n\s*SIGNATURES`,  // Signatures section
	}

	minEnd := maxLen
	for _, pattern := range endPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringIndex(remaining[:maxLen]); match != nil && match[0] > 500 {
			if match[0] < minEnd {
				minEnd = match[0]
			}
		}
	}

	return startPos + minEnd
}

// generateStatementPrompt creates a statement-specific system prompt
// Dynamic mapping: LLM maps source items to FSAP schema + captures non-standard items
func generateStatementPrompt(st StatementType, units *DetectedUnits) string {
	// Try to load from prompt library first
	promptID := getPromptIDForStatement(st)
	if promptID != "" {
		if p, err := prompt.Get().GetSystemPrompt(promptID); err == nil && p != "" {
			return p
		}
	}

	// Fallback to hardcoded prompts
	return generateStatementPromptFallback(st, units)
}

// getPromptIDForStatement maps StatementType to prompt library ID
func getPromptIDForStatement(st StatementType) string {
	switch st {
	case BalanceSheetType:
		return prompt.PromptIDs.ExtractionBalanceSheet
	case IncomeStatementType:
		return prompt.PromptIDs.ExtractionIncomeStatement
	case CashFlowType:
		return prompt.PromptIDs.ExtractionCashFlow
	case SupplementalType:
		return prompt.PromptIDs.ExtractionSupplemental
	default:
		return ""
	}
}

// generateStatementPromptFallback contains the original hardcoded prompts as fallback
func generateStatementPromptFallback(st StatementType, units *DetectedUnits) string {
	baseRules := `You are an expert Financial Analyst extracting data from SEC 10-K filings.

EXTRACTION RULES:
1. Extract values EXACTLY as shown in the source (do NOT scale or convert)
2. **OMIT** standard fields that don't exist in source - DO NOT set them to 0 or null
3. Use the FIRST/MAIN consolidated statements (avoid "Parent Company Only", "Schedule I")
4. Extract ALL YEARS shown (typically 2-3 years)

CRITICAL - ADDITIONAL ITEMS:
- If an item in the source does NOT match any standard field, you MUST add it to "additional_items" array
- EVERY line item in the source must be captured - either in a standard field OR in additional_items
- Example: "Vendor non-trade receivables" is NOT a standard field - add to additional_items

VALUE FORMAT for each item:
- "value": current year value as number
- "label": EXACT original line item name from 10-K 
- "years": {"2024": X, "2023": Y} for multi-year data

ONLY include items that ACTUALLY EXIST in the source document!
Return ONLY valid JSON.`

	switch st {
	case BalanceSheetType:
		return baseRules + `

Your task: Extract BALANCE SHEET data.

=== SECTION 1: CURRENT ASSETS ===
Map source items to these standard fields:
- Cash and cash equivalents → cash_and_equivalents
- Short-term investments, marketable securities → short_term_investments
- Accounts/notes receivable, net → accounts_receivable_net
- Inventories → inventories
- Finance Div. Loans and Leases (current) → finance_div_loans_leases_st
- Finance Div. Other Current Assets → finance_div_other_curr_assets
- Other assets (current) → other_assets
- Other current assets (catch-all) → other_current_assets


=== SECTION 2: NONCURRENT ASSETS ===
- Long-term investments → long_term_investments
- Deferred Charges, LT → deferred_charges_lt
- Property, plant, equipment - at cost → ppe_at_cost
- Accumulated depreciation → accumulated_depreciation (NEGATIVE)
- Finance Div. Loans and Leases, LT → finance_div_loans_leases_lt
- Finance Div. Other LT Assets → finance_div_other_lt_assets
- Deferred Tax Assets, LT → deferred_tax_assets_lt
- Other noncurrent assets → other_noncurrent_assets
- Restricted Cash → restricted_cash


=== SECTION 3: CURRENT LIABILITIES ===
- Accounts payable → accounts_payable
- Accrued liabilities → accrued_liabilities
- Notes payable, short-term debt → notes_payable_short_term_debt
- Current maturities of long-term debt → current_maturities_long_term_debt
- Current operating lease liabilities → current_operating_lease_liabilities
- Finance Div Current → finance_div_curr
- Other Current Liabilities → other_current_liabilities
- Other current liabilities (catch-all) → other_current_liabilities_2


=== SECTION 4: NONCURRENT LIABILITIES ===
- Long-term debt → long_term_debt
- Long-term operating lease liabilities → long_term_operating_lease_liabilities
- Deferred tax liabilities → deferred_tax_liabilities
- Finance Div Non-Current → finance_div_noncurr
- Other noncurrent liabilities → other_noncurrent_liabilities

=== SECTION 5: EQUITY ===
- Common stock + APIC → common_stock_apic
- Retained earnings (deficit) → retained_earnings_deficit
- Treasury stock → treasury_stock (NEGATIVE)
- Accumulated OCI → accum_other_comprehensive_income
- Noncontrolling interests, Redeemable noncontrolling interests, NCI → noncontrolling_interests
  (CRITICAL: Extract this if it exists! Look for variants like "Redeemable noncontrolling interests in subsidiaries")


=== MASTER VALIDATION ===

CRITICAL REQUIREMENT: You MUST extract EVERY line item from the source!
1. Compare your extracted items with the reported Total (e.g., "Total current assets")
2. If there's a GAP, you MISSED items - add them to "additional_items" array

EXAMPLE: Apple has "Vendor non-trade receivables" (~$32B) which is NOT a standard field.
You MUST add it to additional_items:
{"key": "vendor_non_trade_receivables", "value": 32223, "label": "Vendor non-trade receivables", "years": {...}}

Common items that MUST go to additional_items if present:
- Vendor non-trade receivables
- Contract assets
- Prepaid expenses  
- Deferred costs
- Assets held for sale

JSON Structure (ONLY include fields that EXIST in source):
{
  "fiscal_years": [2024, 2023],
  "current_assets": {
    "cash_and_equivalents": {"label": "Cash and cash equivalents", "years": {"2024": 29943, "2023": 29965}},
    "short_term_investments": {"label": "Marketable securities", "years": {"2024": 35228, "2023": 31590}},
    "accounts_receivable_net": {"label": "Accounts receivable, net", "years": {"2024": 33410, "2023": 29508}},
    "inventories": {"label": "Inventories", "years": {"2024": 7286, "2023": 6331}},
    "additional_items": [
      {"label": "Vendor non-trade receivables", "years": {"2024": 32823, "2023": 31477}},
      {"label": "Other current assets", "years": {"2024": 14297, "2023": 14695}}
    ]
  },
  "noncurrent_assets": {
    "long_term_investments": {"label": "Marketable securities", "years": {"2024": 91479, "2023": 100544}},
    "ppe_net": {"label": "Property, plant and equipment, net", "years": {"2024": 45680, "2023": 43715}},
    "additional_items": [...]
  },
  "current_liabilities": {... only fields that exist ...},
  "noncurrent_liabilities": {... only fields that exist ...},
  "equity": {... only fields that exist ...},
  "_reported_for_validation": {
    "total_current_assets": {"value": 152987},
    "total_assets": {"value": 364980},
    "total_current_liabilities": {"value": 176392},
    "total_liabilities": {"value": 308030},
    "total_equity": {"value": 56950}
  }
}

RULES:
1. DO NOT include fields that don't exist in source
2. Use "additional_items" array for ANY item not matching standard fields
3. Each item needs only: "label" (original name) and "years" (multi-year values)`

	case IncomeStatementType:
		return baseRules + `

Your task: Extract INCOME STATEMENT / STATEMENT OF OPERATIONS data.

The Income Statement has 6 SECTIONS. Extract all items into their proper section:

=== SECTION 1: GROSS PROFIT ===
Mapping:
- Total revenues, net sales → revenues
- Cost of goods sold, cost of sales → cost_of_goods_sold  
- Gross profit (calculated: revenues - COGS) → gross_profit [GREY/calculated]

=== SECTION 2: OPERATING COST ===
Mapping:
- SG&A, selling general administrative → sga_expenses
- Advertising expenses → advertising_expenses
- R&D, research and development → rd_expenses
- Add back: Imputed interest on operating leases → imputed_interest_operating_leases
- Other operating expenses → other_operating_expenses (use array if multiple)
- Non-recurring operating expenses → non_recurring_operating_expenses
- Income/Loss from equity affiliates (if operating) → equity_affiliates_operating
- Non-recurring operating gains/losses → non_recurring_operating_gains_losses

=== SECTION 3: NON-OPERATING ITEMS ===
(Better name: "Other Income/Expense")
Mapping:
- Interest expense → interest_expense
- Other non-operating income/expense → other_non_operating_income_expense
- Other income/expense (net) → other_income_expense
- Income/Loss from equity affiliates (if non-operating) → equity_affiliates_non_operating
- Other income or gains, other expenses or losses → other_gains_losses

=== SECTION 4: TAX AND ADJUSTMENTS ===
(Better name: "Tax & Special Items")
Mapping:
- Income tax expense/provision → income_tax_expense
- Income/Loss from discontinued operations → discontinued_operations
- Extraordinary gains/losses → extraordinary_items
- Income/Loss from changes in accounting principles → accounting_changes

=== SECTION 5: NET INCOME ALLOCATION ===
Mapping:
- Net income attributable to common shareholders → net_income_to_common
- Net income attributable to noncontrolling interests → net_income_to_nci

=== SECTION 6: OTHER COMPREHENSIVE INCOME (OCI) ===
Mapping:
- Foreign currency translation adjustments → oci_foreign_currency
- Unrealized gains/losses on securities → oci_securities
- Pension adjustments → oci_pension
- Cash flow hedge adjustments → oci_hedges
- Total other comprehensive income → oci_total
- Comprehensive income → comprehensive_income

=== SUPPLEMENTAL: NONRECURRING ITEMS ===
(For analysis only - NOT included in IS calculations above)
Extract all non-recurring, unusual, or one-time items:
- Impairment charges → impairment_charges
- Restructuring charges → restructuring_charges
- Gain/Loss on asset sales → gain_loss_asset_sales
- Settlement/litigation costs → settlement_costs
- Write-offs → write_offs
- Other non-recurring items → other_nonrecurring

=== ADDITIONAL ITEMS ===
For ANY line item NOT matching standard fields above, add to "additional_items" array.
Examples: "Other operating expenses", "Licensing revenue", "Service revenue", etc.

CRITICAL EXCLUSION RULE:
- Do NOT add calculated subtotals to "additional_items"
- EXCLUDE: "Total Operating Expenses", "Total Net Sales", "Gross Profit", "Operating Income", "Net Income"
- EXCLUDE: "Income Before Tax", "Total Other Income", "Total Costs"
- WE CALCULATE THESE LOCALLY. Extracting them causes double-counting!

JSON Structure (ONLY include fields that EXIST in source):
{
  "fiscal_years": [2024, 2023],
  
  "gross_profit_section": {
    "revenues": {"label": "<source label>", "years": {"2024": X, "2023": Y}},
    "cost_of_goods_sold": {...},
    "gross_profit": {...}
  },
  
  "operating_cost_section": {
    "sga_expenses": {...},
    "rd_expenses": {...},
    "additional_items": [{"label": "<non-standard item>", "years": {...}}]
  },
  
  "non_operating_section": {
    "interest_expense": {...},
    "other_income_expense": {...}
  },
  
  "tax_adjustments_section": {
    "income_tax_expense": {...}
  },
  
  "net_income_section": {
    "net_income_to_common": {...}
  },
  
  "oci_section": {
    "oci_total": {...}
  },
  
  "nonrecurring_section": {
    "impairment_charges": {...},
    "restructuring_charges": {...},
    "additional_items": [...]
  },
  
  "_reported_for_validation": {
    "total_revenues": {"value": X},
    "gross_profit": {"value": X},
    "operating_income": {"value": X},
    "income_before_tax": {"value": X},
    "net_income": {"value": X}
  }
}`

	case CashFlowType:
		return baseRules + `

Your task: Extract CASH FLOW STATEMENT data following the FSAP 5-section structure.

=== SECTION 1: OPERATING ACTIVITIES ===
Map these items to the "operating_activities" object:
- Net income (starting point) → net_income_start
- Depreciation & amortization → depreciation_amortization
- Amortization of intangibles → amortization_intangibles
- Deferred income taxes → deferred_taxes
- Stock-based compensation → stock_based_compensation
- Impairment charges, write-downs → impairment_charges
- Gain/loss on asset sales → gain_loss_asset_sales
- Changes in accounts receivable → change_receivables
- Changes in inventory → change_inventory
- Changes in accounts payable → change_payables
- Changes in accrued expenses → change_accrued_expenses
- Changes in deferred revenue → change_deferred_revenue
- Other working capital changes → other_working_capital
- Other non-cash items → other_non_cash_items

=== SECTION 2: INVESTING ACTIVITIES ===
Map these items to the "investing_activities" object:
- Capital expenditures, purchases of PPE → capex
- Acquisitions, net of cash → acquisitions_net
- Purchases of marketable securities → purchases_securities
- Proceeds from maturities of securities → maturities_securities
- Proceeds from sales of securities → sales_securities
- Proceeds from asset sales → proceeds_asset_sales
- Other investing activities → other_investing

=== SECTION 3: FINANCING ACTIVITIES ===
Map these items to the "financing_activities" object:
- Proceeds from debt issuance → debt_proceeds
- Repayments of debt → debt_repayments
- Proceeds from stock issuance → stock_issuance_proceeds
- Share repurchases, buybacks → share_repurchases
- Dividends paid → dividends_paid
- Payments for employee tax withholding → tax_withholding_payments
- Other financing activities → other_financing

=== SECTION 4: SUPPLEMENTAL INFORMATION ===
Map these items to the "supplemental_info" object:
- Cash paid for interest → cash_interest_paid
- Cash paid for income taxes → cash_taxes_paid
- Non-cash investing activities → non_cash_investing
- Non-cash financing activities → non_cash_financing

=== SECTION 5: CASH SUMMARY ===
Map totals and reconciliation to "cash_summary" object:
- Net cash from operating activities → net_cash_operating (GREY - calculated subtotal)
- Net cash from investing activities → net_cash_investing (GREY - calculated subtotal)
- Net cash from financing activities → net_cash_financing (GREY - calculated subtotal)
- Effect of exchange rate changes → fx_effect
- Net change in cash → net_change_in_cash (GREY - calculated)
- Cash at beginning of period → cash_beginning
- Cash at end of period → cash_ending (GREY - calculated)


=== ADDITIONAL ITEMS ===
For ANY line item NOT matching standard fields above, add to "additional_items" array in the appropriate section.
Examples: "Deferred income taxes", "Other operating activities", "Payments for acquisitions", etc.

JSON Structure (ONLY include fields that EXIST in source):
{
  "fiscal_years": [2024, 2023],
  "operating_activities": {
    "net_income_start": {"label": "<source label>", "years": {"2024": X, "2023": Y}},
    "depreciation_amortization": {...},
    "stock_based_compensation": {...},
    "change_receivables": {...},
    "change_payables": {...},
    "additional_items": [{"label": "<non-standard item>", "years": {...}}]
  },
  "investing_activities": {
    "capex": {...},
    "purchases_securities": {...},
    "sales_securities": {...},
    "additional_items": [...]
  },
  "financing_activities": {
    "debt_repayments": {...},
    "share_repurchases": {...},
    "dividends_paid": {...},
    "additional_items": [...]
  },
  "cash_summary": {
    "net_cash_operating": {"label": "Net cash from operating activities", "years": {...}},
    "net_cash_investing": {...},
    "net_cash_financing": {...},
    "net_change_in_cash": {...}
  }
}`

	case SupplementalType:
		return baseRules + `

Your task: Extract SUPPLEMENTAL / PER-SHARE data.

MAPPING GUIDE:
- Basic earnings per share → eps_basic
- Diluted earnings per share → eps_diluted
- Basic shares outstanding → shares_outstanding_basic
- Diluted shares outstanding → shares_outstanding_diluted
- Effective tax rate → effective_tax_rate
- Dividends per share → dividend_per_share

=== ADDITIONAL ITEMS ===
For ANY line item NOT matching standard fields above, add to "additional_items" array.
Examples: "Book value per share", "Cash dividends declared", "Weighted average shares", etc.

JSON Structure (ONLY include fields that EXIST in source):
{
  "fiscal_years": [2024, 2023],
  "eps_basic": {"label": "<source label>", "years": {"2024": X, "2023": Y}},
  "eps_diluted": {...},
  "shares_outstanding_basic": {...},
  "shares_outstanding_diluted": {...},
  "additional_items": [{"label": "<non-standard item>", "years": {...}}]
}`

	default:
		return baseRules
	}
}

// extractStatement calls the LLM to extract data for a specific statement
func extractStatement(ctx context.Context, provider AIProvider, systemPrompt string, content string, meta *FilingMetadata) (json.RawMessage, error) {
	userPrompt := fmt.Sprintf(`Extract financial data for %s (Fiscal Year %d):

%s

Return ONLY the JSON object.`, meta.CompanyName, meta.FiscalYear, content)

	resp, err := provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Clean response
	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	// Validate it's valid JSON
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(cleanJson), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	// DEBUG: Print and save raw LLM response
	fmt.Printf("\n=== LLM RAW RESPONSE ===\n%s\n=== END ===\n", cleanJson)

	// Save to log file
	logDir := "logs"
	os.MkdirAll(logDir, 0755)
	timestamp := time.Now().Format("20060102_150405")
	logFile := fmt.Sprintf("%s/llm_output_%s.json", logDir, timestamp)
	os.WriteFile(logFile, []byte(cleanJson), 0644)
	fmt.Printf("[LOG] LLM response saved to: %s\n", logFile)

	return raw, nil
}

// filterSubtotalsPass2 performs a second LLM pass to filter out subtotals from additional_items.
// This is more robust than hardcoded filtering as LLM can dynamically understand context.
func filterSubtotalsPass2(ctx context.Context, provider AIProvider, additionalItems []AdditionalItem) ([]AdditionalItem, error) {
	if len(additionalItems) == 0 {
		return additionalItems, nil
	}

	// Build a list of labels for LLM to review
	var labels []string
	for _, item := range additionalItems {
		labels = append(labels, item.Label)
	}

	labelsJSON, _ := json.Marshal(labels)

	systemPrompt := `You are a financial data quality analyst.

Your task: Review a list of financial line item labels and identify which are SUBTOTALS vs ATOMIC ITEMS.

SUBTOTALS are items that:
- Represent the SUM of other items in the same section
- Contains words like "Total", "Subtotal", "Net", "Aggregate"
- Common examples: "Total operating expenses", "Gross profit", "Operating income", "Net income"

ATOMIC ITEMS are:
- Individual line items that cannot be derived by summing other items
- Examples: "Research and development", "Selling expenses", "Advertising costs"

Return a JSON array containing ONLY the labels that are ATOMIC ITEMS (not subtotals).
Return ONLY the JSON array, no explanation.`

	userPrompt := fmt.Sprintf(`Review these financial line item labels and return only the ATOMIC items (exclude subtotals):

%s

Return only a JSON array of labels that are ATOMIC items.`, string(labelsJSON))

	resp, err := provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		fmt.Printf("[Pass2] LLM error, keeping all items: %v\n", err)
		return additionalItems, nil // Fallback: keep all if LLM fails
	}

	// Parse response
	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	var atomicLabels []string
	if err := json.Unmarshal([]byte(cleanJson), &atomicLabels); err != nil {
		fmt.Printf("[Pass2] Failed to parse LLM response, keeping all items: %v\n", err)
		return additionalItems, nil // Fallback: keep all if parse fails
	}

	// Create a set of atomic labels for fast lookup
	atomicSet := make(map[string]bool)
	for _, label := range atomicLabels {
		atomicSet[strings.ToLower(label)] = true
	}

	// Filter items
	var filtered []AdditionalItem
	for _, item := range additionalItems {
		if atomicSet[strings.ToLower(item.Label)] {
			filtered = append(filtered, item)
		} else {
			fmt.Printf("[Pass2] Filtered out subtotal: %s\n", item.Label)
		}
	}

	// Safety check: If filtered list is empty but input was not, assume LLM error and return all items
	// This prevents "0 variables mapped" error if LLM hallucinates or fails
	if len(filtered) == 0 && len(additionalItems) > 0 {
		fmt.Printf("[Pass2] WARNING: Filtered list is empty (LLM likely failed). Keeping all %d items as fallback.\n", len(additionalItems))
		return additionalItems, nil
	}

	fmt.Printf("[Pass2] Kept %d/%d items after subtotal filtering\n", len(filtered), len(additionalItems))
	return filtered, nil
}

// filterSubtotalsPass2FSAPValue filters subtotals from []FSAPValue (used by Balance Sheet)
func filterSubtotalsPass2FSAPValue(ctx context.Context, provider AIProvider, additionalItems []FSAPValue) ([]FSAPValue, error) {
	if len(additionalItems) == 0 {
		return additionalItems, nil
	}

	// Build a list of labels for LLM to review
	var labels []string
	for _, item := range additionalItems {
		labels = append(labels, item.Label)
	}

	labelsJSON, _ := json.Marshal(labels)

	systemPrompt := `You are a financial data quality analyst.

Your task: Review a list of financial line item labels and identify which are SUBTOTALS vs ATOMIC ITEMS.

SUBTOTALS are items that:
- Represent the SUM of other items in the same section
- Contains words like "Total", "Subtotal", "Net", "Aggregate"
- Common examples: "Total current assets", "Total liabilities", "Total equity", "Net cash from operations"

ATOMIC ITEMS are:
- Individual line items that cannot be derived by summing other items
- Examples: "Cash and equivalents", "Accounts receivable", "Inventories", "Property plant equipment"

Return a JSON array containing ONLY the labels that are ATOMIC ITEMS (not subtotals).
Return ONLY the JSON array, no explanation.`

	userPrompt := fmt.Sprintf(`Review these financial line item labels and return only the ATOMIC items (exclude subtotals):

%s

Return only a JSON array of labels that are ATOMIC items.`, string(labelsJSON))

	resp, err := provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		fmt.Printf("[Pass2-BS] LLM error, keeping all items: %v\n", err)
		return additionalItems, nil
	}

	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	var atomicLabels []string
	if err := json.Unmarshal([]byte(cleanJson), &atomicLabels); err != nil {
		fmt.Printf("[Pass2-BS] Failed to parse LLM response, keeping all items: %v\n", err)
		return additionalItems, nil
	}

	atomicSet := make(map[string]bool)
	for _, label := range atomicLabels {
		atomicSet[strings.ToLower(label)] = true
	}

	var filtered []FSAPValue
	for _, item := range additionalItems {
		if atomicSet[strings.ToLower(item.Label)] {
			filtered = append(filtered, item)
		} else {
			fmt.Printf("[Pass2-BS] Filtered out subtotal: %s\n", item.Label)
		}
	}

	// Safety check: If filtered list is empty but input was not, assume LLM error and return all items
	if len(filtered) == 0 && len(additionalItems) > 0 {
		fmt.Printf("[Pass2-BS] WARNING: Filtered list is empty (LLM likely failed). Keeping all %d items as fallback.\n", len(additionalItems))
		return additionalItems, nil
	}

	fmt.Printf("[Pass2-BS] Kept %d/%d items after subtotal filtering\n", len(filtered), len(additionalItems))
	return filtered, nil
}

// mergePartialResult merges a partial extraction result into the main response
func mergePartialResult(response *FSAPDataResponse, result *partialResult) {
	if result.Data == nil {
		fmt.Printf("[MERGE] Warning: result.Data is nil for %s\n", result.StatementType)
		return
	}

	fmt.Printf("[MERGE] Attempting to unmarshal %s (%d bytes)\n", result.StatementType, len(result.Data))

	var err error
	switch result.StatementType {
	case BalanceSheetType:
		err = json.Unmarshal(result.Data, &response.BalanceSheet)
		if err != nil {
			fmt.Printf("[MERGE] ERROR unmarshaling BalanceSheet: %v\n", err)
			fmt.Printf("[MERGE] Raw JSON: %s\n", truncateForLog(string(result.Data), 500))
		} else {
			fmt.Printf("[MERGE] SUCCESS: BS.CurrentAssets.Cash = %v\n", response.BalanceSheet.CurrentAssets.CashAndEquivalents)
		}
	case IncomeStatementType:
		err = json.Unmarshal(result.Data, &response.IncomeStatement)
		if err != nil {
			fmt.Printf("[MERGE] ERROR unmarshaling IncomeStatement: %v\n", err)
		}
	case CashFlowType:
		err = json.Unmarshal(result.Data, &response.CashFlowStatement)
		if err != nil {
			fmt.Printf("[MERGE] ERROR unmarshaling CashFlowStatement: %v\n", err)
		}
	case SupplementalType:
		err = json.Unmarshal(result.Data, &response.SupplementalData)
		if err != nil {
			fmt.Printf("[MERGE] ERROR unmarshaling SupplementalData: %v\n", err)
		}
	}
}

// populateValuesFromYears ensures that the legacy Value field is populated from the Years map
// This is critical because the calculation logic relies on Value, but the LLM now only returns Years
func populateValuesFromYears(resp *FSAPDataResponse) {
	if resp == nil {
		return
	}
	targetYear := fmt.Sprintf("%d", resp.FiscalYear)

	// Process each statement
	populateStruct(&resp.BalanceSheet, targetYear)
	populateStruct(&resp.IncomeStatement, targetYear)
	populateStruct(&resp.CashFlowStatement, targetYear)
	populateStruct(&resp.SupplementalData, targetYear)
}

// populateStruct uses reflection to traverse FSAP structs and populate Value from Years
func populateStruct(v interface{}, year string) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}

	// Check if this is an FSAPValue
	// We detect by checking for "Value" (ptr float64) and "Years" (map) fields
	valueField := val.FieldByName("Value")
	yearsField := val.FieldByName("Years")

	if valueField.IsValid() && yearsField.IsValid() && valueField.CanSet() {
		// This is likely an FSAPValue or similar struct
		// But skip AdditionalItem which has Years but Value is *FSAPValue, not *float64
		// Check that Value field is of type *float64
		if valueField.Type().String() != "*float64" {
			// Not an FSAPValue-style struct, skip direct population
			// But still need to recurse into nested structures
			goto recurse
		}
		if yearsField.Kind() == reflect.Map && !yearsField.IsNil() {
			// Find year value
			mapVal := yearsField.MapIndex(reflect.ValueOf(year))
			if mapVal.IsValid() {
				// Set Value field
				floatVal := mapVal.Float()
				valueField.Set(reflect.ValueOf(&floatVal))
			}
		}
		return // Stop recursing if we found a leaf node
	}

recurse:
	// Handle AdditionalItems arrays
	if val.Type().Name() == "AdditionalItem" {
		// Populate value for AdditionalItem wrapper
		if val.FieldByName("Value").IsValid() {
			// AdditionalItem.Value is of type *FSAPValue, so recurse into it
			populateStruct(val.FieldByName("Value").Interface(), year)
		}
		return
	}

	// Recurse into fields
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)

		// Handle slices (e.g. AdditionalItems)
		if field.Kind() == reflect.Slice {
			for j := 0; j < field.Len(); j++ {
				populateStruct(field.Index(j).Addr().Interface(), year)
			}
			continue
		}

		// Handle nested structs/pointers
		if field.Kind() == reflect.Ptr || field.Kind() == reflect.Struct {
			populateStruct(field.Interface(), year)
		}
	}
}
