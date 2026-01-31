package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/llm"

	"github.com/joho/godotenv"
)

// DeepSeekAIProvider wrapper for extraction
type DeepSeekAIProvider struct {
	provider *llm.DeepSeekProvider
}

func (p *DeepSeekAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}

// CachedExtraction represents the JSON structure for cached extraction
type CachedExtraction struct {
	ID              string                  `json:"id"`
	CIK             string                  `json:"cik"`
	Ticker          string                  `json:"ticker"`
	CompanyName     string                  `json:"company_name"`
	FiscalYear      int                     `json:"fiscal_year"`
	FiscalPeriod    string                  `json:"fiscal_period"`
	FormType        string                  `json:"form_type"`
	AccessionNumber string                  `json:"accession_number"`
	Data            *edgar.FSAPDataResponse `json:"data"`
}

func main() {
	// Load environment
	if err := godotenv.Load("../../../.env"); err != nil {
		log.Println("Warning: .env not found, using environment variables")
	}

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("Error: DEEPSEEK_API_KEY is not set")
	}

	// Target CIK and years
	cik := "0001318605" // Tesla Inc.
	ticker := "TSLA"
	years := []int{2022} // Test NCI fix on FY2022 only

	parser := edgar.NewParser()
	provider := &DeepSeekAIProvider{provider: &llm.DeepSeekProvider{}}

	for _, year := range years {
		fmt.Printf("\n=== Processing FY%d 10-K ===\n", year)

		// Check if already cached - DISABLED FOR NCI TEST
		cachePath := fmt.Sprintf("../../batch_data/%s/%s_FY%d.json", ticker, ticker, year)
		_ = cachePath // Defined but skip check for re-extraction test

		// 1. Get Metadata
		fmt.Printf("Step 1: Fetching metadata for FY%d...\n", year)
		meta, err := parser.GetFilingMetadataByYear(cik, "10-K", year)
		if err != nil {
			log.Printf("Error fetching FY%d metadata: %v\n", year, err)
			continue
		}
		fmt.Printf("Found: %s (Accession: %s, Filed: %s)\n", meta.Form, meta.AccessionNumber, meta.FilingDate)

		// 2. Fetch HTML
		fmt.Println("Step 2: Fetching filing HTML...")
		html, err := parser.FetchSmartFilingHTML(meta)
		if err != nil {
			log.Printf("Error fetching HTML: %v\n", err)
			continue
		}
		fmt.Printf("HTML size: %d bytes\n", len(html))

		// 3. Convert to Markdown
		fmt.Println("Step 3: Converting to Markdown...")
		markdown := parser.ExtractItem8Markdown(html)
		fmt.Printf("Markdown size: %d bytes\n", len(markdown))

		// 4. Parallel Extraction
		fmt.Println("Step 4: Running parallel LLM extraction...")
		startTime := time.Now()
		extracted, err := edgar.ParallelExtract(context.Background(), markdown, provider, meta)
		if err != nil {
			log.Printf("Extraction failed for FY%d: %v\n", year, err)
			continue
		}
		fmt.Printf("Extraction completed in %v\n", time.Since(startTime))

		// 5. Save to cache
		cached := CachedExtraction{
			ID:              meta.AccessionNumber,
			CIK:             meta.CIK,
			Ticker:          ticker,
			CompanyName:     meta.CompanyName,
			FiscalYear:      year,
			FiscalPeriod:    "FY",
			FormType:        meta.Form,
			AccessionNumber: meta.AccessionNumber,
			Data:            extracted,
		}

		jsonData, err := json.MarshalIndent(cached, "", "  ")
		if err != nil {
			log.Printf("JSON marshal error: %v\n", err)
			continue
		}

		// Ensure directory exists
		os.MkdirAll(fmt.Sprintf("../../batch_data/%s", ticker), 0755)
		err = os.WriteFile(cachePath, jsonData, 0644)
		if err != nil {
			log.Printf("Error writing cache: %v\n", err)
			continue
		}

		fmt.Printf("Saved: %s\n", cachePath)

		// Rate limit between years
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n=== Done ===")
}
