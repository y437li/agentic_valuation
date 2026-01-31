package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentic_valuation/pkg/core/edgar"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FSAPCache provides caching for LLM-extracted FSAP data
// Supports Hybrid Vault: DB (Primary) + File System (Fallback/Local)
type FSAPCache struct {
	pool    *pgxpool.Pool
	fileDir string
}

// NewFSAPCache creates a new FSAP cache instance
// If pool is nil, it falls back to a file-based cache in the specified directory.
// If dir is empty, it acts as a no-op cache (or defaults to .cache if pool is nil).
func NewFSAPCache(pool *pgxpool.Pool, dir string) *FSAPCache {
	if pool == nil && dir == "" {
		dir = filepath.Join(".cache", "edgar", "fsap_extractions")
	}
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("[WARNING] Check FSAPCache dir: %v\n", err)
		}
	}
	return &FSAPCache{pool: pool, fileDir: dir}
}

// CacheEntry represents a cached FSAP extraction
type CacheEntry struct {
	ID              string                  `json:"id"`
	CIK             string                  `json:"cik"`
	Ticker          string                  `json:"ticker"`
	CompanyName     string                  `json:"company_name"`
	FiscalYear      int                     `json:"fiscal_year"`
	FiscalPeriod    string                  `json:"fiscal_period"`
	FormType        string                  `json:"form_type"`
	AccessionNumber string                  `json:"accession_number"`
	FilingDate      string                  `json:"filing_date"`  // NEW: Date filed with SEC (e.g., "2024-11-01")
	IsAmendment     bool                    `json:"is_amendment"` // NEW: True if 10-K/A or 10-Q/A
	Data            *edgar.FSAPDataResponse `json:"data"`
	ExtractedAt     time.Time               `json:"extracted_at"`
	LLMProvider     string                  `json:"llm_provider"`
}

// GetByAccession retrieves cached data by SEC accession number (most reliable key)
func (c *FSAPCache) GetByAccession(ctx context.Context, accessionNumber string) (*edgar.FSAPDataResponse, error) {
	// 1. Try DB
	if c.pool != nil {
		query := `
			SELECT data 
			FROM fsap_extractions 
			WHERE accession_number = $1
			LIMIT 1
		`
		var dataJSON []byte
		err := c.pool.QueryRow(ctx, query, accessionNumber).Scan(&dataJSON)
		if err == nil {
			var resp edgar.FSAPDataResponse
			if err := json.Unmarshal(dataJSON, &resp); err != nil {
				return nil, fmt.Errorf("failed to unmarshal db cached data: %w", err)
			}
			return &resp, nil
		}
		// If DB fails or returns no rows, fall through to File check?
		// Usually we assume if DB is configured, we rely on it. But Hybrid might imply consistency.
		// For now, if pool is set, we rely on it. Only use File if pool is nil.
		return nil, nil
	}

	// 2. Try File System
	if c.fileDir != "" {
		path := c.accessionPath(accessionNumber)
		return c.loadFromFile(path)
	}

	return nil, nil
}

// Get retrieves cached data by CIK, fiscal year, and period
func (c *FSAPCache) Get(ctx context.Context, cik string, fiscalYear int, fiscalPeriod string) (*edgar.FSAPDataResponse, error) {
	if c.pool != nil {
		query := `
			SELECT data 
			FROM fsap_extractions 
			WHERE cik = $1 AND fiscal_year = $2 AND fiscal_period = $3
			ORDER BY created_at DESC
			LIMIT 1
		`
		var dataJSON []byte
		err := c.pool.QueryRow(ctx, query, cik, fiscalYear, fiscalPeriod).Scan(&dataJSON)
		if err != nil {
			return nil, nil // Cache miss
		}
		var resp edgar.FSAPDataResponse
		if err := json.Unmarshal(dataJSON, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal db cached data: %w", err)
		}
		return &resp, nil
	}

	// File fallback (Iterative search - potentially slow, but fine for local)
	if c.fileDir != "" {
		// Since files are named by accession, we must scan or maintain an index.
		// Optimized: If we knew the Accession, we'd use GetByAccession.
		// Without Accession, we have to glob/scan.
		return c.scanFileCache(cik, fiscalYear)
	}

	return nil, nil
}

// GetByTicker retrieves cached data by ticker symbol
func (c *FSAPCache) GetByTicker(ctx context.Context, ticker string, fiscalYear int, fiscalPeriod string) (*edgar.FSAPDataResponse, error) {
	if c.pool != nil {
		query := `
			SELECT data 
			FROM fsap_extractions 
			WHERE ticker = $1 AND fiscal_year = $2 AND fiscal_period = $3
			ORDER BY created_at DESC
			LIMIT 1
		`
		var dataJSON []byte
		err := c.pool.QueryRow(ctx, query, ticker, fiscalYear, fiscalPeriod).Scan(&dataJSON)
		if err != nil {
			return nil, nil
		}
		var resp edgar.FSAPDataResponse
		if err := json.Unmarshal(dataJSON, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal db cached data: %w", err)
		}
		return &resp, nil
	}

	// File fallback (Scan)
	if c.fileDir != "" {
		return c.scanFileCacheByTicker(ticker, fiscalYear)
	}

	return nil, nil
}

