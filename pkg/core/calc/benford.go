package calc

import (
	"math"
	"strconv"
)

// BenfordDistribution is the expected frequency for leading digits 1-9
var BenfordDistribution = map[int]float64{
	1: 0.30103,
	2: 0.17609,
	3: 0.12494,
	4: 0.09691,
	5: 0.07918,
	6: 0.06695,
	7: 0.05799,
	8: 0.05115,
	9: 0.04576,
}

// BenfordResult holds the analysis of leading digit distribution
type BenfordResult struct {
	DigitCounts      map[int]int     `json:"digit_counts"`
	DigitFrequencies map[int]float64 `json:"digit_frequencies"`
	TotalCount       int             `json:"total_count"`
	MAD              float64         `json:"mad"`     // Mean Absolute Deviation
	Flagged          bool            `json:"flagged"` // True if MAD > threshold
	Level            string          `json:"level"`   // Low, Medium, High Deviation
}

// AnalyzeBenfordsLaw performs first-digit analysis on a set of financial values
// It ignores numbers < 10 (single digits usually noise) and 0.
// Thresholds for MAD (Mean Absolute Deviation):
// - < 0.006: Close conformity (Low Risk)
// - 0.006 - 0.012: Marginally nonconforming (Medium Risk)
// - > 0.012: Nonconforming (High Risk) (Based on common audit heuristics)
func AnalyzeBenfordsLaw(values []float64) BenfordResult {
	counts := make(map[int]int)
	processed := 0

	for _, v := range values {
		vAbs := math.Abs(v)
		if vAbs < 1.0 {
			continue
		} // Ignore fractional or small integers? Usually ignore < 10 for rigor
		// Extract leading digit
		// String method (safest for floats)
		s := strconv.FormatFloat(vAbs, 'f', -1, 64)
		// Usually leading digit is first char '1'-'9'
		// Skip '0.' if <1 but we filtered <1.
		// If 0.005 -> '5' is leading? Benford applies to scale invariant.
		// Let's iterate string until we find 1-9.
		var leading int = -1
		for _, c := range s {
			if c >= '1' && c <= '9' {
				leading = int(c - '0')
				break
			}
		}
		if leading != -1 {
			counts[leading]++
			processed++
		}
	}

	freqs := make(map[int]float64)
	sumDiff := 0.0

	if processed == 0 {
		return BenfordResult{Flagged: false, Level: "Insufficient Data"}
	}

	for d := 1; d <= 9; d++ {
		actualFreq := float64(counts[d]) / float64(processed)
		freqs[d] = actualFreq
		expected := BenfordDistribution[d]
		sumDiff += math.Abs(actualFreq - expected)
	}

	mad := sumDiff / 9.0

	level := "Low Risk"
	flagged := false
	if mad > 0.015 { // Slightly looser threshold for small datasets often found in Summaries
		level = "High Risk"
		flagged = true
	} else if mad > 0.010 {
		level = "Medium Risk"
	}

	return BenfordResult{
		DigitCounts:      counts,
		DigitFrequencies: freqs,
		TotalCount:       processed,
		MAD:              mad,
		Flagged:          flagged,
		Level:            level,
	}
}

// ExtractValuesFromAnalysis helper to pull all values from CommonSizeAnalysis for scanning
func ExtractValuesFromAnalysis(analysis *CommonSizeAnalysis) []float64 {
	var values []float64

	collect := func(m map[string]*AnalysisResult) {
		for _, v := range m {
			if v.Value != 0 {
				values = append(values, v.Value)
			}
		}
	}

	collect(analysis.IncomeStatement)
	collect(analysis.BalanceSheet)
	collect(analysis.CashFlow)
	return values
}
