package calc

import (
	"agentic_valuation/pkg/core/edgar"
	"math"
	"strings"
)

// CalculateBalanceSheetTotals computes the calculated totals for each section of the Balance Sheet.
// It populates the CalculatedTotal field in each section struct using the primary year (.Value).
func CalculateBalanceSheetTotals(bs *edgar.BalanceSheet) {
	if bs == nil {
		return
	}

	// 1. Current Assets - sum all fields
	caTotal := sumFSAPValues(
		bs.CurrentAssets.CashAndEquivalents,
		bs.CurrentAssets.ShortTermInvestments,
		bs.CurrentAssets.AccountsReceivableNet,
		bs.CurrentAssets.Inventories,
		bs.CurrentAssets.FinanceDivLoansST,
		bs.CurrentAssets.FinanceDivOtherCurrAsset,
		bs.CurrentAssets.OtherAssets,
		bs.CurrentAssets.OtherCurrentAssets,
	)
	caTotal += sumAdditionalItems(bs.CurrentAssets.AdditionalItems)
	bs.CurrentAssets.CalculatedTotal = &caTotal

	// 2. Noncurrent Assets - direct sum (signs already correct from preprocessing)
	ncaTotal := sumFSAPValues(
		bs.NoncurrentAssets.PPEAtCost,
		bs.NoncurrentAssets.AccumulatedDepreciation, // Already negative from preprocessing
		bs.NoncurrentAssets.LongTermInvestments,
		bs.NoncurrentAssets.DeferredChargesLT,
		bs.NoncurrentAssets.Intangibles,
		bs.NoncurrentAssets.Goodwill,
		bs.NoncurrentAssets.FinanceDivLoansLT,
		bs.NoncurrentAssets.FinanceDivOtherLTAssets,
		bs.NoncurrentAssets.DeferredTaxAssetsLT,
		bs.NoncurrentAssets.RestrictedCash,
		bs.NoncurrentAssets.OtherNoncurrentAssets,
	)
	// If PPE Net is provided, use it instead of PPE at Cost + Accum Depr
	if bs.NoncurrentAssets.PPENet != nil && bs.NoncurrentAssets.PPENet.Value != nil {
		// Subtract PPE at Cost and Accum Depr, add PPE Net
		ncaTotal -= getValue(bs.NoncurrentAssets.PPEAtCost)
		ncaTotal -= getValue(bs.NoncurrentAssets.AccumulatedDepreciation)
		ncaTotal += getValue(bs.NoncurrentAssets.PPENet)
	}
	ncaTotal += sumAdditionalItems(bs.NoncurrentAssets.AdditionalItems)
	bs.NoncurrentAssets.CalculatedTotal = &ncaTotal

	// 3. Current Liabilities
	clTotal := sumFSAPValues(
		bs.CurrentLiabilities.AccountsPayable,
		bs.CurrentLiabilities.AccruedLiabilities,
		bs.CurrentLiabilities.NotesPayableShortTermDebt,
		bs.CurrentLiabilities.CurrentMaturitiesLTD,
		bs.CurrentLiabilities.CurrentOperatingLeaseLiab,
		bs.CurrentLiabilities.DeferredRevenueCurrent,
		bs.CurrentLiabilities.FinanceDivCurr,
		bs.CurrentLiabilities.OtherCurrentLiabilities,
	)
	clTotal += sumAdditionalItems(bs.CurrentLiabilities.AdditionalItems)
	bs.CurrentLiabilities.CalculatedTotal = &clTotal

	// 4. Noncurrent Liabilities
	nclTotal := sumFSAPValues(
		bs.NoncurrentLiabilities.LongTermDebt,
		bs.NoncurrentLiabilities.LongTermOperatingLeaseLiab,
		bs.NoncurrentLiabilities.DeferredTaxLiabilities,
		bs.NoncurrentLiabilities.PensionObligations,
		bs.NoncurrentLiabilities.FinanceDivNoncurr,
		bs.NoncurrentLiabilities.OtherNoncurrentLiabilities,
	)
	nclTotal += sumAdditionalItems(bs.NoncurrentLiabilities.AdditionalItems)
	bs.NoncurrentLiabilities.CalculatedTotal = &nclTotal

	// 5. Equity - direct sum (Treasury Stock already negative from preprocessing)
	eqTotal := sumFSAPValues(
		bs.Equity.PreferredStock,
		bs.Equity.CommonStockAPIC,
		bs.Equity.RetainedEarningsDeficit,       // Can be negative (deficit)
		bs.Equity.TreasuryStock,                 // Already negative from preprocessing
		bs.Equity.AccumOtherComprehensiveIncome, // Can be positive or negative
		bs.Equity.NoncontrollingInterests,
	)
	eqTotal += sumAdditionalItems(bs.Equity.AdditionalItems)
	bs.Equity.CalculatedTotal = &eqTotal
}