// Save stores extracted FSAP data in the cache
func (c *FSAPCache) Save(ctx context.Context, resp *edgar.FSAPDataResponse, meta *edgar.FilingMetadata) error {
	dataJSON, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Use ticker from metadata or response
	ticker := ""
	if len(meta.Tickers) > 0 {
		ticker = meta.Tickers[0]
	}

	// 1. Save to DB
	if c.pool != nil {
		query := `
			INSERT INTO fsap_extractions (
				cik, ticker, company_name, fiscal_year, fiscal_period, 
				form_type, accession_number, filing_url, data,
				llm_provider, variables_mapped, processing_time_ms
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (accession_number) 
			DO UPDATE SET 
				data = EXCLUDED.data,
				updated_at = NOW(),
				llm_provider = EXCLUDED.llm_provider,
				variables_mapped = EXCLUDED.variables_mapped,
				processing_time_ms = EXCLUDED.processing_time_ms
		`
		_, err = c.pool.Exec(ctx, query,
			meta.CIK, ticker, meta.CompanyName, meta.FiscalYear, meta.FiscalPeriod,
			meta.Form, meta.AccessionNumber, meta.FilingURL, dataJSON,
			resp.Metadata.LLMProvider, resp.Metadata.VariablesMapped, resp.Metadata.ProcessingTimeMs,
		)
		if err != nil {
			return fmt.Errorf("failed to save to db cache: %w", err)
		}
	}

	// 2. Save to File (Always if configured, or if pool is nil)
	if c.fileDir != "" {
		// Only save successful extractions or partials?
		// We save whatever is passed.
		acc := meta.AccessionNumber
		if acc == "" {
			// Fallback key if missing accession (should not happen in prod)
			acc = fmt.Sprintf("%s_%d_%s", meta.CIK, meta.FiscalYear, meta.Form)
		}

		entry := CacheEntry{
			ID:              acc,
			CIK:             meta.CIK,
			Ticker:          ticker,
			CompanyName:     meta.CompanyName,
			FiscalYear:      meta.FiscalYear,
			FiscalPeriod:    meta.FiscalPeriod,
			FormType:        meta.Form,
			AccessionNumber: acc,
			FilingDate:      meta.FilingDate,
			IsAmendment:     isAmendedForm(meta.Form),
			Data:            resp,
			ExtractedAt:     time.Now(),
			LLMProvider:     resp.Metadata.LLMProvider,
		}

		fileBytes, _ := json.MarshalIndent(entry, "", "  ")
		path := c.accessionPath(acc)
		if err := ioutil.WriteFile(path, fileBytes, 0644); err != nil {
			return fmt.Errorf("failed to save to file cache: %w", err)
		}
	}

	return nil
}

// Exists checks if a filing is already cached
func (c *FSAPCache) Exists(ctx context.Context, accessionNumber string) bool {
	if c.pool != nil {
		query := `SELECT 1 FROM fsap_extractions WHERE accession_number = $1 LIMIT 1`
		var exists int
		err := c.pool.QueryRow(ctx, query, accessionNumber).Scan(&exists)
		if err == nil {
			return true
		}
	}

	if c.fileDir != "" {
		path := c.accessionPath(accessionNumber)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// Internal File Helpers

func (c *FSAPCache) accessionPath(accession string) string {
	// Sanitize accession
	safeAcc := strings.ReplaceAll(accession, "-", "")
	return filepath.Join(c.fileDir, safeAcc+".json")
}

func (c *FSAPCache) loadFromFile(path string) (*edgar.FSAPDataResponse, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, nil // Not found
	}
	// Try parsing as CacheEntry wrapper first
	var entry CacheEntry
	if err := json.Unmarshal(bytes, &entry); err == nil && entry.Data != nil {
		return entry.Data, nil
	}

	// Fallback: maybe it's raw FSAPDataResponse
	var resp edgar.FSAPDataResponse
	if err := json.Unmarshal(bytes, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *FSAPCache) scanFileCache(targetCIK string, targetYear int) (*edgar.FSAPDataResponse, error) {
	files, err := ioutil.ReadDir(c.fileDir)
	if err != nil {
		return nil, nil
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		path := filepath.Join(c.fileDir, f.Name())
		// Parse header only? JSON doesn't support easy header parsing.
		// Read full file.
		entry, err := c.loadEntry(path)
		if err != nil {
			continue
		}
		// Check match
		if entry.CIK == targetCIK && entry.FiscalYear == targetYear {
			// Found it! (Return first match)
			return entry.Data, nil
		}
	}
	return nil, nil
}

func (c *FSAPCache) scanFileCacheByTicker(targetTicker string, targetYear int) (*edgar.FSAPDataResponse, error) {
	files, err := ioutil.ReadDir(c.fileDir)
	if err != nil {
		return nil, nil
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		path := filepath.Join(c.fileDir, f.Name())
		entry, err := c.loadEntry(path)
		if err != nil {
			continue
		}
		// Case insensitive ticker check
		if strings.EqualFold(entry.Ticker, targetTicker) && entry.FiscalYear == targetYear {
			return entry.Data, nil
		}
	}
	return nil, nil
}

func (c *FSAPCache) loadEntry(path string) (*CacheEntry, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(bytes, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// isAmendedForm checks if a form type is an amendment (e.g., 10-K/A, 10-KA, 10-Q/A)
func isAmendedForm(formType string) bool {
	upper := strings.ToUpper(formType)
	return strings.HasSuffix(upper, "/A") || strings.HasSuffix(upper, "A")
}
