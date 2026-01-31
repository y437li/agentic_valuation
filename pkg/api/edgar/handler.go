// Package edgar provides HTTP API handlers for SEC EDGAR data extraction.
// Refactored to use LLM Navigator + Go Extractor pattern (no XBRL).
package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"agentic_valuation/pkg/core/agent"
	"agentic_valuation/pkg/core/calc"
	coreEdgar "agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/fee"
)

// Package-level Agent Manager for LLM integration
var agentManager *agent.Manager

// InitHandler initializes the handler with an agent manager
func InitHandler(manager *agent.Manager) {
	agentManager = manager
}

// =============================================================================
// FSAP EXTRACTION HANDLER (LLM Navigator + Go Extractor)
// =============================================================================

// FSAPMappingRequest for the FSAP-format endpoint
type FSAPMappingRequest struct {
	Ticker     string `json:"ticker"`
	CIK        string `json:"cik"`
	Form       string `json:"form"`        // "10-K" or "10-Q"
	FiscalYear int    `json:"fiscal_year"` // Target fiscal year
}

// HandleEdgarFSAPMapping handles POST /api/edgar/fsap-map
// Returns data in fsap_data JSON format with source_path evidence
func HandleEdgarFSAPMapping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FSAPMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default values
	if req.Ticker == "" && req.CIK == "" {
		req.Ticker = "AAPL" // Default to Apple
	}
	if req.Form == "" {
		req.Form = "10-K"
	}

	// Resolve CIK from ticker if needed
	cik := req.CIK
	if cik == "" && req.Ticker != "" {
		var err error
		cik, err = lookupCIKByTicker(req.Ticker)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	startTime := time.Now()

	// Create parser
	parser := coreEdgar.NewParser()

	// Step 1: Get filing metadata
	meta, err := parser.GetFilingMetadataByYear(cik, req.Form, req.FiscalYear)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get filing metadata: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 2: Fetch HTML using Smart Fetcher (handles exhibit redirects)
	html, err := parser.FetchSmartFilingHTML(meta)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch filing HTML: %v", err), http.StatusInternalServerError)
		return
	}

	// Debug: Log HTML size
	log.Printf("[Handler] Fetched HTML size: %d bytes", len(html))
	if len(html) > 200 {
		log.Printf("[Handler] HTML preview: %s...", html[:200])
	}

	// Step 3: Check if agent manager is available
	if agentManager == nil {
		http.Error(w, "Agent manager not initialized", http.StatusInternalServerError)
		return
	}

	// Step 4: Use FEE orchestrator for extraction
	feeMetadata := fee.DocumentMetadata{
		CIK:             meta.CIK,
		CompanyName:     meta.CompanyName,
		FilingDate:      meta.FilingDate,
		Form:            meta.Form,
		AccessionNumber: meta.AccessionNumber,
	}

	// Get LLM provider for extraction
	provider := agentManager.GetProvider("data_extraction")
	aiProvider := coreEdgar.NewLLMAdapter(provider)

	orchestrator := fee.NewExtractionOrchestrator(nil, aiProvider)
	resp, err := orchestrator.ExtractToFSAP(context.Background(), html, feeMetadata, req.FiscalYear)
	if err != nil {
		http.Error(w, fmt.Sprintf("Extraction failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 5: Calculate aggregates
	calc.CalculateBalanceSheetTotals(&resp.BalanceSheet)

	// Set metadata
	resp.Metadata.ProcessingTimeMs = time.Since(startTime).Milliseconds()
	resp.Metadata.LLMProvider = "LLM Navigator + Go Extractor"

	json.NewEncoder(w).Encode(resp)
}

// =============================================================================
// CIK LOOKUP (kept for ticker resolution)
// =============================================================================

func lookupCIKByTicker(ticker string) (string, error) {
	// Common tickers for quick lookup
	tickerMap := map[string]string{
		"AAPL":  "0000320193",
		"MSFT":  "0000789019",
		"GOOGL": "0001652044",
		"AMZN":  "0001018724",
		"META":  "0001326801",
		"TSLA":  "0001318605",
		"NVDA":  "0001045810",
		"F":     "0000037996",
		"GM":    "0001467858",
		"INTC":  "0000050863",
	}

	if cik, ok := tickerMap[ticker]; ok {
		return cik, nil
	}

	return "", fmt.Errorf("ticker %s not found in lookup table", ticker)
}