// CalculateIncomeStatementTotals computes calculated totals for Income Statement sections.
// Flow-through validation:
//   - Gross Profit = Revenues - COGS
//   - Operating Income = Gross Profit - Operating Expenses
//   - Income Before Tax = Operating Income + Non-Operating Items
//   - Net Income = Income Before Tax - Taxes
func CalculateIncomeStatementTotals(is *edgar.IncomeStatement) *IncomeStatementTotals {
	if is == nil {
		return nil
	}

	result := &IncomeStatementTotals{}

	// Section 1: Gross Profit
	if is.GrossProfitSection != nil {
		gps := is.GrossProfitSection
		result.Revenues = getValue(gps.Revenues)
		result.COGS = getValue(gps.CostOfGoodsSold) // Already negative from preprocessing
		result.GrossProfitCalc = result.Revenues + result.COGS
		result.GrossProfitReported = getValue(gps.GrossProfit)
	}

	// Section 2: Operating Costs
	if is.OperatingCostSection != nil {
		ops := is.OperatingCostSection
		result.OpExTotal = sumFSAPValues(
			ops.SGAExpenses,
			ops.RDExpenses,
			ops.AdvertisingExpenses,
			ops.OtherOperatingExpenses,
		)
		result.OpExTotal += sumAdditionalItemsIS(ops.AdditionalItems)
		result.OperatingIncomeCalc = result.GrossProfitCalc + result.OpExTotal // OpEx are negative
		result.OperatingIncomeReported = getValue(ops.OperatingIncome)
	}

	// Section 3: Non-Operating Items
	if is.NonOperatingSection != nil {
		nos := is.NonOperatingSection
		result.NonOpTotal = sumFSAPValues(
			nos.InterestExpense,
			nos.OtherIncomeExpense,
			nos.EquityAffiliatesNonOperating,
		)
		result.NonOpTotal += sumAdditionalItemsIS(nos.AdditionalItems)
		result.IncomeBeforeTaxCalc = result.OperatingIncomeCalc + result.NonOpTotal
		result.IncomeBeforeTaxReported = getValue(nos.IncomeBeforeTax)
	}

	// Section 4: Tax & Adjustments
	if is.TaxAdjustments != nil {
		tas := is.TaxAdjustments
		result.TaxExpense = getValue(tas.IncomeTaxExpense) // Should be negative
		result.Adjustments = sumFSAPValues(
			tas.DiscontinuedOperations,
			tas.ExtraordinaryItems,
		)
		result.Adjustments += sumAdditionalItemsIS(tas.AdditionalItems)
		result.NetIncomeCalc = result.IncomeBeforeTaxCalc + result.TaxExpense + result.Adjustments
	}

	// Section 5: Net Income Allocation
	if is.NetIncomeSection != nil {
		nis := is.NetIncomeSection
		result.NetIncomeToCommon = getValue(nis.NetIncomeToCommon)
		result.NetIncomeToNCI = getValue(nis.NetIncomeToNCI)
		// Use NetIncomeToCommon as the reported Net Income for validation
		result.NetIncomeReported = result.NetIncomeToCommon
	}

	// Section 6: OCI
	if is.OCISection != nil {
		oci := is.OCISection
		result.OCITotal = sumFSAPValues(
			oci.OCIForeignCurrency,
			oci.OCISecurities,
			oci.OCIPension,
			oci.OCIHedges,
		)
		result.OCITotal += sumAdditionalItemsIS(oci.AdditionalItems)
		result.ComprehensiveIncomeCalc = result.NetIncomeCalc + result.OCITotal
		result.ComprehensiveIncomeReported = getValue(oci.OtherComprehensiveIncome)
	}

	return result
}

