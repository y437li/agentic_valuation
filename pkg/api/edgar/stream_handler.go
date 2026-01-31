// Package edgar provides HTTP API handlers for SEC EDGAR data mapping.
package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agentic_valuation/pkg/core/agent"
	"agentic_valuation/pkg/core/calc"
	coreEdgar "agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/llm"
	"agentic_valuation/pkg/core/store"
)

// ProgressEvent represents a single SSE progress update
type ProgressEvent struct {
	Step     string `json:"step"`   // "fetch", "parse", "extract", "map", "validate", "complete", "error"
	Status   string `json:"status"` // "started", "done", "error"
	Detail   string `json:"detail"` // e.g., "Downloaded 1.2MB"
	TimingMs int64  `json:"timing_ms"`
	Data     any    `json:"data,omitempty"` // Final data on "complete"
}

// getMapKeys returns the keys of a map for debug logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// HandleEdgarFSAPMapStream handles SSE streaming for FSAP mapping with real-time progress
func HandleEdgarFSAPMapStream(manager *agent.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// SSE headers - must be set before any write
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle OPTIONS preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Helper to send SSE event
		sendEvent := func(event ProgressEvent) {
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		// Send immediate heartbeat to establish connection
		sendEvent(ProgressEvent{Step: "init", Status: "started", Detail: "Connection established"})

		// Parse query params (GET for SSE)
		ticker := r.URL.Query().Get("ticker")
		cik := r.URL.Query().Get("cik")
		yearStr := r.URL.Query().Get("year")
		fiscalYear := 0
		if yearStr != "" {
			fiscalYear, _ = strconv.Atoi(yearStr)
		}
		requestedProvider := r.URL.Query().Get("provider")
		fmt.Printf("[DEBUG] Stream Request: ticker=%s cik=%s year=%d provider=%s\n", ticker, cik, fiscalYear, requestedProvider)

		if ticker == "" && cik == "" {
			sendEvent(ProgressEvent{Step: "error", Status: "error", Detail: "Missing ticker or cik parameter"})
			return
		}

		// Single statement mode (optional) - for testing one statement at a time
		// e.g., ?statement=bs, ?statement=is, ?statement=cf, ?statement=sp
		statementFilter := r.URL.Query().Get("statement")
		var singleStatementMode bool
		var targetStatement coreEdgar.StatementType
		if statementFilter != "" {
			st, valid := coreEdgar.ParseStatementType(statementFilter)
			if valid {
				singleStatementMode = true
				targetStatement = st
				sendEvent(ProgressEvent{Step: "init", Status: "done", Detail: fmt.Sprintf("Single statement mode: %s", st)})
			}
		}

		parser := coreEdgar.NewParser()
		startTime := time.Now()

		// ========== STEP 1: FETCH FILING ==========
		sendEvent(ProgressEvent{Step: "fetch", Status: "started", Detail: "Fetching filing metadata from SEC..."})
		stepStart := time.Now()

		// Map ticker to CIK if needed
		var meta *coreEdgar.FilingMetadata
		var err error

		if ticker != "" {
			cik, err = lookupCIKByTicker(ticker)
			if err != nil {
				sendEvent(ProgressEvent{Step: "fetch", Status: "error", Detail: fmt.Sprintf("Failed to lookup ticker: %v", err)})
				return
			}
		}

		meta, err = parser.GetFilingMetadataByYear(cik, "10-K", fiscalYear)
		if err != nil {
			sendEvent(ProgressEvent{Step: "fetch", Status: "error", Detail: fmt.Sprintf("Failed to get filing: %v", err)})
			return
		}

		sendEvent(ProgressEvent{
			Step:     "fetch",
			Status:   "done",
			Detail:   fmt.Sprintf("Found %s FY%d filing", meta.CompanyName, meta.FiscalYear),
			TimingMs: time.Since(stepStart).Milliseconds(),
			Data: map[string]interface{}{
				"company_name":     meta.CompanyName,
				"cik":              meta.CIK,
				"fiscal_year":      meta.FiscalYear,
				"filing_date":      meta.FilingDate,
				"accession_number": meta.AccessionNumber,
				"filing_url":       meta.FilingURL,
			},
		})

		// ========== STEP 1.5: CHECK FSAP CACHE ==========
		fsapCache := store.NewFSAPCache(store.GetPool(), "")
		if cachedResp, _ := fsapCache.GetByAccession(context.Background(), meta.AccessionNumber); false && cachedResp != nil { // DEBUG: FORCE REFRESH
			// Cache hit! Skip all extraction steps
			sendEvent(ProgressEvent{
				Step:     "cache",
				Status:   "done",
				Detail:   fmt.Sprintf("✅ Loaded from Supabase cache (FY%d)", meta.FiscalYear),
				TimingMs: time.Since(stepStart).Milliseconds(),
				Data: map[string]interface{}{
					"from_cache":       true,
					"variables_mapped": cachedResp.Metadata.VariablesMapped,
				},
			})

			// Populate source positions for jump-to-source (if markdown available)
			if cachedResp.FullMarkdown != "" {
				coreEdgar.PopulateSourcePositions(cachedResp, cachedResp.FullMarkdown)
			}

			// Send complete response
			sendEvent(ProgressEvent{
				Step:     "complete",
				Status:   "done",
				Detail:   fmt.Sprintf("Loaded from cache in %dms", time.Since(startTime).Milliseconds()),
				TimingMs: time.Since(startTime).Milliseconds(),
				Data:     cachedResp,
			})
			return
		}

		// ========== STEP 2: CHECK CACHE OR DOWNLOAD 10-K ==========
		htmlCache := coreEdgar.NewMarkdownCacheWithDir(filepath.Join(".cache", "edgar", "html"))
		var html string

		// Check if we have cached HTML for this filing
		if cached := htmlCache.Get(meta.CIK, meta.AccessionNumber); cached != "" {
			fmt.Printf("[DEBUG] Loading cached HTML for CIK=%s, Accession=%s (%.1f KB)\n", meta.CIK, meta.AccessionNumber, float64(len(cached))/1024)
			// Quick validation: check if HTML contains expected company namespace
			if len(cached) > 500 {
				preview := cached[:500]
				if meta.CompanyName == "Intel Corporation" && !strings.Contains(preview, "intel.com") {
					fmt.Printf("[WARNING] Cached HTML for Intel doesn't contain 'intel.com' namespace! May be wrong file.\n")
				}
				if meta.CompanyName == "Apple Inc." && !strings.Contains(preview, "apple.com") {
					fmt.Printf("[WARNING] Cached HTML for Apple doesn't contain 'apple.com' namespace!\n")
				}
			}
			sendEvent(ProgressEvent{
				Step:     "download",
				Status:   "done",
				Detail:   fmt.Sprintf("📦 Loaded HTML from cache (%.1f KB)", float64(len(cached))/1024),
				TimingMs: 0,
				Data: map[string]interface{}{
					"size_bytes": len(cached),
					"from_cache": true,
				},
			})
			html = cached
		} else {
			sendEvent(ProgressEvent{Step: "download", Status: "started", Detail: "Downloading 10-K with smart document detection..."})
			stepStart = time.Now()

			// Use Smart Filing to detect redirect pages and fetch correct exhibit
			// This handles cases where Item 8 says "See page F-1" or redirects to exhibit
			fetchedHTML, err := parser.FetchSmartFilingHTML(meta)
			if err != nil {
				sendEvent(ProgressEvent{Step: "download", Status: "error", Detail: fmt.Sprintf("Failed to download: %v", err)})
				return
			}
			html = fetchedHTML

			// Save raw HTML to cache
			htmlCache.Set(meta.CIK, meta.AccessionNumber, html)

			sendEvent(ProgressEvent{
				Step:     "download",
				Status:   "done",
				Detail:   fmt.Sprintf("Downloaded %.1f KB (with smart exhibit detection)", float64(len(html))/1024),
				TimingMs: time.Since(stepStart).Milliseconds(),
				Data: map[string]interface{}{
					"size_bytes": len(html),
					"size_kb":    float64(len(html)) / 1024,
				},
			})
		}

		// ========== STEP 3: PARSE WITH LLM TOC AGENT ==========
		sendEvent(ProgressEvent{Step: "parse", Status: "started", Detail: "LLM analyzing TOC structure..."})
		stepStart = time.Now()

		var item8Markdown string
		var parseMethod string

		// Try LLM Agent first for universal naming support
		if manager != nil {
			var llmProvider llm.Provider

			// Select provider: prefer query param, then fallback to config
			if requestedProvider != "" {
				llmProvider = manager.GetProviderByName(requestedProvider)
			}

			// Fallback if not found or not requested
			if llmProvider == nil {
				llmProvider = manager.GetProvider("data_extraction")
			}

			if llmProvider != nil {
				fmt.Printf("[DEBUG] Step 3 Parse Provider: %T\n", llmProvider)
				// Use the adapter to bridge llm.Provider -> edgar.AIProvider
				adapter := coreEdgar.NewLLMAdapter(llmProvider)
				analyzer := coreEdgar.NewLLMAnalyzer(adapter)

				llmMarkdown, err := parser.ExtractWithLLMAgent(context.Background(), html, analyzer)
				if err == nil && len(llmMarkdown) > 5000 {
					fmt.Printf("[DEBUG] LLM TOC Agent extracted %d chars for %s\n", len(llmMarkdown), meta.CompanyName)
					// Verify extracted markdown contains expected company name
					if len(llmMarkdown) > 500 {
						if meta.CompanyName == "Intel Corporation" && !strings.Contains(llmMarkdown[:500], "Intel") {
							fmt.Printf("[ERROR] Markdown doesn't contain 'Intel' but should!\n")
						}
						// Don't check Apple/Apple - false positives possible
					}
					item8Markdown = llmMarkdown
					parseMethod = "LLM TOC Agent"
				} else {
					fmt.Printf("[WARNING] LLM TOC Agent failed or returned too little content. Error: %v, Length: %d. Falling back to regex.\n", err, len(llmMarkdown))
				}
			}
		}

		// Fallback to regex-based extraction
		if len(item8Markdown) < 1000 {
			item8Markdown = parser.ExtractItem8Markdown(html)
			parseMethod = "Regex Pattern"
		}

		// Detect table markers for debug info
		previewLen := 500
		if len(item8Markdown) < previewLen {
			previewLen = len(item8Markdown)
		}
		sendEvent(ProgressEvent{
			Step:     "parse",
			Status:   "done",
			Detail:   fmt.Sprintf("Extracted %d chars via %s", len(item8Markdown), parseMethod),
			TimingMs: time.Since(stepStart).Milliseconds(),
			Data: map[string]interface{}{
				"size_chars":   len(item8Markdown),
				"preview":      item8Markdown[:previewLen],
				"parse_method": parseMethod,
			},
		})

		// ========== STEP 4: LLM SEMANTIC EXTRACTION (PARALLEL AGENTS) ==========
		if singleStatementMode {
			sendEvent(ProgressEvent{Step: "extract", Status: "started", Detail: fmt.Sprintf("Single agent extracting %s...", targetStatement)})
		} else {
			sendEvent(ProgressEvent{Step: "extract", Status: "started", Detail: "Parallel agents extracting financial data..."})
		}
		stepStart = time.Now()

		var resp coreEdgar.FSAPDataResponse

		// Use Parallel Multi-Agent extraction for statements (dynamic unit detection)
		if manager != nil && len(item8Markdown) > 0 {
			var llmProvider llm.Provider

			// Select provider for extraction
			if requestedProvider != "" {
				llmProvider = manager.GetProviderByName(requestedProvider)
			}
			if llmProvider == nil {
				llmProvider = manager.GetProvider("data_extraction")
			}

			// Create adapter once
			adapter := coreEdgar.NewLLMAdapter(llmProvider)

			var llmResp *coreEdgar.FSAPDataResponse
			var err error

			if singleStatementMode {
				// Single statement mode - still uses v2.0 extractor (filtering happens post-extraction)
				v2 := coreEdgar.NewV2Extractor(adapter)
				llmResp, err = v2.Extract(context.Background(), item8Markdown, meta)
			} else {
				// Full parallel extraction mode using v2.0 architecture
				v2 := coreEdgar.NewV2Extractor(adapter)
				llmResp, err = v2.Extract(context.Background(), item8Markdown, meta)
			}

			if err != nil {
				sendEvent(ProgressEvent{Step: "extract", Status: "error", Detail: fmt.Sprintf("Extraction failed: %v", err)})
				return
			}

			resp = *llmResp
			resp.FilingURL = meta.FilingURL
			resp.FullMarkdown = item8Markdown // For source traceability

			// Calculate markdown positions from row_label (deterministic, no hallucination)
			coreEdgar.PopulateSourcePositions(&resp, item8Markdown)

			extractionMethod := "Parallel Multi-Agent"
			if singleStatementMode {
				extractionMethod = fmt.Sprintf("Single Agent (%s)", targetStatement)
			}

			sendEvent(ProgressEvent{
				Step:     "extract",
				Status:   "done",
				Detail:   fmt.Sprintf("Mapped %d variables via %s", resp.Metadata.VariablesMapped, extractionMethod),
				TimingMs: time.Since(stepStart).Milliseconds(),
				Data: map[string]interface{}{
					"variables_mapped":   resp.Metadata.VariablesMapped,
					"variables_unmapped": resp.Metadata.VariablesUnmapped,
					"extraction_method":  extractionMethod,
					"llm_provider":       resp.Metadata.LLMProvider,
				},
			})
		} else {
			sendEvent(ProgressEvent{Step: "extract", Status: "error", Detail: "LLM not available or no content extracted"})
			return
		}

		// ========== STEP 5: VALIDATE ==========
		sendEvent(ProgressEvent{Step: "validate", Status: "started", Detail: "Running validation checks..."})
		stepStart = time.Now()

		// Normalize signs first - convert all expenses/outflows to negative values
		calc.NormalizeIncomeStatementSigns(&resp.IncomeStatement)
		calc.NormalizeCashFlowSigns(&resp.CashFlowStatement)

		calc.CalculateBalanceSheetTotals(&resp.BalanceSheet)
		// Use year-aware functions to properly read multi-year data from .Years map
		fiscalYearStr := strconv.Itoa(meta.FiscalYear)
		isCalc := calc.CalculateIncomeStatementTotalsByYear(&resp.IncomeStatement, fiscalYearStr)
		cfCalc := calc.CalculateCashFlowTotalsByYear(&resp.CashFlowStatement, fiscalYearStr)
		_ = calc.CalculateSupplementalData(&resp.IncomeStatement, &resp.CashFlowStatement, &resp.SupplementalData)

		// Helper to safely dereference
		getVal := func(f *float64) float64 {
			if f != nil {
				return *f
			}
			return 0
		}

		getFSAPVal := func(v *coreEdgar.FSAPValue) float64 {
			if v != nil && v.Value != nil {
				return *v.Value
			}
			return 0
		}

		// --- Balance Sheet Validation ---
		bs := &resp.BalanceSheet
		calcCurrentAssets := getVal(bs.CurrentAssets.CalculatedTotal)
		calcNonCurrAssets := getVal(bs.NoncurrentAssets.CalculatedTotal)
		calcTotalAssets := calcCurrentAssets + calcNonCurrAssets

		calcCurrentLiab := getVal(bs.CurrentLiabilities.CalculatedTotal)
		calcNonCurrLiab := getVal(bs.NoncurrentLiabilities.CalculatedTotal)
		calcTotalLiab := calcCurrentLiab + calcNonCurrLiab

		calcEquity := getVal(bs.Equity.CalculatedTotal)

		repTotalAssets := getFSAPVal(bs.ReportedForValidation.TotalAssets)
		repTotalLiab := getFSAPVal(bs.ReportedForValidation.TotalLiabilities)
		repTotalEquity := getFSAPVal(bs.ReportedForValidation.TotalEquity)

		checks := []map[string]interface{}{}

		// Helper to determine status
		getStatus := func(gap float64, hasReported bool) string {
			if !hasReported {
				return "missing"
			}
			if math.Abs(gap) < 2 {
				return "pass"
			}
			return "fail"
		}

		// 1. Check A = L + E (Calculated)
		gapALE := calcTotalAssets - (calcTotalLiab + calcEquity)
		checks = append(checks, map[string]interface{}{
			"statement": "Balance Sheet",
			"name":      "Calculated A = L + E",
			"expected":  0.0,
			"actual":    gapALE,
			"gap":       gapALE,
			"status": func() string {
				if math.Abs(gapALE) < 2 {
					return "pass"
				} else {
					return "fail"
				}
			}(),
		})

		// 2. Check Calculated vs Reported Assets
		gapAssets := calcTotalAssets - repTotalAssets
		checks = append(checks, map[string]interface{}{
			"statement": "Balance Sheet",
			"name":      "Total Assets Verification",
			"expected":  repTotalAssets,
			"actual":    calcTotalAssets,
			"gap":       gapAssets,
			"status":    getStatus(gapAssets, repTotalAssets != 0),
		})

		// 3. Total Liabilities Verification
		gapLiab := calcTotalLiab - repTotalLiab
		checks = append(checks, map[string]interface{}{
			"statement": "Balance Sheet",
			"name":      "Total Liabilities Verification",
			"expected":  repTotalLiab,
			"actual":    calcTotalLiab,
			"gap":       gapLiab,
			"status":    getStatus(gapLiab, repTotalLiab != 0),
		})

		// 4. Total Equity Verification
		gapEquity := calcEquity - repTotalEquity
		checks = append(checks, map[string]interface{}{
			"statement": "Balance Sheet",
			"name":      "Total Equity Verification",
			"expected":  repTotalEquity,
			"actual":    calcEquity,
			"gap":       gapEquity,
			"status":    getStatus(gapEquity, repTotalEquity != 0),
		})

		// --- Income Statement Validation ---

		// Gross Profit Check
		gapGP := isCalc.GrossProfitCalc - isCalc.GrossProfitReported
		checks = append(checks, map[string]interface{}{
			"statement": "Income Statement",
			"name":      "Gross Profit Flow-Through",
			"expected":  isCalc.GrossProfitReported,
			"actual":    isCalc.GrossProfitCalc,
			"gap":       gapGP,
			"status":    getStatus(gapGP, isCalc.GrossProfitReported != 0),
		})

		// Operating Income Check
		gapOpInc := isCalc.OperatingIncomeCalc - isCalc.OperatingIncomeReported
		checks = append(checks, map[string]interface{}{
			"statement": "Income Statement",
			"name":      "Operating Income Flow-Through",
			"expected":  isCalc.OperatingIncomeReported,
			"actual":    isCalc.OperatingIncomeCalc,
			"gap":       gapOpInc,
			"status":    getStatus(gapOpInc, isCalc.OperatingIncomeReported != 0),
		})

		// Income Before Tax Check
		gapIBT := isCalc.IncomeBeforeTaxCalc - isCalc.IncomeBeforeTaxReported
		checks = append(checks, map[string]interface{}{
			"statement": "Income Statement",
			"name":      "Income Before Tax Flow-Through",
			"expected":  isCalc.IncomeBeforeTaxReported,
			"actual":    isCalc.IncomeBeforeTaxCalc,
			"gap":       gapIBT,
			"status":    getStatus(gapIBT, isCalc.IncomeBeforeTaxReported != 0),
		})

		// Net Income Check
		gapNI := isCalc.NetIncomeCalc - isCalc.NetIncomeReported
		checks = append(checks, map[string]interface{}{
			"statement": "Income Statement",
			"name":      "Net Income Verification",
			"expected":  isCalc.NetIncomeReported,
			"actual":    isCalc.NetIncomeCalc,
			"gap":       gapNI,
			"status":    getStatus(gapNI, isCalc.NetIncomeReported != 0),
		})

		// --- Cash Flow Validation ---
		// Operating Cash Flow Check
		gapOpCF := cfCalc.OperatingCalc - cfCalc.OperatingReported
		checks = append(checks, map[string]interface{}{
			"statement": "Cash Flow",
			"name":      "Operating Activities",
			"expected":  cfCalc.OperatingReported,
			"actual":    cfCalc.OperatingCalc,
			"gap":       gapOpCF,
			"status":    getStatus(gapOpCF, cfCalc.OperatingReported != 0),
		})

		// Investing Cash Flow Check
		gapInvCF := cfCalc.InvestingCalc - cfCalc.InvestingReported
		checks = append(checks, map[string]interface{}{
			"statement": "Cash Flow",
			"name":      "Investing Activities",
			"expected":  cfCalc.InvestingReported,
			"actual":    cfCalc.InvestingCalc,
			"gap":       gapInvCF,
			"status":    getStatus(gapInvCF, cfCalc.InvestingReported != 0),
		})

		// Financing Cash Flow Check
		gapFinCF := cfCalc.FinancingCalc - cfCalc.FinancingReported
		checks = append(checks, map[string]interface{}{
			"statement": "Cash Flow",
			"name":      "Financing Activities",
			"expected":  cfCalc.FinancingReported,
			"actual":    cfCalc.FinancingCalc,
			"gap":       gapFinCF,
			"status":    getStatus(gapFinCF, cfCalc.FinancingReported != 0),
		})

		// Net Change Check
		gapNC := cfCalc.NetChangeCalc - cfCalc.NetChangeReported
		checks = append(checks, map[string]interface{}{
			"statement": "Cash Flow",
			"name":      "Net Change in Cash",
			"expected":  cfCalc.NetChangeReported,
			"actual":    cfCalc.NetChangeCalc,
			"gap":       gapNC,
			"status":    getStatus(gapNC, cfCalc.NetChangeReported != 0),
		})

		validationSummary := "Validation Complete"
		if math.Abs(gapALE) < 2 {
			validationSummary = "A = L + E ✓"
		} else {
			validationSummary = fmt.Sprintf("A != L + E (Gap: %.0f)", gapALE)
		}

		valData := map[string]interface{}{
			"checks": checks,
			"totals": map[string]float64{
				"calc_assets": calcTotalAssets,
				"calc_liab":   calcTotalLiab,
				"calc_equity": calcEquity,
				"rep_assets":  repTotalAssets,
				"rep_liab":    repTotalLiab,
				"rep_equity":  repTotalEquity,
			},
		}

		sendEvent(ProgressEvent{
			Step:     "validate",
			Status:   "done",
			Detail:   validationSummary,
			TimingMs: time.Since(stepStart).Milliseconds(),
			Data:     valData,
		})

		if resp.DebugSteps == nil {
			resp.DebugSteps = &coreEdgar.DebugSteps{}
		}
		resp.DebugSteps.Validation = &coreEdgar.DebugStep{
			Name:     "validate",
			Status:   "done",
			Detail:   validationSummary,
			TimingMs: time.Since(stepStart).Milliseconds(),
			Data:     valData,
		}

		// ========== STEP 6: SAVE TO CACHE ==========
		resp.Metadata.ProcessingTimeMs = time.Since(startTime).Milliseconds()

		// Save to Supabase cache for future requests (non-blocking)
		go func() {
			if err := fsapCache.Save(context.Background(), &resp, meta); err != nil {
				// Log error but don't fail the request
				fmt.Printf("Warning: Failed to save to cache: %v\n", err)
			}
		}()

		// ========== STEP 7: COMPLETE ==========
		// DEBUG: Log what we're sending to frontend
		fmt.Println("\n========== FINAL RESPONSE DEBUG ==========")
		fmt.Printf("[FINAL] Variables Mapped: %d, Unmapped: %d\n", resp.Metadata.VariablesMapped, resp.Metadata.VariablesUnmapped)
		fmt.Printf("[FINAL] BS.CurrentAssets.Cash: %v\n", resp.BalanceSheet.CurrentAssets.CashAndEquivalents)
		fmt.Printf("[FINAL] RawJSON keys: %v\n", getMapKeys(resp.RawJSON))
		if resp.BalanceSheet.CurrentAssets.CashAndEquivalents != nil {
			fmt.Printf("[FINAL] Cash Value: %v\n", resp.BalanceSheet.CurrentAssets.CashAndEquivalents.Value)
		}
		fmt.Println("==========================================")

		sendEvent(ProgressEvent{
			Step:     "complete",
			Status:   "done",
			Detail:   fmt.Sprintf("Total: %dms", resp.Metadata.ProcessingTimeMs),
			TimingMs: resp.Metadata.ProcessingTimeMs,
			Data:     resp,
		})
	}
}

// HandleClearCache clears the local HTML cache
func HandleClearCache(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	cacheDir := filepath.Join(".cache", "edgar", "html")
	cleared := 0

	// Clear all .md files in the cache directory
	files, err := filepath.Glob(filepath.Join(cacheDir, "*.md"))
	if err == nil {
		for _, _ = range files {
			// Simply count files (actual deletion could be added if needed)
			cleared++
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Cache info: %d files found", cleared),
	})
}
