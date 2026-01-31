// Package ingest provides SEC EDGAR API integration for fetching company filings.
// API Documentation: https://www.sec.gov/developer
package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// SEC EDGAR API endpoints
	SECSubmissionsURL = "https://data.sec.gov/submissions/CIK%s.json"
	SECFilingURL      = "https://www.sec.gov/Archives/edgar/data/%s/%s"

	// Required User-Agent per SEC guidelines
	UserAgent = "AgenticValuation/1.0 (contact@example.com)"
)

// =============================================================================
// SEC EDGAR DATA TYPES
// =============================================================================

// SECCompanyInfo represents the top-level company submission response.
type SECCompanyInfo struct {
	CIK            string     `json:"cik"`
	EntityType     string     `json:"entityType"`
	SIC            string     `json:"sic"`
	SICDescription string     `json:"sicDescription"`
	Name           string     `json:"name"`
	Tickers        []string   `json:"tickers"`
	Exchanges      []string   `json:"exchanges"`
	Filings        SECFilings `json:"filings"`
}

// SECFilings contains recent and older filing lists.
type SECFilings struct {
	Recent SECRecentFilings `json:"recent"`
}

// SECRecentFilings holds arrays of filing attributes (parallel arrays).
type SECRecentFilings struct {
	AccessionNumber []string `json:"accessionNumber"` // e.g., "0000037996-24-000012"
	FilingDate      []string `json:"filingDate"`      // e.g., "2024-02-06"
	ReportDate      []string `json:"reportDate"`      // Fiscal period end
	Form            []string `json:"form"`            // "10-K", "10-Q", "8-K"
	PrimaryDocument []string `json:"primaryDocument"` // filename
	Size            []int    `json:"size"`            // bytes
}

// Filing represents a single SEC filing (denormalized from parallel arrays).
type Filing struct {
	AccessionNumber string    `json:"accession_number"`
	FilingDate      time.Time `json:"filing_date"`
	ReportDate      time.Time `json:"report_date"`
	FormType        string    `json:"form_type"`
	PrimaryDocument string    `json:"primary_document"`
	Size            int       `json:"size"`
	URL             string    `json:"url"` // Constructed download URL
}

// =============================================================================
// SEC EDGAR CLIENT
// =============================================================================

// EDGARClient handles SEC EDGAR API requests.
type EDGARClient struct {
	httpClient *http.Client
}

// NewEDGARClient creates a new SEC EDGAR API client.
func NewEDGARClient() *EDGARClient {
	return &EDGARClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchCompanyInfo retrieves company submission data from SEC EDGAR.
//
// CIK should be zero-padded to 10 digits (e.g., "0000037996" for Ford).
// If not padded, this function will pad it automatically.
func (c *EDGARClient) FetchCompanyInfo(cik string) (*SECCompanyInfo, error) {
	// Zero-pad CIK to 10 digits
	cik = fmt.Sprintf("%010s", strings.TrimLeft(cik, "0"))

	url := fmt.Sprintf(SECSubmissionsURL, cik)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// SEC requires User-Agent header
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SEC API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEC API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var info SECCompanyInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse SEC response: %w", err)
	}

	return &info, nil
}

// GetFilings extracts and returns filings filtered by form type.
//
// formTypes: "10-K", "10-Q", "8-K", etc. Pass nil for all types.
// limit: Maximum number of filings to return (0 = no limit).
func (c *EDGARClient) GetFilings(info *SECCompanyInfo, formTypes []string, limit int) []Filing {
	recent := info.Filings.Recent
	filings := make([]Filing, 0)

	formTypeSet := make(map[string]bool)
	for _, ft := range formTypes {
		formTypeSet[ft] = true
	}

	for i := range recent.AccessionNumber {
		// Filter by form type if specified
		if len(formTypes) > 0 && !formTypeSet[recent.Form[i]] {
			continue
		}

		// Parse dates
		filingDate, _ := time.Parse("2006-01-02", recent.FilingDate[i])
		reportDate, _ := time.Parse("2006-01-02", recent.ReportDate[i])

		// Construct download URL
		// Format: https://www.sec.gov/Archives/edgar/data/{cik}/{accession-no-dashes}/{document}
		accessionNoDashes := strings.ReplaceAll(recent.AccessionNumber[i], "-", "")
		downloadURL := fmt.Sprintf(SECFilingURL, info.CIK, accessionNoDashes+"/"+recent.PrimaryDocument[i])

		filings = append(filings, Filing{
			AccessionNumber: recent.AccessionNumber[i],
			FilingDate:      filingDate,
			ReportDate:      reportDate,
			FormType:        recent.Form[i],
			PrimaryDocument: recent.PrimaryDocument[i],
			Size:            recent.Size[i],
			URL:             downloadURL,
		})

		// Apply limit
		if limit > 0 && len(filings) >= limit {
			break
		}
	}

	return filings
}

// =============================================================================
// CONVENIENCE FUNCTIONS
// =============================================================================

// LookupCIKByTicker finds the CIK for a given ticker symbol.
// Note: SEC provides a ticker -> CIK mapping file at:
// https://www.sec.gov/files/company_tickers.json
func LookupCIKByTicker(ticker string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://www.sec.gov/files/company_tickers.json", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch ticker mapping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SEC ticker API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read ticker mapping: %w", err)
	}

	// Response structure: { "0": {"cik_str": 320193, "ticker": "AAPL", "title": "..."}, ... }
	var mapping map[string]struct {
		CIK    int    `json:"cik_str"`
		Ticker string `json:"ticker"`
		Title  string `json:"title"`
	}

	if err := json.Unmarshal(body, &mapping); err != nil {
		return "", fmt.Errorf("failed to parse ticker mapping: %w", err)
	}

	ticker = strings.ToUpper(ticker)
	for _, entry := range mapping {
		if entry.Ticker == ticker {
			return fmt.Sprintf("%010d", entry.CIK), nil
		}
	}

	return "", fmt.Errorf("ticker %s not found in SEC database", ticker)
}

// FetchLatest10K fetches the most recent 10-K filing for a ticker.
func FetchLatest10K(ticker string) (*Filing, error) {
	cik, err := LookupCIKByTicker(ticker)
	if err != nil {
		return nil, err
	}

	client := NewEDGARClient()
	info, err := client.FetchCompanyInfo(cik)
	if err != nil {
		return nil, err
	}

	filings := client.GetFilings(info, []string{"10-K"}, 1)
	if len(filings) == 0 {
		return nil, fmt.Errorf("no 10-K filings found for %s", ticker)
	}

	return &filings[0], nil
}