// IncomeStatementTotals holds calculated totals for Income Statement validation
type IncomeStatementTotals struct {
	// Section 1: Gross Profit
	Revenues            float64
	COGS                float64
	GrossProfitCalc     float64
	GrossProfitReported float64

	// Section 2: Operating
	OpExTotal               float64
	OperatingIncomeCalc     float64
	OperatingIncomeReported float64

	// Section 3: Non-Operating
	NonOpTotal              float64
	IncomeBeforeTaxCalc     float64
	IncomeBeforeTaxReported float64

	// Section 4: Tax & Adjustments
	TaxExpense        float64
	Adjustments       float64
	NetIncomeCalc     float64
	NetIncomeReported float64

	// Section 5: Allocation
	NetIncomeToCommon float64
	NetIncomeToNCI    float64

	// Section 6: OCI
	OCITotal                    float64
	ComprehensiveIncomeCalc     float64
	ComprehensiveIncomeReported float64
}

// CalculateIncomeStatementTotalsByYear computes calculated totals for a specific fiscal year.
// This is the multi-year-aware version that reads from .Years map.
func CalculateIncomeStatementTotalsByYear(is *edgar.IncomeStatement, year string) *IncomeStatementTotals {
	if is == nil {
		return nil
	}

	result := &IncomeStatementTotals{}

	// Section 1: Gross Profit
	if is.GrossProfitSection != nil {
		gps := is.GrossProfitSection
		result.Revenues = getValueByYear(gps.Revenues, year)
		result.COGS = getValueByYear(gps.CostOfGoodsSold, year) // Already negative from preprocessing
		result.GrossProfitCalc = result.Revenues + result.COGS
		result.GrossProfitReported = getValueByYear(gps.GrossProfit, year)
	}

	// Section 2: Operating Costs
	if is.OperatingCostSection != nil {
		ops := is.OperatingCostSection
		result.OpExTotal = sumFSAPValuesByYear(year,
			ops.SGAExpenses,
			ops.RDExpenses,
			ops.AdvertisingExpenses,
			ops.OtherOperatingExpenses,
		)
		result.OpExTotal += sumAdditionalItemsByYearIS(ops.AdditionalItems, year)
		result.OperatingIncomeCalc = result.GrossProfitCalc + result.OpExTotal // OpEx already negative
		result.OperatingIncomeReported = getValueByYear(ops.OperatingIncome, year)
	}

	// Section 3: Non-Operating Items
	if is.NonOperatingSection != nil {
		nos := is.NonOperatingSection
		result.NonOpTotal = sumFSAPValuesByYear(year,
			nos.InterestExpense,
			nos.OtherIncomeExpense,
			nos.EquityAffiliatesNonOperating,
		)
		result.NonOpTotal += sumAdditionalItemsByYearIS(nos.AdditionalItems, year)
		result.IncomeBeforeTaxCalc = result.OperatingIncomeCalc + result.NonOpTotal
		result.IncomeBeforeTaxReported = getValueByYear(nos.IncomeBeforeTax, year)
	}

	// Section 4: Tax & Adjustments
	if is.TaxAdjustments != nil {
		tas := is.TaxAdjustments
		result.TaxExpense = getValueByYear(tas.IncomeTaxExpense, year)
		result.Adjustments = sumFSAPValuesByYear(year,
			tas.DiscontinuedOperations,
			tas.ExtraordinaryItems,
		)
		result.Adjustments += sumAdditionalItemsByYearIS(tas.AdditionalItems, year)
		result.NetIncomeCalc = result.IncomeBeforeTaxCalc + result.TaxExpense + result.Adjustments // Tax already negative
	}

	// Section 5: Net Income Allocation
	if is.NetIncomeSection != nil {
		nis := is.NetIncomeSection
		result.NetIncomeToCommon = getValueByYear(nis.NetIncomeToCommon, year)
		result.NetIncomeToNCI = getValueByYear(nis.NetIncomeToNCI, year)
		result.NetIncomeReported = result.NetIncomeToCommon
	}

	// Section 6: OCI
	if is.OCISection != nil {
		oci := is.OCISection
		result.OCITotal = sumFSAPValuesByYear(year,
			oci.OCIForeignCurrency,
			oci.OCISecurities,
			oci.OCIPension,
			oci.OCIHedges,
		)
		result.OCITotal += sumAdditionalItemsByYearIS(oci.AdditionalItems, year)
		result.ComprehensiveIncomeCalc = result.NetIncomeCalc + result.OCITotal
		result.ComprehensiveIncomeReported = getValueByYear(oci.OtherComprehensiveIncome, year)
	}

	return result
}

