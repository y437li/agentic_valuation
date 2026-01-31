package debate

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/store"
	"context"
	"fmt"
	"sort"
)

// MaterialPoolLoader is responsible for re-hydrating a MaterialPool from persistent storage.
// This bypasses the need for re-extraction.
type MaterialPoolLoader struct {
	repo *store.AnalysisRepo
}

// NewMaterialPoolLoader creates a new loader.
func NewMaterialPoolLoader(repo *store.AnalysisRepo) *MaterialPoolLoader {
	return &MaterialPoolLoader{repo: repo}
}

// LoadFromDB fetches the GoldenRecord from DB and constructs a MaterialPool.
func (l *MaterialPoolLoader) LoadFromDB(ctx context.Context, ticker string) (*MaterialPool, error) {
	// 1. Load from DB
	record, anal, err := l.repo.Load(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to load from db: %w", err)
	}

	// 2. Convert GoldenRecord Timeline -> []*edgar.FSAPDataResponse
	history := make([]*edgar.FSAPDataResponse, 0)

	var years []int
	for y := range record.Timeline {
		years = append(years, y)
	}
	sort.Ints(years) // Ensure chronological order

	for _, year := range years {
		snapshot := record.Timeline[year]
		fsapResp := &edgar.FSAPDataResponse{
			FiscalYear:        snapshot.FiscalYear,
			BalanceSheet:      snapshot.BalanceSheet,
			IncomeStatement:   snapshot.IncomeStatement,
			CashFlowStatement: snapshot.CashFlowStatement,
			SupplementalData:  snapshot.SupplementalData,
			// We can reconstruct other fields if needed, like Metadata
		}
		history = append(history, fsapResp)
	}

	if len(history) == 0 {
		return nil, fmt.Errorf("no history found in loaded record")
	}

	// 3. Initialize Material Pool
	// We can treat the latest year as primary and the rest as history,
	// or the builder handles full history aggregation.

	// We trust the persisted analysis

	mp := &MaterialPool{
		FinancialHistory:  history,
		CommonSizeHistory: make(map[int]*calc.CommonSizeAnalysis),
		ThreeLevelHistory: make(map[int]*calc.ThreeLevelAnalysis),
		ImpliedMetrics:    make(map[int]calc.ImpliedMetrics),
	}

	// Inject pre-calculated analysis
	for y, yearlyAnal := range anal.Timeline {
		if yearlyAnal.CommonSize != nil {
			mp.CommonSizeHistory[y] = yearlyAnal.CommonSize
		}
		if yearlyAnal.Ratios != nil {
			mp.ThreeLevelHistory[y] = yearlyAnal.Ratios
		}
		mp.ImpliedMetrics[y] = yearlyAnal.Implied
	}

	return mp, nil
}
