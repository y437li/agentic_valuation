package debate

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"fmt"
	"sort"
)

// MaterialPoolBuilder helps assemble the MaterialPool from various sub-agents
type MaterialPoolBuilder struct {
	pool *MaterialPool
	err  error
}

// NewMaterialPoolBuilder initializes the builder with primary FSAP data
// It automatically aggregates history and runs calculation engine for all years
func NewMaterialPoolBuilder(primary *edgar.FSAPDataResponse, history []*edgar.FSAPDataResponse) *MaterialPoolBuilder {
	mp := &MaterialPool{
		FinancialHistory:  make([]*edgar.FSAPDataResponse, 0),
		CommonSizeHistory: make(map[int]*calc.CommonSizeAnalysis),
		ThreeLevelHistory: make(map[int]*calc.ThreeLevelAnalysis),
		ImpliedMetrics:    make(map[int]calc.ImpliedMetrics),
	}

	// 1. Consolidate all available financial data
	allFinancials := make([]*edgar.FSAPDataResponse, 0)
	if history != nil {
		allFinancials = append(allFinancials, history...)
	}

	// Check if primary is already in history (by Year)
	foundPrimary := false
	for _, f := range allFinancials {
		if f.FiscalYear == primary.FiscalYear {
			foundPrimary = true
			break
		}
	}
	if !foundPrimary {
		allFinancials = append(allFinancials, primary)
	}

	// Sort by FiscalYear Ascending
	sort.Slice(allFinancials, func(i, j int) bool {
		return allFinancials[i].FiscalYear < allFinancials[j].FiscalYear
	})

	mp.FinancialHistory = allFinancials // Store valid sorted history

	// 2. Generate Common Size Analysis for EACH year in the pool
	// This ensures we have the "past 5 years common size" compliant with user request.
	for i, current := range allFinancials {
		// derivedHistory is all years *before* current
		derivedHistory := allFinancials[:i]

		analysis := calc.AnalyzeFinancials(current, derivedHistory)
		mp.CommonSizeHistory[current.FiscalYear] = analysis

		// 3. Generate 3-Level Analysis (Growth, Returns, Risk)
		var prior *edgar.FSAPDataResponse
		if i > 0 {
			prior = allFinancials[i-1]
		}
		mp.ThreeLevelHistory[current.FiscalYear] = calc.PerformThreeLevelAnalysis(current, prior)

		// 4. Calculate Implied Technical Metrics (Useful Life, Tax Rate, etc.)
		mp.ImpliedMetrics[current.FiscalYear] = calc.CalculateImpliedMetrics(current)
	}

	return &MaterialPoolBuilder{pool: mp}
}

// WithBusinessStrategy adds the qualitative strategy analysis
func (b *MaterialPoolBuilder) WithBusinessStrategy(s *edgar.StrategyAnalysis) *MaterialPoolBuilder {
	b.pool.BusinessStrategy = s
	return b
}

// WithRiskProfile adds the risk analysis
func (b *MaterialPoolBuilder) WithRiskProfile(r *edgar.RiskAnalysis) *MaterialPoolBuilder {
	b.pool.RiskProfile = r
	return b
}

// WithCapitalAllocation adds capital allocation details
func (b *MaterialPoolBuilder) WithCapitalAllocation(c *edgar.CapitalAnalysis) *MaterialPoolBuilder {
	b.pool.CapitalAllocation = c
	return b
}

// WithQualitativeSegments adds segment analysis (qualitative)
func (b *MaterialPoolBuilder) WithQualitativeSegments(s *edgar.SegmentAnalysis) *MaterialPoolBuilder {
	b.pool.QualitativeSegments = s
	return b
}

// WithQuantitativeSegments adds segment analysis (quantitative)
func (b *MaterialPoolBuilder) WithQuantitativeSegments(s *edgar.SegmentAnalysis) *MaterialPoolBuilder {
	b.pool.QuantitativeSegments = s
	return b
}

// WithMacroTrends adds macro research
func (b *MaterialPoolBuilder) WithMacroTrends(m *MacroResearch) *MaterialPoolBuilder {
	b.pool.MacroTrends = m
	return b
}

// WithMarketSentiment adds sentiment research
func (b *MaterialPoolBuilder) WithMarketSentiment(s *SentimentResearch) *MaterialPoolBuilder {
	b.pool.MarketSentiment = s
	return b
}

// WithTranscripts adds earnings call transcripts
func (b *MaterialPoolBuilder) WithTranscripts(transcripts []Transcript) *MaterialPoolBuilder {
	b.pool.TranscriptHistory = transcripts
	return b
}

// WithNoteData adds extracted notes and normalized table data for assumption generation
func (b *MaterialPoolBuilder) WithNoteData(notes []edgar.ExtractedNote) *MaterialPoolBuilder {
	b.pool.ExtractedNotes = notes
	// Flatten all table rows from notes
	var allRows []edgar.NoteTableRow
	for _, note := range notes {
		for _, table := range note.Tables {
			allRows = append(allRows, table.Rows...)
		}
	}
	b.pool.NoteTableData = allRows
	return b
}

// Build returns the constructed MaterialPool
func (b *MaterialPoolBuilder) Build() (*MaterialPool, error) {
	if b.err != nil {
		return nil, b.err
	}
	if len(b.pool.FinancialHistory) == 0 {
		return nil, fmt.Errorf("material pool requires at least one financial record")
	}
	return b.pool, nil
}
