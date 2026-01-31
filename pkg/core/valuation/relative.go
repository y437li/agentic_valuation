package valuation

import (
	"sort"
)

// MetricInput holds the target company's current metrics (LTM or NTM)
type MetricInput struct {
	Revenue   float64
	EBITDA    float64
	NetIncome float64
	NetDebt   float64
	SharesOut float64
}

// PeerComparable represents a comparable company or transaction
type PeerComparable struct {
	Name          string
	EV_Revenue    float64
	EV_EBITDA     float64
	PE_Ratio      float64
	IsTransaction bool // True for Precedent Transaction, False for Trading Comp
}

// RelativeValuationResult holds the valuation range derived from multiples
type RelativeValuationResult struct {
	ImpliedEV_Revenue [2]float64 // Low, High
	ImpliedEV_EBITDA  [2]float64
	ImpliedPE_Price   [2]float64
	CompositePrice    [2]float64 // Weighted or specific logic
}

// CalculateComps performs Comparable Companies Analysis
func CalculateComps(target MetricInput, peers []PeerComparable) RelativeValuationResult {
	return calculateMultiples(target, peers, false)
}

// CalculateTransactions performs Precedent Transaction Analysis
// Usually involves a control premium, so multiples are higher.
func CalculateTransactions(target MetricInput, peers []PeerComparable) RelativeValuationResult {
	return calculateMultiples(target, peers, true)
}

func calculateMultiples(target MetricInput, peers []PeerComparable, onlyTransactions bool) RelativeValuationResult {
	var revMults, ebitdaMults, peMults []float64

	for _, p := range peers {
		if p.IsTransaction != onlyTransactions {
			continue
		}
		if p.EV_Revenue > 0 {
			revMults = append(revMults, p.EV_Revenue)
		}
		if p.EV_EBITDA > 0 {
			ebitdaMults = append(ebitdaMults, p.EV_EBITDA)
		}
		if p.PE_Ratio > 0 {
			peMults = append(peMults, p.PE_Ratio)
		}
	}

	res := RelativeValuationResult{}

	// Helper to get ranges (25th - 75th percentile)
	getRange := func(mults []float64) (float64, float64) {
		if len(mults) == 0 {
			return 0, 0
		}
		sort.Float64s(mults)
		lowIdx := int(float64(len(mults)) * 0.25)
		highIdx := int(float64(len(mults)) * 0.75)
		if highIdx >= len(mults) {
			highIdx = len(mults) - 1
		}
		return mults[lowIdx], mults[highIdx]
	}

	// EV/Revenue Implied EV
	rLo, rHi := getRange(revMults)
	res.ImpliedEV_Revenue = [2]float64{rLo * target.Revenue, rHi * target.Revenue}

	// EV/EBITDA Implied EV
	eLo, eHi := getRange(ebitdaMults)
	res.ImpliedEV_EBITDA = [2]float64{eLo * target.EBITDA, eHi * target.EBITDA}

	// P/E Implied Price (Direct Equity Value)
	pLo, pHi := getRange(peMults)
	res.ImpliedPE_Price = [2]float64{pLo * target.NetIncome / target.SharesOut, pHi * target.NetIncome / target.SharesOut}

	return res
}
