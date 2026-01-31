package store

import (
	"agentic_valuation/pkg/core/analysis"
	"agentic_valuation/pkg/core/synthesis"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// AnalysisRepo handles the storage of GoldenRecords and Analysis results.
type AnalysisRepo struct{}

// NewAnalysisRepo creates a new repository instance.
func NewAnalysisRepo() *AnalysisRepo {
	return &AnalysisRepo{}
}

// Save persists the synthesized GoldenRecord and its analysis.
// It uses an upsert strategy based on Ticker.
func (r *AnalysisRepo) Save(ctx context.Context, record *synthesis.GoldenRecord, anal *analysis.CompanyAnalysis) error {
	pool := GetPool()
	if pool == nil {
		return fmt.Errorf("database pool not initialized")
	}

	// Prepare data for JSONB column
	data := struct {
		GoldenRecord    *synthesis.GoldenRecord   `json:"golden_record"`
		CompanyAnalysis *analysis.CompanyAnalysis `json:"company_analysis"`
	}{
		GoldenRecord:    record,
		CompanyAnalysis: anal,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Upsert query
	// Ideally we should have separate columns but a single JSONB blob is flexible for now.
	// We assume a table `financial_analysis` exists.
	// If not, we might need to create it or assume schema is managed elsewhere (migrations).

	// Schema assumption:
	// CREATE TABLE IF NOT EXISTS financial_analysis (
	//   ticker TEXT PRIMARY KEY,
	//   cik TEXT,
	//   analysis_json JSONB,
	//   updated_at TIMESTAMPTZ
	// );

	query := `
		INSERT INTO financial_analysis (ticker, cik, analysis_json, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ticker) 
		DO UPDATE SET 
			cik = EXCLUDED.cik,
			analysis_json = EXCLUDED.analysis_json,
			updated_at = EXCLUDED.updated_at;
	`

	_, err = pool.Exec(ctx, query, record.Ticker, record.CIK, jsonData, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save analysis: %w", err)
	}

	return nil
}

// Load retrieves the full analysis (Record + Metrics) for a ticker.
func (r *AnalysisRepo) Load(ctx context.Context, ticker string) (*synthesis.GoldenRecord, *analysis.CompanyAnalysis, error) {
	pool := GetPool()
	if pool == nil {
		return nil, nil, fmt.Errorf("database pool not initialized")
	}

	query := `SELECT analysis_json FROM financial_analysis WHERE ticker = $1`

	var jsonData []byte
	err := pool.QueryRow(ctx, query, ticker).Scan(&jsonData)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, fmt.Errorf("no analysis found for ticker %s", ticker)
		}
		return nil, nil, fmt.Errorf("failed to load analysis: %w", err)
	}

	var data struct {
		GoldenRecord    *synthesis.GoldenRecord   `json:"golden_record"`
		CompanyAnalysis *analysis.CompanyAnalysis `json:"company_analysis"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal analysis data: %w", err)
	}

	return data.GoldenRecord, data.CompanyAnalysis, nil
}