// sumAdditionalItemsByYearIS sums AdditionalItem values for a specific year
// Supports both {value: {years}} and direct {years} formats
func sumAdditionalItemsByYearIS(items []edgar.AdditionalItem, year string) float64 {
	total := 0.0
	for _, item := range items {
		// Safety check: Skip obvious subtotals
		if item.Label != "" && isSubtotalLabel(item.Label) {
			continue
		}

		// Try wrapped format: {value: {years}}
		if item.Value != nil {
			if item.Value.Years != nil {
				if v, ok := item.Value.Years[year]; ok {
					total += v
					continue
				}
			}
			if item.Value.Value != nil {
				total += *item.Value.Value
				continue
			}
		}
		// Try direct format: {years} (LLM sometimes outputs this)
		if item.Years != nil {
			if v, ok := item.Years[year]; ok {
				total += v
			}
		}
	}
	return total
}

// sumAdditionalItemsIS sums AdditionalItem values (different struct from FSAPValue)
func sumAdditionalItemsIS(items []edgar.AdditionalItem) float64 {
	total := 0.0
	for _, item := range items {
		// Safety check: Skip obvious subtotals
		if item.Label != "" && isSubtotalLabel(item.Label) {
			continue
		}

		if item.Value != nil && item.Value.Value != nil {
			total += *item.Value.Value
		}
	}
	return total
}

// CalculateCashFlowTotals computes calculated totals for Cash Flow Statement sections.
// Operating starts from Net Income Attributable to Common Shareholders.
// Validation: Net Change in Cash = Operating + Investing + Financing + FX Effect
func CalculateCashFlowTotals(cf *edgar.CashFlowStatement) *CashFlowTotals {
	if cf == nil {
		return nil
	}

	result := &CashFlowTotals{}

	// Section 1: Operating Activities
	// Starts from Net Income (Attributable to Common Shareholders)
	if cf.OperatingActivities != nil {
		ops := cf.OperatingActivities
		// Track starting Net Income for cross-statement validation
		result.NetIncomeStart = getValue(ops.NetIncomeStart)

		result.OperatingCalc = sumFSAPValues(
			ops.NetIncomeStart, // Should equal IS Net Income to Common
			ops.DepreciationAmortization,
			ops.AmortizationIntangibles,
			ops.DeferredTaxes,
			ops.StockBasedCompensation,
			ops.ImpairmentCharges,
			ops.GainLossAssetSales,
			ops.ChangeReceivables,
			ops.ChangeInventory,
			ops.ChangePayables,
			ops.ChangeAccruedExpenses,
			ops.ChangeDeferredRevenue,
			ops.OtherWorkingCapital,
			ops.OtherNonCashItems,
		)
		result.OperatingCalc += sumAdditionalItemsIS(ops.AdditionalItems)
	}

	// Section 2: Investing Activities
	if cf.InvestingActivities != nil {
		inv := cf.InvestingActivities
		result.InvestingCalc = sumFSAPValues(
			inv.Capex,
			inv.AcquisitionsNet,
			inv.PurchasesSecurities,
			inv.MaturitiesSecurities,
			inv.SalesSecurities,
			inv.ProceedsAssetSales,
			inv.OtherInvesting,
		)
		result.InvestingCalc += sumAdditionalItemsIS(inv.AdditionalItems)
	}

	// Section 3: Financing Activities
	if cf.FinancingActivities != nil {
		fin := cf.FinancingActivities
		result.FinancingCalc = sumFSAPValues(
			fin.DebtProceeds,
			fin.DebtRepayments,
			fin.StockIssuanceProceeds,
			fin.ShareRepurchases,
			fin.DividendsPaid,
			fin.TaxWithholdingPayments,
			fin.OtherFinancing,
		)
		result.FinancingCalc += sumAdditionalItemsIS(fin.AdditionalItems)
	}

	// Section 4: Cash Summary (reported values from filing)
	if cf.CashSummary != nil {
		cs := cf.CashSummary
		result.OperatingReported = getValue(cs.NetCashOperating)
		result.InvestingReported = getValue(cs.NetCashInvesting)
		result.FinancingReported = getValue(cs.NetCashFinancing)
		result.FXEffect = getValue(cs.FXEffect)
		result.NetChangeReported = getValue(cs.NetChangeInCash)
		result.CashBeginning = getValue(cs.CashBeginning)
		result.CashEnding = getValue(cs.CashEnding)
	}

	// Calculate validation: Net Change = Operating + Investing + Financing + FX
	result.NetChangeCalc = result.OperatingCalc + result.InvestingCalc + result.FinancingCalc + result.FXEffect

	// Cash reconciliation: Ending = Beginning + Net Change
	result.CashEndingCalc = result.CashBeginning + result.NetChangeCalc

	return result
}

