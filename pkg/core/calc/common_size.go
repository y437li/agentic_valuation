package calc

import (
	"agentic_valuation/pkg/core/edgar"
	"fmt"
)

// CommonSizeDefaults holds the calculated baseline rates
type CommonSizeDefaults struct {
	RevenueAction          float64 // Not calculated from history, usually default
	COGSPercent            float64
	SGAPercent             float64
	RDPercent              float64
	TaxRate                float64
	StockBasedCompPercent  float64
	DebtInterestRate       float64
	DAPercent              float64            // D&A as % of Revenue
	CapExPercent           float64            // CapEx as % of Revenue
	DividendsPercent       float64            // Dividends as % of Revenue
	RepurchasePercent      float64            // Repurchases as % of Revenue
	NetIncomeMargin        float64            // Net Income as % of Revenue
	OtherOperatingPercent  float64            // Other Operating Expenses as % of Revenue
	OtherNonOpPercent      float64            // Other Non-Operating Income/Exp as % of Revenue
	EquityAffiliatesPct    float64            // Equity Affiliates as % of Revenue
	DiscontinuedOpsPercent float64            // Discontinued Operations as % of Revenue
	CustomItems            map[string]float64 // Dynamic Line Items as % of Revenue
	ReceivablesPercent     float64            // Accounts Receivable as % of Revenue
	InventoryPercent       float64            // Inventory as % of Revenue
	APPercent              float64            // Accounts Payable as % of Revenue
	DeferredRevPercent     float64            // Deferred Revenue as % of Revenue
}

