package analysis

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/synthesis"
	"fmt"
	"sort"
	"time"
)

// AnalysisEngine orchestrates the calculation of financial metrics from a GoldenRecord.
type AnalysisEngine struct{}

// NewAnalysisEngine creates a new instance of the engine.
func NewAnalysisEngine() *AnalysisEngine {
	return &AnalysisEngine{}
}

// Analyze performs the full suite of financial analysis on the Golden Record.
// It iterates through the timeline, generating Common-Size, Ratios, and Growth metrics.
func (e *AnalysisEngine) Analyze(record *synthesis.GoldenRecord) (*CompanyAnalysis, error) {
	if record == nil {
		return nil, fmt.Errorf("golden record is nil")
	}

	analysis := &CompanyAnalysis{
		Ticker:       record.Ticker,
		CIK:          record.CIK,
		LastAnalyzed: time.Now(),
		Timeline:     make(map[int]*YearlyAnalysis),
	}

	// Helper to get a strictly sorted list of years to calculate growth correctly
	var years []int
	for y := range record.Timeline {
		years = append(years, y)
	}
	sort.Ints(years)

	for i, year := range years {
		snapshot := record.Timeline[year]

		// 1. Prepare Data Format (Conversion to FSAPDataResponse style for calc package)
		currentData := &edgar.FSAPDataResponse{
			FiscalYear:        snapshot.FiscalYear,
			BalanceSheet:      snapshot.BalanceSheet,
			IncomeStatement:   snapshot.IncomeStatement,
			CashFlowStatement: snapshot.CashFlowStatement,
			SupplementalData:  snapshot.SupplementalData,
		}

		// 2. Prepare History for Common-Size Trends
		// We need the *previous* years for potential trend analysis if calc uses it.
		var historyData []*edgar.FSAPDataResponse
		for j := 0; j < i; j++ {
			prevYear := years[j]
			prevSnapshot := record.Timeline[prevYear]
			history := &edgar.FSAPDataResponse{
				FiscalYear:        prevSnapshot.FiscalYear,
				BalanceSheet:      prevSnapshot.BalanceSheet,
				IncomeStatement:   prevSnapshot.IncomeStatement,
				CashFlowStatement: prevSnapshot.CashFlowStatement,
				SupplementalData:  prevSnapshot.SupplementalData,
			}
			historyData = append(historyData, history)
		}

		// 3. Run Calculations

		// A. Common-Size
		// AnalyzeFinancials returns *calc.CommonSizeAnalysis
		cs := calc.AnalyzeFinancials(currentData, historyData)

		// B. Three-Level Analysis (Ratios)
		var priorData *edgar.FSAPDataResponse
		if i > 0 {
			priorSnapshot := record.Timeline[years[i-1]]
			priorData = &edgar.FSAPDataResponse{
				FiscalYear:        priorSnapshot.FiscalYear,
				BalanceSheet:      priorSnapshot.BalanceSheet,
				IncomeStatement:   priorSnapshot.IncomeStatement,
				CashFlowStatement: priorSnapshot.CashFlowStatement,
				SupplementalData:  priorSnapshot.SupplementalData,
			}
		}
		// PerformThreeLevelAnalysis returns *calc.ThreeLevelAnalysis
		ratios := calc.PerformThreeLevelAnalysis(currentData, priorData)

		// C. Implied Metrics
		// CalculateImpliedMetrics returns calc.ImpliedMetrics
		implied := calc.CalculateImpliedMetrics(currentData)

		// D. Growth Metrics
		growth := calculateGrowth(currentData, priorData)

		// E. Benford's Law
		benfordVals := e.extractAllValues(currentData)
		benfordRes := calc.AnalyzeBenfordsLaw(benfordVals)

		// 4. Aggregate Result
		analysis.Timeline[year] = &YearlyAnalysis{
			FiscalYear: year,
			CommonSize: cs,
			Ratios:     ratios,
			Implied:    implied,
			Growth:     growth,
			Benford:    &benfordRes, // Take address
		}
	}

	return analysis, nil
}

func calculateGrowth(current, prior *edgar.FSAPDataResponse) GrowthMetrics {
	if prior == nil {
		return GrowthMetrics{}
	}

	g := func(curr, prev *float64) float64 {
		if curr == nil || prev == nil || *prev == 0 {
			return 0
		}
		return (*curr - *prev) / *prev
	}

	metrics := GrowthMetrics{}

	// Revenue
	if current.IncomeStatement.GrossProfitSection != nil && prior.IncomeStatement.GrossProfitSection != nil {
		if current.IncomeStatement.GrossProfitSection.Revenues != nil && prior.IncomeStatement.GrossProfitSection.Revenues != nil {
			metrics.RevenueGrowth = g(current.IncomeStatement.GrossProfitSection.Revenues.Value, prior.IncomeStatement.GrossProfitSection.Revenues.Value)
		}
	}

	// Operating Income
	if current.IncomeStatement.OperatingCostSection != nil && prior.IncomeStatement.OperatingCostSection != nil {
		if current.IncomeStatement.OperatingCostSection.OperatingIncome != nil && prior.IncomeStatement.OperatingCostSection.OperatingIncome != nil {
			metrics.OpIncomeGrowth = g(current.IncomeStatement.OperatingCostSection.OperatingIncome.Value, prior.IncomeStatement.OperatingCostSection.OperatingIncome.Value)
		}
	}

	// Net Income
	if current.IncomeStatement.NetIncomeSection != nil && prior.IncomeStatement.NetIncomeSection != nil {
		if current.IncomeStatement.NetIncomeSection.NetIncomeToCommon != nil && prior.IncomeStatement.NetIncomeSection.NetIncomeToCommon != nil {
			metrics.NetIncomeGrowth = g(current.IncomeStatement.NetIncomeSection.NetIncomeToCommon.Value, prior.IncomeStatement.NetIncomeSection.NetIncomeToCommon.Value)
		}
	}

	// Total Assets
	if current.BalanceSheet.ReportedForValidation.TotalAssets != nil && prior.BalanceSheet.ReportedForValidation.TotalAssets != nil {
		metrics.TotalAssetsGrowth = g(current.BalanceSheet.ReportedForValidation.TotalAssets.Value, prior.BalanceSheet.ReportedForValidation.TotalAssets.Value)
	}

	// Equity
	if current.BalanceSheet.ReportedForValidation.TotalEquity != nil && prior.BalanceSheet.ReportedForValidation.TotalEquity != nil {
		metrics.EquityGrowth = g(current.BalanceSheet.ReportedForValidation.TotalEquity.Value, prior.BalanceSheet.ReportedForValidation.TotalEquity.Value)
	}

	// FCF (requires calculation or extraction, currently using placeholders if explicit field not present)
	// Assuming logic handles this or we skip FCF growth for this simplified version.

	return metrics
}