// CalculateSupplementalData computes the Grey (calculated) fields for Supplemental Data
// Requires data from Income Statement and Cash Flow Statement
func CalculateSupplementalData(
	is *edgar.IncomeStatement,
	cf *edgar.CashFlowStatement,
	sd *edgar.SupplementalData,
) *SupplementalDataCalc {
	result := &SupplementalDataCalc{}

	// Get Income Before Tax and Tax Expense from IS
	var incomeTax, incomeBeforeTax float64
	if is != nil {
		if is.TaxAdjustments != nil {
			incomeTax = getValue(is.TaxAdjustments.IncomeTaxExpense)
		}
		if is.NonOperatingSection != nil {
			incomeBeforeTax = getValue(is.NonOperatingSection.IncomeBeforeTax)
		}
	}

	// Calculate Effective Tax Rate = |Tax Expense| / Income Before Tax
	if incomeBeforeTax != 0 {
		rate := math.Abs(incomeTax) / incomeBeforeTax
		result.EffectiveTaxRate = &rate
	}

	// Get NonRecurring items from dedicated NonRecurringSection
	var nonRecurringItems float64
	if is != nil && is.NonRecurringSection != nil {
		nrs := is.NonRecurringSection
		nonRecurringItems = sumFSAPValues(
			nrs.ImpairmentCharges,
			nrs.RestructuringCharges,
			nrs.GainLossAssetSales,
			nrs.SettlementCosts,
			nrs.WriteOffs,
			nrs.OtherNonRecurring,
		)
		nonRecurringItems += sumAdditionalItemsIS(nrs.AdditionalItems)
	}

	// Calculate After-Tax NonRecurring = NonRecurring * (1 - TaxRate)
	if result.EffectiveTaxRate != nil && nonRecurringItems != 0 {
		afterTax := nonRecurringItems * (1 - *result.EffectiveTaxRate)
		result.AfterTaxNonRecurring = &afterTax
	}

	// Get Dividends from Cash Flow Financing
	var totalDividends float64
	if cf != nil && cf.FinancingActivities != nil {
		totalDividends = math.Abs(getValue(cf.FinancingActivities.DividendsPaid))
	}

	// Get Shares Outstanding from Supplemental Data
	var sharesOutstanding float64
	if sd != nil && sd.SharesOutstandingBasic != nil {
		sharesOutstanding = getValue(sd.SharesOutstandingBasic)
	}

	// Calculate Common Dividends per Share = Total Dividends / Shares
	if sharesOutstanding != 0 {
		dps := totalDividends / sharesOutstanding
		result.CommonDividendsPerShare = &dps
	}

	// Get Depreciation from Cash Flow Operating
	if cf != nil && cf.OperatingActivities != nil {
		depn := getValue(cf.OperatingActivities.DepreciationAmortization)
		result.DepreciationExpense = &depn
	}

	return result
}

