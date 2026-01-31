package projection

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"math"
)

// ProjectionEngine handles the articulation of financial statements for future periods.
// It ensures that Income Statement, Cash Flow, and Balance Sheet are mathematically consistent.
type ProjectionEngine struct {
	Skeleton *StandardSkeleton
}

// NewProjectionEngine creates a new articulation engine
func NewProjectionEngine(skeleton *StandardSkeleton) *ProjectionEngine {
	return &ProjectionEngine{
		Skeleton: skeleton,
	}
}

// ProjectYear calculates T+1 financials based on T-0 (history) and assumptions
func (e *ProjectionEngine) ProjectYear(
	prevIS *edgar.IncomeStatement,
	prevBS *edgar.BalanceSheet,
	prevSegments []edgar.StandardizedSegment, // Optional: Supports SOTP
	assumptions ProjectionAssumptions,
	targetYear int,
) *ProjectedFinancials {

	// 1. Income Statement
	projIS, projSegments, projNI, projDividends, projRev := e.projectIncomeStatement(prevIS, prevBS, prevSegments, assumptions)

	// Extract COGS for BS drivers (Inventory/AP often drive off COGS)
	projCOGS := getValue(projIS.GrossProfitSection.CostOfGoodsSold)

	// 2. Balance Sheet
	projBS, revolverNeeded, projDep, projCapex, projSBC := e.projectBalanceSheet(prevBS, assumptions, projRev, projCOGS, projNI, projDividends)

	// 3. Cash Flow
	projCF := e.projectCashFlow(prevBS, projBS, projNI, projDep, projCapex, projSBC, revolverNeeded, projDividends)

	return &ProjectedFinancials{
		Year:            targetYear,
		IncomeStatement: projIS,
		BalanceSheet:    projBS,
		CashFlow:        projCF,
		Segments:        projSegments,
	}
}