func (e *AnalysisEngine) extractAllValues(d *edgar.FSAPDataResponse) []float64 {
	var values []float64

	// Helper to append if not nil/zero
	appendVal := func(v *edgar.FSAPValue) {
		if v != nil && v.Value != nil && *v.Value != 0 {
			values = append(values, *v.Value)
		}
	}

	// Balance Sheet
	appendVal(d.BalanceSheet.ReportedForValidation.TotalAssets) // wrapper handling

	// Assets - safely checking for nils
	if d.BalanceSheet.CurrentAssets.CashAndEquivalents != nil {
		appendVal(d.BalanceSheet.CurrentAssets.CashAndEquivalents)
	}
	if d.BalanceSheet.CurrentAssets.ShortTermInvestments != nil {
		appendVal(d.BalanceSheet.CurrentAssets.ShortTermInvestments)
	}
	if d.BalanceSheet.CurrentAssets.AccountsReceivableNet != nil {
		appendVal(d.BalanceSheet.CurrentAssets.AccountsReceivableNet)
	}
	if d.BalanceSheet.CurrentAssets.Inventories != nil {
		appendVal(d.BalanceSheet.CurrentAssets.Inventories)
	}

	if d.BalanceSheet.NoncurrentAssets.PPENet != nil {
		appendVal(d.BalanceSheet.NoncurrentAssets.PPENet)
	}
	if d.BalanceSheet.NoncurrentAssets.Goodwill != nil {
		appendVal(d.BalanceSheet.NoncurrentAssets.Goodwill)
	}
	if d.BalanceSheet.NoncurrentAssets.Intangibles != nil {
		appendVal(d.BalanceSheet.NoncurrentAssets.Intangibles)
	}

	// Liabilities & Equity
	if d.BalanceSheet.CurrentLiabilities.AccountsPayable != nil {
		appendVal(d.BalanceSheet.CurrentLiabilities.AccountsPayable)
	}
	if d.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt != nil {
		appendVal(d.BalanceSheet.CurrentLiabilities.NotesPayableShortTermDebt)
	}
	if d.BalanceSheet.NoncurrentLiabilities.LongTermDebt != nil {
		appendVal(d.BalanceSheet.NoncurrentLiabilities.LongTermDebt)
	}

	if d.BalanceSheet.Equity.RetainedEarningsDeficit != nil {
		appendVal(d.BalanceSheet.Equity.RetainedEarningsDeficit)
	}
	if d.BalanceSheet.ReportedForValidation.TotalEquity != nil {
		appendVal(d.BalanceSheet.ReportedForValidation.TotalEquity)
	}

	// Income Statement
	if d.IncomeStatement.GrossProfitSection != nil {
		if d.IncomeStatement.GrossProfitSection.Revenues != nil {
			appendVal(d.IncomeStatement.GrossProfitSection.Revenues)
		}
		if d.IncomeStatement.GrossProfitSection.CostOfGoodsSold != nil {
			appendVal(d.IncomeStatement.GrossProfitSection.CostOfGoodsSold)
		}
		if d.IncomeStatement.GrossProfitSection.GrossProfit != nil {
			appendVal(d.IncomeStatement.GrossProfitSection.GrossProfit)
		}
	}

	if d.IncomeStatement.OperatingCostSection != nil {
		if d.IncomeStatement.OperatingCostSection.SGAExpenses != nil {
			appendVal(d.IncomeStatement.OperatingCostSection.SGAExpenses)
		}
		if d.IncomeStatement.OperatingCostSection.RDExpenses != nil {
			appendVal(d.IncomeStatement.OperatingCostSection.RDExpenses)
		}
		if d.IncomeStatement.OperatingCostSection.OperatingIncome != nil {
			appendVal(d.IncomeStatement.OperatingCostSection.OperatingIncome)
		}
	}

	if d.IncomeStatement.NetIncomeSection != nil {
		if d.IncomeStatement.NetIncomeSection.NetIncomeToCommon != nil {
			appendVal(d.IncomeStatement.NetIncomeSection.NetIncomeToCommon)
		}
	}

	// Cash Flow - add nil safety for CashSummary
	if d.CashFlowStatement.CashSummary != nil {
		if d.CashFlowStatement.CashSummary.NetCashOperating != nil {
			appendVal(d.CashFlowStatement.CashSummary.NetCashOperating)
		}
		if d.CashFlowStatement.CashSummary.NetCashInvesting != nil {
			appendVal(d.CashFlowStatement.CashSummary.NetCashInvesting)
		}
		if d.CashFlowStatement.CashSummary.NetCashFinancing != nil {
			appendVal(d.CashFlowStatement.CashSummary.NetCashFinancing)
		}
	}

	return values
}