// SupplementalDataCalc holds calculated supplemental data fields
type SupplementalDataCalc struct {
	EffectiveTaxRate        *float64 // Tax / IBT
	AfterTaxNonRecurring    *float64 // NonRecurring * (1 - TaxRate)
	CommonDividendsPerShare *float64 // Dividends / Shares
	DepreciationExpense     *float64 // From Cash Flow
}

// CashFlowTotals holds calculated totals for Cash Flow Statement validation
type CashFlowTotals struct {
	// Starting Net Income (should equal IS Net Income to Common)
	NetIncomeStart float64

	// Operating Activities
	OperatingCalc     float64
	OperatingReported float64

	// Investing Activities
	InvestingCalc     float64
	InvestingReported float64

	// Financing Activities
	FinancingCalc     float64
	FinancingReported float64

	// Cash Summary
	FXEffect          float64
	NetChangeCalc     float64
	NetChangeReported float64
	CashBeginning     float64
	CashEndingCalc    float64
	CashEnding        float64
}

// CalculateCashFlowTotalsByYear computes calculated totals for a specific fiscal year.
// This is the multi-year-aware version that reads from .Years map.
func CalculateCashFlowTotalsByYear(cf *edgar.CashFlowStatement, year string) *CashFlowTotals {
	if cf == nil {
		return nil
	}

	result := &CashFlowTotals{}

	// Section 1: Operating Activities
	if cf.OperatingActivities != nil {
		ops := cf.OperatingActivities
		result.NetIncomeStart = getValueByYear(ops.NetIncomeStart, year)

		result.OperatingCalc = sumFSAPValuesByYear(year,
			ops.NetIncomeStart,
			ops.DepreciationAmortization,
			ops.AmortizationIntangibles,
			ops.DeferredTaxes,
			ops.StockBasedCompensation,
			ops.ImpairmentCharges,
			ops.GainLossAssetSales,
			ops.ChangeReceivables,
			ops.ChangeInventory,
			ops.ChangePayables,
			ops.ChangeAccruedExpenses,
			ops.ChangeDeferredRevenue,
			ops.OtherWorkingCapital,
			ops.OtherNonCashItems,
		)
		result.OperatingCalc += sumAdditionalItemsByYearIS(ops.AdditionalItems, year)
	}

	// Section 2: Investing Activities
	if cf.InvestingActivities != nil {
		inv := cf.InvestingActivities
		result.InvestingCalc = sumFSAPValuesByYear(year,
			inv.Capex,
			inv.AcquisitionsNet,
			inv.PurchasesSecurities,
			inv.MaturitiesSecurities,
			inv.SalesSecurities,
			inv.ProceedsAssetSales,
			inv.OtherInvesting,
		)
		result.InvestingCalc += sumAdditionalItemsByYearIS(inv.AdditionalItems, year)
	}

	// Section 3: Financing Activities
	if cf.FinancingActivities != nil {
		fin := cf.FinancingActivities
		result.FinancingCalc = sumFSAPValuesByYear(year,
			fin.DebtProceeds,
			fin.DebtRepayments,
			fin.StockIssuanceProceeds,
			fin.ShareRepurchases,
			fin.DividendsPaid,
			fin.TaxWithholdingPayments,
			fin.OtherFinancing,
		)
		result.FinancingCalc += sumAdditionalItemsByYearIS(fin.AdditionalItems, year)
	}

	// Section 4: Cash Summary (reported values)
	if cf.CashSummary != nil {
		cs := cf.CashSummary
		result.OperatingReported = getValueByYear(cs.NetCashOperating, year)
		result.InvestingReported = getValueByYear(cs.NetCashInvesting, year)
		result.FinancingReported = getValueByYear(cs.NetCashFinancing, year)
		result.FXEffect = getValueByYear(cs.FXEffect, year)
		result.NetChangeReported = getValueByYear(cs.NetChangeInCash, year)
		result.CashBeginning = getValueByYear(cs.CashBeginning, year)
		result.CashEnding = getValueByYear(cs.CashEnding, year)
	}

	// Calculate validation: Net Change = Operating + Investing + Financing + FX
	result.NetChangeCalc = result.OperatingCalc + result.InvestingCalc + result.FinancingCalc + result.FXEffect

	// Cash reconciliation: Ending = Beginning + Net Change
	result.CashEndingCalc = result.CashBeginning + result.NetChangeCalc

	return result
}