// projectIncomeStatement calculates the projected Income Statement
func (e *ProjectionEngine) projectIncomeStatement(
	prevIS *edgar.IncomeStatement,
	prevBS *edgar.BalanceSheet,
	prevSegments []edgar.StandardizedSegment,
	assumptions ProjectionAssumptions,
) (*edgar.IncomeStatement, []edgar.StandardizedSegment, float64, float64, float64) {

	// -------------------------------------------------------------------------
	// Revenue Logic: SOTP vs Aggregate
	// -------------------------------------------------------------------------
	prevRev := getValue(prevIS.GrossProfitSection.Revenues)
	projRev := 0.0
	projSegments := make([]edgar.StandardizedSegment, 0)

	// If we have Segment Growth Drivers AND Previous Segments, use SOTP
	if len(assumptions.SegmentGrowth) > 0 && len(prevSegments) > 0 {
		totalSegRev := 0.0
		for _, seg := range prevSegments {
			growth, ok := assumptions.SegmentGrowth[seg.Name]
			if !ok {
				growth = assumptions.RevenueGrowth // Fallback to aggregate
			}
			prevSegRev := getValue(seg.Revenues)
			newSegRev := prevSegRev * (1 + growth)

			// Create projected segment
			newSeg := seg
			newVal := newSegRev
			newSeg.Revenues = &edgar.FSAPValue{Value: &newVal}
			projSegments = append(projSegments, newSeg)

			totalSegRev += newSegRev
		}
		// If segments explain most revenue (>80%), use SOTP sum.
		// Otherwise, scaling might be needed, but for now Trust the Segments.
		projRev = totalSegRev
	} else {
		// Aggregate Growth
		projRev = prevRev * (1 + assumptions.RevenueGrowth)
	}

	// COGS
	projCOGS := -(projRev * assumptions.COGSPercent) // Negative expense

	// Gross Profit
	projGP := projRev + projCOGS

	// OpEx
	var projSelling, projAdmin, projSGA float64

	if assumptions.SellingMarketingPercent != 0 || assumptions.GeneralAdminPercent != 0 {
		// Granular Build-up
		projSelling = -(projRev * assumptions.SellingMarketingPercent)
		projAdmin = -(projRev * assumptions.GeneralAdminPercent)
		projSGA = projSelling + projAdmin
	} else {
		// Aggregate Fallback
		projSGA = -(projRev * assumptions.SGAPercent)
	}

	projRD := -(projRev * assumptions.RDPercent)
	projOpInc := projGP + projSGA + projRD

	// -------------------------------------------------------------------------
	// Interest Expense & Income (Granular)
	// -------------------------------------------------------------------------
	// Debt Interest
	prevLTD := getValue(prevBS.NoncurrentLiabilities.LongTermDebt)
	prevSTDebt := getValue(prevBS.CurrentLiabilities.NotesPayableShortTermDebt)
	totalDebt := prevLTD + prevSTDebt

	interestRate := assumptions.DebtInterestRate
	if interestRate == 0 {
		interestRate = assumptions.PreTaxCostOfDebt // Fallback to WACC component
	}

	// If still 0, create a floor based on history?
	// For now, let's respect the 0 if explicit, otherwise fallback to history implied
	prevInterestExp := getValue(prevIS.NonOperatingSection.InterestExpense)
	if interestRate == 0 && totalDebt > 0 {
		// Implied rate from history
		impliedRate := math.Abs(prevInterestExp) / totalDebt
		if impliedRate > 0 {
			interestRate = impliedRate
		}
	}
	projInterestExp := -(totalDebt * interestRate)

	// Cash Interest
	prevCash := getValue(prevBS.CurrentAssets.CashAndEquivalents)
	projInterestInc := prevCash * assumptions.CashInterestRate

	// Net Interest
	projNetInterest := projInterestExp + projInterestInc

	// Pre-Tax Income
	projEBT := projOpInc + projNetInterest

	// Tax
	projTax := -(projEBT * assumptions.TaxRate)

	// Net Income (Attrib to all)
	projNI := projEBT + projTax

	// Dividends (New)
	projDividends := projNI * assumptions.DividendPayoutRatio

	// EPS Calculation
	shares := assumptions.SharesOutstanding
	if shares == 0 {
		shares = 100.0 // Default to avoid division by zero or use specific fallback
	}
	projEPS := 0.0
	if shares != 0 {
		projEPS = projNI / shares
	}

	// Construct Projected IS
	projIS := &edgar.IncomeStatement{
		GrossProfitSection: &edgar.GrossProfitSection{
			Revenues:        &edgar.FSAPValue{Value: &projRev},
			CostOfGoodsSold: &edgar.FSAPValue{Value: &projCOGS},
			GrossProfit:     &edgar.FSAPValue{Value: &projGP},
		},
		OperatingCostSection: &edgar.OperatingCostSection{
			SGAExpenses:      &edgar.FSAPValue{Value: &projSGA}, // Total
			SellingMarketing: &edgar.FSAPValue{Value: &projSelling},
			GeneralAdmin:     &edgar.FSAPValue{Value: &projAdmin},
			RDExpenses:       &edgar.FSAPValue{Value: &projRD},
			OperatingIncome:  &edgar.FSAPValue{Value: &projOpInc},
		},
		NonOperatingSection: &edgar.NonOperatingSection{
			InterestExpense: &edgar.FSAPValue{Value: &projNetInterest}, // Net for now in this slot
			IncomeBeforeTax: &edgar.FSAPValue{Value: &projEBT},
		},
		TaxAdjustments: &edgar.TaxAdjustmentsSection{
			IncomeTaxExpense: &edgar.FSAPValue{Value: &projTax},
		},
		NetIncomeSection: &edgar.NetIncomeSection{
			NetIncomeToCommon:     &edgar.FSAPValue{Value: &projNI},
			EPSBasic:              &edgar.FSAPValue{Value: &projEPS},
			EPSDiluted:            &edgar.FSAPValue{Value: &projEPS},
			WeightedAverageShares: &edgar.FSAPValue{Value: &shares},
		},
	}

	// -------------------------------------------------------------------------
	// Dynamic Item Projection (NodeDrivers)
	// -------------------------------------------------------------------------
	// Project AdditionalItems using NodeDrivers map (% of Revenue).
	if assumptions.NodeDrivers != nil {
		for key, pct := range assumptions.NodeDrivers {
			projValue := projRev * pct
			projValuePtr := new(float64)
			*projValuePtr = projValue

			switch {
			// === Income Statement ===
			case len(key) > 15 && key[:15] == "IS-GrossProfit:":
				label := key[16:] // Skip "IS-GrossProfit: "
				projIS.GrossProfitSection.AdditionalItems = append(
					projIS.GrossProfitSection.AdditionalItems,
					edgar.AdditionalItem{Label: label, Value: &edgar.FSAPValue{Value: projValuePtr}},
				)
			case len(key) > 10 && key[:10] == "IS-OpCost:":
				label := key[11:]
				projIS.OperatingCostSection.AdditionalItems = append(
					projIS.OperatingCostSection.AdditionalItems,
					edgar.AdditionalItem{Label: label, Value: &edgar.FSAPValue{Value: projValuePtr}},
				)
			case len(key) > 9 && key[:9] == "IS-NonOp:":
				label := key[10:]
				projIS.NonOperatingSection.AdditionalItems = append(
					projIS.NonOperatingSection.AdditionalItems,
					edgar.AdditionalItem{Label: label, Value: &edgar.FSAPValue{Value: projValuePtr}},
				)
			case len(key) > 7 && key[:7] == "IS-Tax:":
				label := key[8:]
				projIS.TaxAdjustments.AdditionalItems = append(
					projIS.TaxAdjustments.AdditionalItems,
					edgar.AdditionalItem{Label: label, Value: &edgar.FSAPValue{Value: projValuePtr}},
				)
			}
		}
	}

	return projIS, projSegments, projNI, projDividends, projRev
}

