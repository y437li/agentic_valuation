// Package ingest provides SEC EDGAR API integration and content fetching.
// This file implements the pipeline.ContentFetcher interface for live SEC data.
package ingest

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/edgar/converter"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SECContentFetcher implements pipeline.ContentFetcher using live SEC EDGAR data.
// It fetches HTML from SEC Archives and converts to Markdown via Pandoc.
type SECContentFetcher struct {
	client    *EDGARClient
	converter *converter.PandocAdapter
	cacheDir  string // Optional local cache directory
}

// NewSECContentFetcher creates a new content fetcher for live SEC data.
// If cacheDir is provided, fetched content will be cached locally.
func NewSECContentFetcher(cacheDir string) *SECContentFetcher {
	return &SECContentFetcher{
		client:    NewEDGARClient(),
		converter: converter.NewPandocAdapter(),
		cacheDir:  cacheDir,
	}
}

// FetchMarkdown implements pipeline.ContentFetcher interface.
// It fetches the filing HTML from SEC EDGAR and converts to Markdown.
// Uses edgar.Parser.FetchSmartFilingHTML which correctly handles iXBRL format.
func (f *SECContentFetcher) FetchMarkdown(ctx context.Context, cik string, accessionNumber string) (string, error) {
	// 1. Check cache first
	if f.cacheDir != "" {
		cacheKey := fmt.Sprintf("%s_%s.md", cik, strings.ReplaceAll(accessionNumber, "-", ""))
		cachePath := filepath.Join(f.cacheDir, "markdown", cacheKey)
		if content, err := os.ReadFile(cachePath); err == nil && len(content) > 50000 {
			// Only use cache if it's substantial (>50KB)
			return string(content), nil
		}
	}

	// 2. Use edgar.Parser to get filing metadata and smart-fetch HTML
	parser := edgar.NewParser()
	meta, err := parser.GetFilingMetadataByAccession(cik, accessionNumber)
	if err != nil {
		return "", fmt.Errorf("failed to get filing metadata: %w", err)
	}

	// 3. Use FetchSmartFilingHTML which handles iXBRL correctly
	// This finds the main document (>500KB) with actual table content
	html, err := parser.FetchSmartFilingHTML(meta)
	if err != nil {
		return "", fmt.Errorf("failed to fetch smart filing HTML: %w", err)
	}

	// 4. Convert HTML to Markdown using the full HTMLToMarkdown pipeline
	// This includes HTMLSanitizer which preserves tables through placeholder restoration
	markdown := edgar.HTMLToMarkdown(html)

	if len(markdown) < 1000 {
		return "", fmt.Errorf("conversion produced insufficient content (%d bytes)", len(markdown))
	}

	// 5. Cache the result
	if f.cacheDir != "" {
		cacheKey := fmt.Sprintf("%s_%s.md", cik, strings.ReplaceAll(accessionNumber, "-", ""))
		cachePath := filepath.Join(f.cacheDir, "markdown", cacheKey)
		os.MkdirAll(filepath.Dir(cachePath), 0755)
		os.WriteFile(cachePath, []byte(markdown), 0644)
	}

	return markdown, nil
}

// fetchHTML downloads a filing document from SEC EDGAR.
func (f *SECContentFetcher) fetchHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := f.client.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SEC returned status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// FetchFilingByTicker is a convenience method to fetch the latest 10-K for a ticker.
func (f *SECContentFetcher) FetchFilingByTicker(ctx context.Context, ticker string) (string, *Filing, error) {
	filing, err := FetchLatest10K(ticker)
	if err != nil {
		return "", nil, err
	}

	// Get CIK from ticker
	cik, err := LookupCIKByTicker(ticker)
	if err != nil {
		return "", nil, err
	}

	markdown, err := f.FetchMarkdown(ctx, cik, filing.AccessionNumber)
	if err != nil {
		return "", nil, err
	}

	return markdown, filing, nil
}