// BalanceSheetTotalsByYear holds calculated totals for a specific fiscal year
type BalanceSheetTotalsByYear struct {
	Year                       string
	TotalCurrentAssets         float64
	TotalNoncurrentAssets      float64
	TotalAssets                float64
	TotalCurrentLiabilities    float64
	TotalNoncurrentLiabilities float64
	TotalLiabilities           float64
	TotalEquity                float64
	BalanceCheck               float64 // Should be 0 if A = L + E
}

// CalculateBalanceSheetByYear computes section totals for a specific fiscal year
func CalculateBalanceSheetByYear(bs *edgar.BalanceSheet, year string) *BalanceSheetTotalsByYear {
	if bs == nil {
		return nil
	}

	result := &BalanceSheetTotalsByYear{Year: year}

	// 1. Current Assets
	result.TotalCurrentAssets = sumFSAPValuesByYear(year,
		bs.CurrentAssets.CashAndEquivalents,
		bs.CurrentAssets.ShortTermInvestments,
		bs.CurrentAssets.AccountsReceivableNet,
		bs.CurrentAssets.Inventories,
		bs.CurrentAssets.FinanceDivLoansST,
		bs.CurrentAssets.FinanceDivOtherCurrAsset,
		bs.CurrentAssets.OtherAssets,
		bs.CurrentAssets.OtherCurrentAssets,
	)
	result.TotalCurrentAssets += sumAdditionalItemsByYear(bs.CurrentAssets.AdditionalItems, year)

	// 2. Noncurrent Assets
	ppeAtCost := getValueByYear(bs.NoncurrentAssets.PPEAtCost, year)
	accumDepr := getValueByYear(bs.NoncurrentAssets.AccumulatedDepreciation, year)
	ppeNet := getValueByYear(bs.NoncurrentAssets.PPENet, year)

	if ppeNet != 0 {
		result.TotalNoncurrentAssets += ppeNet
	} else if ppeAtCost != 0 {
		result.TotalNoncurrentAssets += ppeAtCost - math.Abs(accumDepr)
	}

	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.LongTermInvestments, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.DeferredChargesLT, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.Intangibles, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.Goodwill, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.FinanceDivLoansLT, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.FinanceDivOtherLTAssets, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.DeferredTaxAssetsLT, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.RestrictedCash, year)
	result.TotalNoncurrentAssets += getValueByYear(bs.NoncurrentAssets.OtherNoncurrentAssets, year)
	result.TotalNoncurrentAssets += sumAdditionalItemsByYear(bs.NoncurrentAssets.AdditionalItems, year)

	// 3. Current Liabilities
	result.TotalCurrentLiabilities = sumFSAPValuesByYear(year,
		bs.CurrentLiabilities.AccountsPayable,
		bs.CurrentLiabilities.AccruedLiabilities,
		bs.CurrentLiabilities.NotesPayableShortTermDebt,
		bs.CurrentLiabilities.CurrentMaturitiesLTD,
		bs.CurrentLiabilities.CurrentOperatingLeaseLiab,
		bs.CurrentLiabilities.DeferredRevenueCurrent,
		bs.CurrentLiabilities.FinanceDivCurr,
		bs.CurrentLiabilities.OtherCurrentLiabilities,
	)
	result.TotalCurrentLiabilities += sumAdditionalItemsByYear(bs.CurrentLiabilities.AdditionalItems, year)

	// 4. Noncurrent Liabilities
	result.TotalNoncurrentLiabilities = sumFSAPValuesByYear(year,
		bs.NoncurrentLiabilities.LongTermDebt,
		bs.NoncurrentLiabilities.LongTermOperatingLeaseLiab,
		bs.NoncurrentLiabilities.DeferredTaxLiabilities,
		bs.NoncurrentLiabilities.PensionObligations,
		bs.NoncurrentLiabilities.FinanceDivNoncurr,
		bs.NoncurrentLiabilities.OtherNoncurrentLiabilities,
	)
	result.TotalNoncurrentLiabilities += sumAdditionalItemsByYear(bs.NoncurrentLiabilities.AdditionalItems, year)

	// 5. Equity
	result.TotalEquity += getValueByYear(bs.Equity.PreferredStock, year)
	result.TotalEquity += getValueByYear(bs.Equity.CommonStockAPIC, year)
	result.TotalEquity += getValueByYear(bs.Equity.RetainedEarningsDeficit, year)
	result.TotalEquity += getValueByYear(bs.Equity.AccumOtherComprehensiveIncome, year)
	result.TotalEquity += getValueByYear(bs.Equity.NoncontrollingInterests, year)

	treasuryVal := getValueByYear(bs.Equity.TreasuryStock, year)
	if treasuryVal > 0 {
		result.TotalEquity -= treasuryVal
	} else {
		result.TotalEquity += treasuryVal
	}
	result.TotalEquity += sumAdditionalItemsByYear(bs.Equity.AdditionalItems, year)

	// Calculate aggregates
	result.TotalAssets = result.TotalCurrentAssets + result.TotalNoncurrentAssets
	result.TotalLiabilities = result.TotalCurrentLiabilities + result.TotalNoncurrentLiabilities
	result.BalanceCheck = result.TotalAssets - result.TotalLiabilities - result.TotalEquity

	return result
}