// projectBalanceSheet calculates the projected Balance Sheet
func (e *ProjectionEngine) projectBalanceSheet(
	prevBS *edgar.BalanceSheet,
	assumptions ProjectionAssumptions,
	projRev float64,
	projCOGS float64, // Needed for DSI/DPO
	projNI float64,
	projDividends float64,
) (*edgar.BalanceSheet, float64, float64, float64, float64) {

	// -------------------------------------------------------------------------
	// A. Current Assets (Drivers)
	// -------------------------------------------------------------------------
	var projAR float64
	if assumptions.ReceivablesPercent != 0 {
		projAR = projRev * assumptions.ReceivablesPercent
	} else {
		projAR = (projRev / 365.0) * assumptions.DSO
	}

	var projInv float64
	if assumptions.InventoryPercent != 0 {
		// Assuming InventoryPercent is % of REVENUE (consistent with common_size)
		projInv = projRev * assumptions.InventoryPercent
	} else {
		projInv = (-projCOGS / 365.0) * assumptions.DSI
	}

	// Rollforward Other Current Assets
	prevOtherCA := getValue(prevBS.CurrentAssets.OtherCurrentAssets)
	prevSTInvest := getValue(prevBS.CurrentAssets.ShortTermInvestments)

	projOtherCA := prevOtherCA
	projSTInvest := prevSTInvest

	// -------------------------------------------------------------------------
	// B. Non-Current Assets (PPE + Rollforwards)
	// -------------------------------------------------------------------------
	// PPE
	prevPPEAtCost := getValue(prevBS.NoncurrentAssets.PPEAtCost)
	prevAccumDep := math.Abs(getValue(prevBS.NoncurrentAssets.AccumulatedDepreciation))
	prevPPENet := getValue(prevBS.NoncurrentAssets.PPENet)

	if prevPPEAtCost == 0 && prevPPENet > 0 {
		prevPPEAtCost = prevPPENet
	}

	// Depreciation
	projDep := 0.0
	if assumptions.UsefulLifeForecast > 0 {
		projDep = prevPPEAtCost / assumptions.UsefulLifeForecast
	} else if assumptions.DepreciationPercent > 0 {
		projDep = prevPPEAtCost * assumptions.DepreciationPercent
	} else {
		projDep = projRev * 0.03 // Fallback
	}

	projCapex := -(projRev * assumptions.CapexPercent)

	projPPEAtCost := prevPPEAtCost + math.Abs(projCapex)
	projAccumDep := prevAccumDep + projDep
	projPPENet := projPPEAtCost - projAccumDep

	// Other Non-Current Rollforwards
	prevGoodwill := getValue(prevBS.NoncurrentAssets.Goodwill)
	prevIntangibles := getValue(prevBS.NoncurrentAssets.Intangibles)
	prevLTI := getValue(prevBS.NoncurrentAssets.LongTermInvestments)
	prevDTA := getValue(prevBS.NoncurrentAssets.DeferredTaxAssetsLT)
	prevOtherNCA := getValue(prevBS.NoncurrentAssets.OtherNoncurrentAssets)

	projGoodwill := prevGoodwill
	projIntangibles := prevIntangibles
	projLTI := prevLTI
	projDTA := prevDTA
	projOtherNCA := prevOtherNCA

	// -------------------------------------------------------------------------
	// C. Current Liabilities (Drivers)
	// -------------------------------------------------------------------------
	var projAP float64
	if assumptions.AccountsPayablePercent != 0 {
		projAP = projRev * assumptions.AccountsPayablePercent
	} else {
		projAP = (-projCOGS / 365.0) * assumptions.DPO
	}

	var projDefRev float64
	if assumptions.DeferredRevenuePercent != 0 {
		projDefRev = projRev * assumptions.DeferredRevenuePercent
	} else {
		prevDefRev := getValue(prevBS.CurrentLiabilities.DeferredRevenueCurrent)
		projDefRev = prevDefRev // Naive rollforward
	}

	prevAccrued := getValue(prevBS.CurrentLiabilities.AccruedLiabilities)
	prevOtherCL := getValue(prevBS.CurrentLiabilities.OtherCurrentLiabilities)

	projAccrued := prevAccrued
	projOtherCL := prevOtherCL

	// -------------------------------------------------------------------------
	// D. Non-Current Liabilities
	// -------------------------------------------------------------------------
	prevDTL := getValue(prevBS.NoncurrentLiabilities.DeferredTaxLiabilities)
	prevOtherNCL := getValue(prevBS.NoncurrentLiabilities.OtherNoncurrentLiabilities)
	prevLTD := getValue(prevBS.NoncurrentLiabilities.LongTermDebt)

	projDTL := prevDTL
	projOtherNCL := prevOtherNCL
	projLTD := prevLTD // Debt held constant before plug

	// -------------------------------------------------------------------------
	// E. Equity
	// -------------------------------------------------------------------------
	prevStock := getValue(prevBS.Equity.CommonStockAPIC)
	prevRE := getValue(prevBS.Equity.RetainedEarningsDeficit)
	prevNCI := getValue(prevBS.Equity.NoncontrollingInterests)
	prevAOCI := getValue(prevBS.Equity.AccumOtherComprehensiveIncome)
	prevTreasury := getValue(prevBS.Equity.TreasuryStock)

	// Stock Based Comp Impact (Increases Equity/APIC)
	projSBC := projRev * assumptions.StockBasedCompPercent

	projStock := prevStock + projSBC          // SBC increases APIC
	projRE := prevRE + projNI - projDividends // Add NI, Subtract Dividends
	projNCI := prevNCI
	projAOCI := prevAOCI
	projTreasury := prevTreasury

	// -------------------------------------------------------------------------
	// F. Balance Sheet Balancing (The Plug)
	// -------------------------------------------------------------------------
	// Strategy: Cash = (L + E) - (Non-Cash Assets)

	// Sum L + E (Excluding ST Debt Plug)
	clTotalNoPlug := projAP + projAccrued + projOtherCL + projDefRev
	nclTotal := projLTD + projDTL + projOtherNCL
	eqTotal := projStock + projRE + projNCI + projAOCI + projTreasury

	totalSources := clTotalNoPlug + nclTotal + eqTotal

	// Sum Non-Cash Assets
	ncaTotal := projAR + projInv + projOtherCA + projSTInvest +
		projPPENet + projGoodwill + projIntangibles +
		projLTI + projDTA + projOtherNCA

	// Derived Cash
	derivedCash := totalSources - ncaTotal
	revolverNeeded := 0.0

	if derivedCash < 0 {
		revolverNeeded = math.Abs(derivedCash)
		derivedCash = 0 // Min cash floor
	}

	projCash := derivedCash
	projDebtST := revolverNeeded
	negProjAccumDep := -projAccumDep

	// Construct Projected BS
	projBS := &edgar.BalanceSheet{
		CurrentAssets: edgar.CurrentAssets{
			CashAndEquivalents:    &edgar.FSAPValue{Value: &projCash},
			ShortTermInvestments:  &edgar.FSAPValue{Value: &projSTInvest},
			AccountsReceivableNet: &edgar.FSAPValue{Value: &projAR},
			Inventories:           &edgar.FSAPValue{Value: &projInv},
			OtherCurrentAssets:    &edgar.FSAPValue{Value: &projOtherCA},
			CalculatedTotal:       new(float64),
		},
		NoncurrentAssets: edgar.NoncurrentAssets{
			PPEAtCost:               &edgar.FSAPValue{Value: &projPPEAtCost},
			AccumulatedDepreciation: &edgar.FSAPValue{Value: &negProjAccumDep},
			PPENet:                  &edgar.FSAPValue{Value: &projPPENet},
			Goodwill:                &edgar.FSAPValue{Value: &projGoodwill},
			Intangibles:             &edgar.FSAPValue{Value: &projIntangibles},
			LongTermInvestments:     &edgar.FSAPValue{Value: &projLTI},
			DeferredTaxAssetsLT:     &edgar.FSAPValue{Value: &projDTA},
			OtherNoncurrentAssets:   &edgar.FSAPValue{Value: &projOtherNCA},
			CalculatedTotal:         new(float64),
		},
		CurrentLiabilities: edgar.CurrentLiabilities{
			AccountsPayable:           &edgar.FSAPValue{Value: &projAP},
			AccruedLiabilities:        &edgar.FSAPValue{Value: &projAccrued},
			DeferredRevenueCurrent:    &edgar.FSAPValue{Value: &projDefRev},
			OtherCurrentLiabilities:   &edgar.FSAPValue{Value: &projOtherCL},
			NotesPayableShortTermDebt: &edgar.FSAPValue{Value: &projDebtST},
			CalculatedTotal:           new(float64),
		},
		NoncurrentLiabilities: edgar.NoncurrentLiabilities{
			LongTermDebt:               &edgar.FSAPValue{Value: &projLTD},
			DeferredTaxLiabilities:     &edgar.FSAPValue{Value: &projDTL},
			OtherNoncurrentLiabilities: &edgar.FSAPValue{Value: &projOtherNCL},
			CalculatedTotal:            new(float64),
		},
		Equity: edgar.Equity{
			CommonStockAPIC:               &edgar.FSAPValue{Value: &projStock},
			RetainedEarningsDeficit:       &edgar.FSAPValue{Value: &projRE},
			NoncontrollingInterests:       &edgar.FSAPValue{Value: &projNCI},
			AccumOtherComprehensiveIncome: &edgar.FSAPValue{Value: &projAOCI},
			TreasuryStock:                 &edgar.FSAPValue{Value: &projTreasury},
			CalculatedTotal:               new(float64),
		},
	}

	// Dynamic Item Projection (NodeDrivers) for Balance Sheet
	if assumptions.NodeDrivers != nil {
		for key, pct := range assumptions.NodeDrivers {
			projValue := projRev * pct
			projValuePtr := new(float64)
			*projValuePtr = projValue

			switch {
			case len(key) > 6 && key[:6] == "BS-CA:":
				label := key[7:]
				projBS.CurrentAssets.AdditionalItems = append(
					projBS.CurrentAssets.AdditionalItems,
					edgar.FSAPValue{Label: label, Value: projValuePtr},
				)
			case len(key) > 7 && key[:7] == "BS-NCA:":
				label := key[8:]
				projBS.NoncurrentAssets.AdditionalItems = append(
					projBS.NoncurrentAssets.AdditionalItems,
					edgar.FSAPValue{Label: label, Value: projValuePtr},
				)
			case len(key) > 6 && key[:6] == "BS-CL:":
				label := key[7:]
				projBS.CurrentLiabilities.AdditionalItems = append(
					projBS.CurrentLiabilities.AdditionalItems,
					edgar.FSAPValue{Label: label, Value: projValuePtr},
				)
			case len(key) > 7 && key[:7] == "BS-NCL:":
				label := key[8:]
				projBS.NoncurrentLiabilities.AdditionalItems = append(
					projBS.NoncurrentLiabilities.AdditionalItems,
					edgar.FSAPValue{Label: label, Value: projValuePtr},
				)
			case len(key) > 6 && key[:6] == "BS-Eq:":
				label := key[7:]
				projBS.Equity.AdditionalItems = append(
					projBS.Equity.AdditionalItems,
					edgar.FSAPValue{Label: label, Value: projValuePtr},
				)
			}
		}
	}

	calc.CalculateBalanceSheetTotals(projBS)

	return projBS, revolverNeeded, projDep, projCapex, projSBC
}