// CalculateCommonSizeDefaults computes baseline assumptions from historical data
func CalculateCommonSizeDefaults(histData *edgar.YearData) CommonSizeDefaults {
	// 1. Set Conservative Fallbacks
	defaults := CommonSizeDefaults{
		RevenueAction:         0.05, // 5% default growth
		COGSPercent:           0.60,
		SGAPercent:            0.15,
		RDPercent:             0.05,
		TaxRate:               0.21,
		StockBasedCompPercent: 0.02,
		DebtInterestRate:      0.05,
	}

	if histData == nil {
		return defaults
	}

	is := histData.IncomeStatement
	cf := histData.CashFlowStatement
	bs := histData.BalanceSheet

	// Need Valid Revenue to calculate % of Rev
	if is.GrossProfitSection != nil && is.GrossProfitSection.Revenues != nil && is.GrossProfitSection.Revenues.Value != nil {
		rev := *is.GrossProfitSection.Revenues.Value
		if rev > 0 {
			// COGS %
			if is.GrossProfitSection.CostOfGoodsSold != nil && is.GrossProfitSection.CostOfGoodsSold.Value != nil {
				defaults.COGSPercent = *is.GrossProfitSection.CostOfGoodsSold.Value / rev
			}

			// SG&A %
			if is.OperatingCostSection != nil {
				// 1. Calculate Implied Total OpEx (GP - OpInc) to detect overlaps
				var impliedOpEx float64
				if is.GrossProfitSection.GrossProfit != nil && is.GrossProfitSection.GrossProfit.Value != nil &&
					is.OperatingCostSection.OperatingIncome != nil && is.OperatingCostSection.OperatingIncome.Value != nil {
					gp := *is.GrossProfitSection.GrossProfit.Value
					opInc := *is.OperatingCostSection.OperatingIncome.Value
					impliedOpEx = gp - opInc // This is the "True" Total Operating Expenses
				}

				// R&D % first (so we can subtract if needed)
				var rdVal float64
				if is.OperatingCostSection.RDExpenses != nil && is.OperatingCostSection.RDExpenses.Value != nil {
					rdVal = *is.OperatingCostSection.RDExpenses.Value
					defaults.RDPercent = rdVal / rev
				}

				if is.OperatingCostSection.SGAExpenses != nil && is.OperatingCostSection.SGAExpenses.Value != nil {
					sgaVal := *is.OperatingCostSection.SGAExpenses.Value

					// Heuristic: If SGA is suspiciously close to Implied OpEx (>90%) AND we have RD,
					// assume SGA includes RD (Extraction artifact).
					// Also ensure SGA > RD to avoid negative numbers.
					if impliedOpEx > 0 && rdVal > 0 && sgaVal > rdVal {
						ratio := sgaVal / impliedOpEx
						if ratio > 0.90 && ratio < 1.10 {
							fmt.Printf(" [Quant] Detected Overlap: SGA ($%.1f) covers Total OpEx ($%.1f). Subtracting RD ($%.1f)\n", sgaVal, impliedOpEx, rdVal)
							sgaVal -= rdVal
						}
					}
					defaults.SGAPercent = sgaVal / rev
				}
			}

			// Tax Rate % (Income Tax / Income Before Tax)
			if is.TaxAdjustments != nil && is.NonOperatingSection != nil {
				if is.TaxAdjustments.IncomeTaxExpense != nil && is.TaxAdjustments.IncomeTaxExpense.Value != nil &&
					is.NonOperatingSection.IncomeBeforeTax != nil && is.NonOperatingSection.IncomeBeforeTax.Value != nil {
					ibt := *is.NonOperatingSection.IncomeBeforeTax.Value
					if ibt > 0 {
						defaults.TaxRate = *is.TaxAdjustments.IncomeTaxExpense.Value / ibt
					}
				}
			}

			// Stock-Based Compensation % (SBC / Revenue)
			if cf.OperatingActivities != nil && cf.OperatingActivities.StockBasedCompensation != nil && cf.OperatingActivities.StockBasedCompensation.Value != nil {
				defaults.StockBasedCompPercent = *cf.OperatingActivities.StockBasedCompensation.Value / rev
			}

			// D&A % (D&A / Revenue)
			if cf.OperatingActivities != nil && cf.OperatingActivities.DepreciationAmortization != nil && cf.OperatingActivities.DepreciationAmortization.Value != nil {
				defaults.DAPercent = *cf.OperatingActivities.DepreciationAmortization.Value / rev
			}

			// CapEx % (CapEx / Revenue) - CapEx is usually negative in CF, use absolute value
			if cf.InvestingActivities != nil && cf.InvestingActivities.Capex != nil && cf.InvestingActivities.Capex.Value != nil {
				val := *cf.InvestingActivities.Capex.Value
				if val < 0 {
					val = -val
				}
				defaults.CapExPercent = val / rev
			}

			// Dividends % (Dividends / Revenue)
			if cf.FinancingActivities != nil && cf.FinancingActivities.DividendsPaid != nil && cf.FinancingActivities.DividendsPaid.Value != nil {
				val := *cf.FinancingActivities.DividendsPaid.Value
				if val < 0 {
					val = -val
				}
				defaults.DividendsPercent = val / rev
			}

			// Repurchases % (Buybacks / Revenue)
			if cf.FinancingActivities != nil && cf.FinancingActivities.ShareRepurchases != nil && cf.FinancingActivities.ShareRepurchases.Value != nil {
				val := *cf.FinancingActivities.ShareRepurchases.Value
				if val < 0 {
					val = -val
				}
				defaults.RepurchasePercent = val / rev
			}

			// Net Income Margin
			if is.NetIncomeSection != nil && is.NetIncomeSection.NetIncomeToCommon != nil && is.NetIncomeSection.NetIncomeToCommon.Value != nil {
				defaults.NetIncomeMargin = *is.NetIncomeSection.NetIncomeToCommon.Value / rev
			}

			// Implied Interest Rate (Interest Exp / Total Debt)
			totalDebt := 0.0
			if bs.NoncurrentLiabilities.LongTermDebt != nil && bs.NoncurrentLiabilities.LongTermDebt.Value != nil {
				totalDebt += *bs.NoncurrentLiabilities.LongTermDebt.Value
			}
			if bs.CurrentLiabilities.NotesPayableShortTermDebt != nil && bs.CurrentLiabilities.NotesPayableShortTermDebt.Value != nil {
				totalDebt += *bs.CurrentLiabilities.NotesPayableShortTermDebt.Value
			}

			if totalDebt > 0 && is.NonOperatingSection != nil && is.NonOperatingSection.InterestExpense != nil && is.NonOperatingSection.InterestExpense.Value != nil {
				defaults.DebtInterestRate = *is.NonOperatingSection.InterestExpense.Value / totalDebt
			}

			defaults.CustomItems = make(map[string]float64)

			// Other Operating Expenses %
			if is.OperatingCostSection != nil {
				if is.OperatingCostSection.OtherOperatingExpenses != nil && is.OperatingCostSection.OtherOperatingExpenses.Value != nil {
					defaults.OtherOperatingPercent = *is.OperatingCostSection.OtherOperatingExpenses.Value / rev
				}
				// Dynamic Operating Items
				for _, item := range is.OperatingCostSection.AdditionalItems {
					if item.Value != nil && item.Value.Value != nil {
						defaults.CustomItems[item.Label] = *item.Value.Value / rev
					}
				}
			}

			// Non-Operating Items (Other Income/Expense, Equity Affiliates)
			if is.NonOperatingSection != nil {
				if is.NonOperatingSection.OtherIncomeExpense != nil && is.NonOperatingSection.OtherIncomeExpense.Value != nil {
					defaults.OtherNonOpPercent = *is.NonOperatingSection.OtherIncomeExpense.Value / rev
				}
				if is.NonOperatingSection.EquityAffiliatesNonOperating != nil && is.NonOperatingSection.EquityAffiliatesNonOperating.Value != nil {
					defaults.EquityAffiliatesPct = *is.NonOperatingSection.EquityAffiliatesNonOperating.Value / rev
				}
				// Dynamic Non-Operating Items
				for _, item := range is.NonOperatingSection.AdditionalItems {
					if item.Value != nil && item.Value.Value != nil {
						defaults.CustomItems[item.Label] = *item.Value.Value / rev
					}
				}
			}

			// Discontinued Operations (TaxAdjustments section)
			if is.TaxAdjustments != nil && is.TaxAdjustments.DiscontinuedOperations != nil && is.TaxAdjustments.DiscontinuedOperations.Value != nil {
				defaults.DiscontinuedOpsPercent = *is.TaxAdjustments.DiscontinuedOperations.Value / rev
			}

			// --- BALANCE SHEET DRIVERS (% of Revenue) ---
			// Receivables (Net)
			if bs.CurrentAssets.AccountsReceivableNet != nil && bs.CurrentAssets.AccountsReceivableNet.Value != nil {
				defaults.ReceivablesPercent = *bs.CurrentAssets.AccountsReceivableNet.Value / rev
			}
			// Inventory
			if bs.CurrentAssets.Inventories != nil && bs.CurrentAssets.Inventories.Value != nil {
				defaults.InventoryPercent = *bs.CurrentAssets.Inventories.Value / rev
			}
			// Accounts Payable
			if bs.CurrentLiabilities.AccountsPayable != nil && bs.CurrentLiabilities.AccountsPayable.Value != nil {
				defaults.APPercent = *bs.CurrentLiabilities.AccountsPayable.Value / rev
			}
			// Deferred Revenue (Short Term)
			if bs.CurrentLiabilities.DeferredRevenueCurrent != nil && bs.CurrentLiabilities.DeferredRevenueCurrent.Value != nil {
				defaults.DeferredRevPercent = *bs.CurrentLiabilities.DeferredRevenueCurrent.Value / rev
			}

			// Dynamic Balance Sheet Items (Assets - AdditionalItems is []FSAPValue)
			for _, item := range bs.CurrentAssets.AdditionalItems {
				if item.Value != nil {
					defaults.CustomItems[item.Label+" (BS-Asset)"] = *item.Value / rev
				}
			}

			// Dynamic Balance Sheet Items (Liabilities - AdditionalItems is []FSAPValue)
			for _, item := range bs.CurrentLiabilities.AdditionalItems {
				if item.Value != nil {
					defaults.CustomItems[item.Label+" (BS-Liab)"] = *item.Value / rev
				}
			}

			// Diagnostics log
			fmt.Printf("[Defaults] Calculated: COGS=%.1f%%, SG&A=%.1f%%, R&D=%.1f%%, Tax=%.1f%%, SBC=%.1f%%, Int=%.1f%%, CapEx=%.1f%%, Div=%.1f%%, Buyback=%.1f%%, OtherOp=%.1f%%, OtherNon=%.1f%%, EqAff=%.1f%%, DiscOps=%.1f%%\n",
				defaults.COGSPercent*100, defaults.SGAPercent*100, defaults.RDPercent*100, defaults.TaxRate*100,
				defaults.StockBasedCompPercent*100, defaults.DebtInterestRate*100,
				defaults.CapExPercent*100, defaults.DividendsPercent*100, defaults.RepurchasePercent*100,
				defaults.OtherOperatingPercent*100, defaults.OtherNonOpPercent*100, defaults.EquityAffiliatesPct*100, defaults.DiscontinuedOpsPercent*100)
		}
	}

	return defaults
}