// ========== Helpers ==========

// getValue extracts primary year value from FSAPValue
func getValue(item *edgar.FSAPValue) float64 {
	if item != nil && item.Value != nil {
		return *item.Value
	}
	return 0
}

// getValueByYear extracts value for a specific year from FSAPValue.Years map
// Falls back to primary .Value if year not found in map
func getValueByYear(item *edgar.FSAPValue, year string) float64 {
	if item == nil {
		return 0
	}
	// Try Years map first
	if item.Years != nil {
		if val, ok := item.Years[year]; ok {
			return val
		}
	}
	// Fallback to primary value
	if item.Value != nil {
		return *item.Value
	}
	return 0
}

func sumFSAPValues(items ...*edgar.FSAPValue) float64 {
	total := 0.0
	for _, item := range items {
		total += getValue(item)
	}
	return total
}

func sumFSAPValuesByYear(year string, items ...*edgar.FSAPValue) float64 {
	total := 0.0
	for _, item := range items {
		total += getValueByYear(item, year)
	}
	return total
}

// isSubtotalLabel detects if a label represents a subtotal/calculated field
// that should be excluded from aggregation to prevent double-counting.
func isSubtotalLabel(label string) bool {
	l := strings.ToLower(label)

	// Direct matches for common subtotals
	if l == "operating expenses" || l == "total operating expenses" {
		return true
	}
	if l == "gross profit" || l == "gross margin" {
		return true
	}
	if l == "operating income" || l == "operating profit" || l == "income from operations" {
		return true
	}
	if l == "net income" || l == "net earnings" || l == "net loss" {
		return true
	}

	// Pattern matches
	if strings.Contains(l, "total ") || strings.HasPrefix(l, "total") {
		return true
	}
	if strings.Contains(l, "income before") {
		return true
	}
	// "Total" usually sufficient, but being explicit helps safety

	return false
}

func sumAdditionalItems(items []edgar.FSAPValue) float64 {
	total := 0.0
	for _, item := range items {
		// Safety check: Skip obvious subtotals
		if item.Label != "" && isSubtotalLabel(item.Label) {
			continue
		}

		if item.Value != nil {
			// Direct sum - signs already correct from preprocessing
			total += *item.Value
		}
	}
	return total
}

func sumAdditionalItemsByYear(items []edgar.FSAPValue, year string) float64 {
	total := 0.0
	for _, item := range items {
		// Safety check: Skip obvious subtotals
		if item.Label != "" && isSubtotalLabel(item.Label) {
			continue
		}

		// Try Years map first
		if item.Years != nil {
			if v, ok := item.Years[year]; ok {
				total += v
				continue
			}
		}
		// Fallback to primary value
		if item.Value != nil {
			total += *item.Value
		}
	}
	return total
}