// projectCashFlow calculates the projected Cash Flow Statement
func (e *ProjectionEngine) projectCashFlow(
	prevBS *edgar.BalanceSheet,
	projBS *edgar.BalanceSheet,
	projNI float64,
	projDep float64,
	projCapex float64,
	projSBC float64,
	revolverNeeded float64, // Used for Financing
	projDividends float64,
) *edgar.CashFlowStatement {

	// 5. Reconcile Cash Flow
	// prevCash already fetched in BS step, but we need Change in Cash for check
	prevCash := getValue(prevBS.CurrentAssets.CashAndEquivalents)
	projCash := getValue(projBS.CurrentAssets.CashAndEquivalents)

	// Working Capital Changes
	prevAR := getValue(prevBS.CurrentAssets.AccountsReceivableNet)
	prevInv := getValue(prevBS.CurrentAssets.Inventories)
	prevAP := getValue(prevBS.CurrentLiabilities.AccountsPayable)

	projAR := getValue(projBS.CurrentAssets.AccountsReceivableNet)
	projInv := getValue(projBS.CurrentAssets.Inventories)
	projAP := getValue(projBS.CurrentLiabilities.AccountsPayable)

	chgAR := -(projAR - prevAR)
	chgInv := -(projInv - prevInv)
	chgAP := (projAP - prevAP)

	finalNetChange := projCash - prevCash

	// Calculate Section Totals explicitly
	// OCF = NI + Dep + SBC + Working Capital Changes
	netCashOp := projNI + projDep + projSBC + chgAR + chgInv + chgAP
	netCashInv := projCapex
	netCashFin := revolverNeeded - projDividends // Inflows (Debt) - Outflows (Divs)

	projCF := &edgar.CashFlowStatement{
		OperatingActivities: &edgar.CFOperatingSection{
			NetIncomeStart:           &edgar.FSAPValue{Value: &projNI},
			DepreciationAmortization: &edgar.FSAPValue{Value: &projDep},
			ChangeReceivables:        &edgar.FSAPValue{Value: &chgAR},
			ChangeInventory:          &edgar.FSAPValue{Value: &chgInv},
			ChangePayables:           &edgar.FSAPValue{Value: &chgAP},
		},
		InvestingActivities: &edgar.CFInvestingSection{
			Capex: &edgar.FSAPValue{Value: &projCapex},
		},
		FinancingActivities: &edgar.CFFinancingSection{
			DebtProceeds:  &edgar.FSAPValue{Value: &revolverNeeded},
			DividendsPaid: &edgar.FSAPValue{Value: &projDividends},
		},
		CashSummary: &edgar.CashSummarySection{
			NetCashOperating: &edgar.FSAPValue{Value: &netCashOp},
			NetCashInvesting: &edgar.FSAPValue{Value: &netCashInv},
			NetCashFinancing: &edgar.FSAPValue{Value: &netCashFin},
			CashBeginning:    &edgar.FSAPValue{Value: &prevCash},
			CashEnding:       &edgar.FSAPValue{Value: &projCash},
			NetChangeInCash:  &edgar.FSAPValue{Value: &finalNetChange},
		},
	}

	return projCF
}
