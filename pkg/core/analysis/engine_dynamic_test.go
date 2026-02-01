package analysis

import (
	"agentic_valuation/pkg/core/edgar"
	"agentic_valuation/pkg/core/synthesis"
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"testing"
	"time"
)

// Helper to generate a random float pointer
func floatPtr(v float64) *float64 {
	return &v
}

// CompanyProfile defines the characteristics of the synthetic data to generate
type CompanyProfile struct {
	Ticker      string
	Type        string // "Healthy", "MissingData", "ZeroValues", "BrokenPointer"
	BaseRevenue float64
}

// GenerateSyntheticGoldenRecord creates a GoldenRecord based on the profile
func GenerateSyntheticGoldenRecord(profile CompanyProfile, years []int) *synthesis.GoldenRecord {
	record := &synthesis.GoldenRecord{
		Ticker:   profile.Ticker,
		CIK:      fmt.Sprintf("CIK-%s", profile.Ticker),
		Timeline: make(map[int]*synthesis.YearlySnapshot), // Correct type: YearlySnapshot
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	baseRevenue := profile.BaseRevenue
	if baseRevenue == 0 && profile.Type != "ZeroValues" {
		baseRevenue = 1000.0
	}

	for i, year := range years {
		// Simulate growth/fluctuation
		growthFactor := 1.0 + (r.Float64()*0.2 - 0.05)
		if i > 0 {
			baseRevenue *= growthFactor
		}

		revenue := baseRevenue
		cogs := revenue * 0.5
		grossProfit := revenue - cogs
		sga := revenue * 0.15
		opIncome := grossProfit - sga
		netIncome := opIncome * 0.75
		assets := revenue * 1.2
		equity := assets * 0.4
		cash := assets * 0.1
		opCash := opIncome * 1.1

		// Apply defects based on Type
		if profile.Type == "ZeroValues" {
			revenue = 0
			cogs = 0
			grossProfit = 0
			sga = 0
			opIncome = 0
			netIncome = 0
			assets = 0
			equity = 0
			cash = 0
			opCash = 0
		}

		// Construct Snapshot
		snapshot := &synthesis.YearlySnapshot{
			FiscalYear: year,
			IncomeStatement: edgar.IncomeStatement{
				GrossProfitSection: &edgar.GrossProfitSection{
					Revenues:        &edgar.FSAPValue{Value: floatPtr(revenue)},
					CostOfGoodsSold: &edgar.FSAPValue{Value: floatPtr(cogs)},
					GrossProfit:     &edgar.FSAPValue{Value: floatPtr(grossProfit)},
				},
				OperatingCostSection: &edgar.OperatingCostSection{
					SGAExpenses:     &edgar.FSAPValue{Value: floatPtr(sga)},
					OperatingIncome: &edgar.FSAPValue{Value: floatPtr(opIncome)},
				},
				NetIncomeSection: &edgar.NetIncomeSection{
					NetIncomeToCommon: &edgar.FSAPValue{Value: floatPtr(netIncome)},
				},
			},
			BalanceSheet: edgar.BalanceSheet{
				ReportedForValidation: edgar.ReportedForValidation{
					TotalAssets: &edgar.FSAPValue{Value: floatPtr(assets)},
					TotalEquity: &edgar.FSAPValue{Value: floatPtr(equity)},
				},
				CurrentAssets: edgar.CurrentAssets{
					CashAndEquivalents: &edgar.FSAPValue{Value: floatPtr(cash)},
				},
			},
			CashFlowStatement: edgar.CashFlowStatement{
				CashSummary: &edgar.CashSummarySection{ // Corrected type pointer
					NetCashOperating: &edgar.FSAPValue{Value: floatPtr(opCash)},
				},
			},
		}

		// Apply structural defects
		if profile.Type == "MissingData" {
			// Simulate missing Income Statement completely
			snapshot.IncomeStatement = edgar.IncomeStatement{}
		}
		if profile.Type == "BrokenPointer" {
			// Nil out a section that might cause panic if not checked
			snapshot.IncomeStatement.GrossProfitSection = nil
		}

		record.Timeline[year] = snapshot
	}

	return record
}

// MockSaver simulates saving to a database
func MockSaver(ticker string, result *CompanyAnalysis) error {
	// Simulate DB latency
	// time.Sleep(1 * time.Millisecond)
	if ticker == "DBFAIL" {
		return fmt.Errorf("database connection refused")
	}
	return nil
}

func TestEngine_DynamicCompanyBatch(t *testing.T) {
	// 1. Setup Report
	reportFile, err := os.Create("engine_test_report.txt")
	if err != nil {
		t.Fatalf("Failed to create report file: %v", err)
	}
	defer reportFile.Close()

	logMsg := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		fmt.Println(msg) // Enable stdout for debugging
		reportFile.WriteString(msg + "\n")
	}

	logMsg("--- Dynamic Analysis Engine Test Report ---")
	logMsg("Time: %s", time.Now().Format(time.RFC3339))
	logMsg("--------------------------------------------------------------------------------------")
	logMsg("%-10s | %-12s | %-10s | %s", "TICKER", "TYPE", "STATUS", "DETAILS")
	logMsg("--------------------------------------------------------------------------------------")

	// 2. Define Companies List (Dynamic generation + specific edge cases)
	var profiles []CompanyProfile

	// A. Healthy Batch (50 companies)
	for i := 1; i <= 50; i++ {
		profiles = append(profiles, CompanyProfile{
			Ticker:      fmt.Sprintf("AAPL%02d", i),
			Type:        "Healthy",
			BaseRevenue: float64(i * 1000),
		})
	}

	// B. Edge Case: Missing Data (10 companies)
	for i := 1; i <= 10; i++ {
		profiles = append(profiles, CompanyProfile{
			Ticker: fmt.Sprintf("MISS%02d", i),
			Type:   "MissingData",
		})
	}

	// C. Edge Case: Zero Values (10 companies)
	for i := 1; i <= 10; i++ {
		profiles = append(profiles, CompanyProfile{
			Ticker: fmt.Sprintf("ZERO%02d", i),
			Type:   "ZeroValues",
		})
	}

	// D. Edge Case: Broken Pointers (10 companies)
	for i := 1; i <= 10; i++ {
		profiles = append(profiles, CompanyProfile{
			Ticker: fmt.Sprintf("NULL%02d", i),
			Type:   "BrokenPointer",
		})
	}

	// E. Edge Case: DB Failure Simulation
	profiles = append(profiles, CompanyProfile{
		Ticker: "DBFAIL",
		Type:   "Healthy",
	})

	engine := NewAnalysisEngine()
	successCount := 0
	failCount := 0

	// 3. Test Loop
	for _, p := range profiles {
		years := []int{2022, 2023, 2024}
		record := GenerateSyntheticGoldenRecord(p, years)

		// A. Run Analysis
		// We use defer/recover to catch panics from BrokenPointer cases if the engine isn't robust
		func() {
			defer func() {
				if r := recover(); r != nil {
					failCount++
					logMsg("%-10s | %-12s | %-10s | %v (PANIC)", p.Ticker, p.Type, "FAIL", r)
					fmt.Printf("Stack Trace for %s:\n%s\n", p.Ticker, debug.Stack())
				}
			}()

			result, err := engine.Analyze(record)

			if err != nil {
				failCount++
				logMsg("%-10s | %-12s | %-10s | Analysis Error: %v", p.Ticker, p.Type, "FAIL", err)
				return
			}

			// Validation of Result
			if result == nil || len(result.Timeline) == 0 {
				failCount++
				logMsg("%-10s | %-12s | %-10s | Result is empty", p.Ticker, p.Type, "FAIL")
				return
			}

			// B. Save to Database (Mock)
			err = MockSaver(p.Ticker, result)
			if err != nil {
				// Note: In a real pipeline, save failure is a failure.
				// Here we count it as a fail for the test report.
				failCount++
				logMsg("%-10s | %-12s | %-10s | DB Save Error: %v", p.Ticker, p.Type, "FAIL", err)
				return
			}

			successCount++
			// Extract a metric for the log
			lastYear := result.Timeline[2024]
			revGrowth := 0.0
			if lastYear != nil {
				revGrowth = lastYear.Growth.RevenueGrowth * 100
			}
			logMsg("%-10s | %-12s | %-10s | Saved to DB. RevGrowth: %.2f%%", p.Ticker, p.Type, "PASS", revGrowth)
		}()
	}

	logMsg("--------------------------------------------------------------------------------------")
	logMsg("Summary:")
	logMsg("Total Processed: %d", len(profiles))
	logMsg("Success: %d", successCount)
	logMsg("Failures: %d", failCount)
	logMsg("--------------------------------------------------------------------------------------")

	// Fail the test if significant failures occurred (optional, depending on strictness)
	// For "BrokenPointer", we expect failures or robust handling.
	// If the engine handles nils gracefully, failCount might be lower.
	if failCount > 20 { // Allow some failures for edge cases, but not too many
		t.Errorf("Too many failures: %d", failCount)
	}
}
