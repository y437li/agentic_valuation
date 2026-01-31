package projection

// =============================================================================
// STANDARD SKELETON (Fixed Parent Nodes)
// These nodes are defined by Go code and cannot be deleted by AI
// AI can only attach/detach drivers and switch strategies
// =============================================================================

// StandardSkeleton defines the fixed financial model structure
// This ensures accounting identity (A = L + E) is always maintained
type StandardSkeleton struct {
	// Income Statement Flow
	Revenue     *Node `json:"revenue"`
	COGS        *Node `json:"cogs"`
	GrossProfit *Node `json:"gross_profit"` // = Revenue - COGS

	// OpEx Breakdown
	SGA              *Node `json:"sga"` // Aggregate if detailed not used
	SellingMarketing *Node `json:"selling_marketing"`
	GeneralAdmin     *Node `json:"general_admin"`
	RD               *Node `json:"rd"`
	OtherOperating   *Node `json:"other_operating"`
	OperatingIncome  *Node `json:"operating_income"` // = GP - OpEx

	InterestExpense *Node `json:"interest_expense"`
	OtherNonOp      *Node `json:"other_non_op"`      // New: FX, etc.
	IncomeBeforeTax *Node `json:"income_before_tax"` // = OpInc - Interest
	TaxExpense      *Node `json:"tax_expense"`

	// Net Income Attribution
	MinorityInterestExp *Node `json:"minority_interest_expense"` // Income attribution
	NetIncome           *Node `json:"net_income"`                // = IBT - Tax

	// Explicit Depreciation Node
	Depreciation *Node `json:"depreciation"`

	// Balance Sheet (Assets)
	// Current
	Cash                 *Node `json:"cash"`
	ShortTermInvestments *Node `json:"short_term_investments"`
	AccountsReceivable   *Node `json:"accounts_receivable"`
	Inventory            *Node `json:"inventory"`
	OtherCurrentAssets   *Node `json:"other_current_assets"`
	TotalCurrentAssets   *Node `json:"total_current_assets"`

	// Non-Current
	PPEAtCost               *Node `json:"ppe_at_cost"`
	AccumulatedDepreciation *Node `json:"accumulated_depreciation"`
	PPENet                  *Node `json:"ppe_net"`
	Goodwill                *Node `json:"goodwill"`
	Intangibles             *Node `json:"intangibles"`
	LongTermInvestments     *Node `json:"long_term_investments"`
	DeferredTaxAssets       *Node `json:"deferred_tax_assets"`
	OtherNonCurrentAssets   *Node `json:"other_non_current_assets"`
	TotalNonCurrentAssets   *Node `json:"total_non_current_assets"`

	TotalAssets *Node `json:"total_assets"`

	// Balance Sheet (Liabilities)
	// Current
	AccountsPayable         *Node `json:"accounts_payable"`
	AccruedLiabilities      *Node `json:"accrued_liabilities"`
	ShortTermDebt           *Node `json:"short_term_debt"` // Revolver/Current Portion
	OtherCurrentLiabilities *Node `json:"other_current_liabilities"`
	TotalCurrentLiabilities *Node `json:"total_current_liabilities"`

	// Non-Current
	LongTermDebt               *Node `json:"long_term_debt"`
	DeferredTaxLiabilities     *Node `json:"deferred_tax_liabilities"`
	OtherNonCurrentLiabilities *Node `json:"other_non_current_liabilities"`
	TotalNonCurrentLiabilities *Node `json:"total_non_current_liabilities"`

	TotalLiabilities *Node `json:"total_liabilities"`

	// Balance Sheet (Equity)
	CommonStock      *Node `json:"common_stock"`
	PreferredStock   *Node `json:"preferred_stock"`
	RetainedEarnings *Node `json:"retained_earnings"`
	TreasuryStock    *Node `json:"treasury_stock"`
	AOCI             *Node `json:"aoci"`
	MinorityInterest *Node `json:"minority_interest"`
	TotalEquity      *Node `json:"total_equity"`

	// Working Capital Drivers
	DSO *Node `json:"dso"` // Days Sales Outstanding
	DSI *Node `json:"dsi"` // Days Sales Inventory
	DPO *Node `json:"dpo"` // Days Payable Outstanding

	// CapEx
	CapEx *Node `json:"capex"`
}

// NewStandardSkeleton creates a skeleton with all fixed nodes initialized
// Each node starts with default GrowthStrategy
func NewStandardSkeleton() *StandardSkeleton {
	defaultStrategy := &GrowthStrategy{GrowthRate: 0.0}

	createNode := func(id, name string) *Node {
		return &Node{
			ID:           id,
			Name:         name,
			Type:         NodeTypeSkeleton,
			Strategy:     defaultStrategy,
			StrategyName: defaultStrategy.Name(),
			Values:       make(map[int]float64),
			UpdatedBy:    "SYSTEM",
		}
	}

	return &StandardSkeleton{
		// Income Statement
		Revenue:             createNode("revenue", "Total Revenue"),
		COGS:                createNode("cogs", "Cost of Goods Sold"),
		GrossProfit:         createNode("gross_profit", "Gross Profit"),
		SGA:                 createNode("sga", "SG&A Expenses"),
		SellingMarketing:    createNode("selling_marketing", "Selling & Marketing"),
		GeneralAdmin:        createNode("general_admin", "General & Admin"),
		RD:                  createNode("rd", "R&D Expenses"),
		OtherOperating:      createNode("other_operating", "Other Operating Expenses"),
		OperatingIncome:     createNode("operating_income", "Operating Income"),
		InterestExpense:     createNode("interest_expense", "Interest Expense"),
		OtherNonOp:          createNode("other_non_op", "Other Non-Operating Inc/Exp"),
		IncomeBeforeTax:     createNode("income_before_tax", "Income Before Tax"),
		TaxExpense:          createNode("tax_expense", "Tax Expense"),
		MinorityInterestExp: createNode("minority_interest_expense", "Minority Interest Expense"),
		NetIncome:           createNode("net_income", "Net Income"),
		Depreciation:        createNode("depreciation", "Depreciation Expense"),

		// Balance Sheet - Assets
		Cash:                 createNode("cash", "Cash & Equivalents"),
		ShortTermInvestments: createNode("short_term_investments", "Short Term Investments"),
		AccountsReceivable:   createNode("accounts_receivable", "Accounts Receivable"),
		Inventory:            createNode("inventory", "Inventory"),
		OtherCurrentAssets:   createNode("other_current_assets", "Other Current Assets"),
		TotalCurrentAssets:   createNode("total_current_assets", "Total Current Assets"),

		PPEAtCost:               createNode("ppe_at_cost", "PPE At Cost"),
		AccumulatedDepreciation: createNode("accumulated_depreciation", "Accumulated Depreciation"),
		PPENet:                  createNode("ppe_net", "Net PPE"),
		Goodwill:                createNode("goodwill", "Goodwill"),
		Intangibles:             createNode("intangibles", "Intangible Assets"),
		LongTermInvestments:     createNode("long_term_investments", "Long Term Investments"),
		DeferredTaxAssets:       createNode("deferred_tax_assets", "Deferred Tax Assets"),
		OtherNonCurrentAssets:   createNode("other_non_current_assets", "Other Non-Current Assets"),
		TotalNonCurrentAssets:   createNode("total_non_current_assets", "Total Non-Current Assets"),
		TotalAssets:             createNode("total_assets", "Total Assets"),

		// Balance Sheet - Liabilities
		AccountsPayable:         createNode("accounts_payable", "Accounts Payable"),
		AccruedLiabilities:      createNode("accrued_liabilities", "Accrued Liabilities"),
		ShortTermDebt:           createNode("short_term_debt", "Short Term Debt"),
		OtherCurrentLiabilities: createNode("other_current_liabilities", "Other Current Liabilities"),
		TotalCurrentLiabilities: createNode("total_current_liabilities", "Total Current Liabilities"),

		LongTermDebt:               createNode("long_term_debt", "Long Term Debt"),
		DeferredTaxLiabilities:     createNode("deferred_tax_liabilities", "Deferred Tax Liabilities"),
		OtherNonCurrentLiabilities: createNode("other_non_current_liabilities", "Other Non-Current Liabilities"),
		TotalNonCurrentLiabilities: createNode("total_non_current_liabilities", "Total Non-Current Liabilities"),
		TotalLiabilities:           createNode("total_liabilities", "Total Liabilities"),

		// Balance Sheet - Equity
		CommonStock:      createNode("common_stock", "Common Stock & APIC"),
		PreferredStock:   createNode("preferred_stock", "Preferred Stock"),
		RetainedEarnings: createNode("retained_earnings", "Retained Earnings"),
		TreasuryStock:    createNode("treasury_stock", "Treasury Stock"),
		AOCI:             createNode("aoci", "Accum Other Comp Income"),
		MinorityInterest: createNode("minority_interest", "Minority Interest"),
		TotalEquity:      createNode("total_equity", "Total Equity"),

		// Working Capital
		DSO: createNode("dso", "Days Sales Outstanding"),
		DSI: createNode("dsi", "Days Sales Inventory"),
		DPO: createNode("dpo", "Days Payable Outstanding"),

		// CapEx
		CapEx: createNode("capex", "Capital Expenditures"),
	}
}

// GetAllNodes returns all skeleton nodes as a map for easy lookup
func (s *StandardSkeleton) GetAllNodes() map[string]*Node {
	return map[string]*Node{
		"revenue":                   s.Revenue,
		"cogs":                      s.COGS,
		"gross_profit":              s.GrossProfit,
		"sga":                       s.SGA,
		"selling_marketing":         s.SellingMarketing,
		"general_admin":             s.GeneralAdmin,
		"rd":                        s.RD,
		"other_operating":           s.OtherOperating,
		"operating_income":          s.OperatingIncome,
		"interest_expense":          s.InterestExpense,
		"other_non_op":              s.OtherNonOp,
		"income_before_tax":         s.IncomeBeforeTax,
		"tax_expense":               s.TaxExpense,
		"minority_interest_expense": s.MinorityInterestExp,
		"net_income":                s.NetIncome,
		"depreciation":              s.Depreciation,

		"cash":                   s.Cash,
		"short_term_investments": s.ShortTermInvestments,
		"accounts_receivable":    s.AccountsReceivable,
		"inventory":              s.Inventory,
		"other_current_assets":   s.OtherCurrentAssets,
		"total_current_assets":   s.TotalCurrentAssets,

		"ppe_at_cost":              s.PPEAtCost,
		"accumulated_depreciation": s.AccumulatedDepreciation,
		"ppe_net":                  s.PPENet,
		"goodwill":                 s.Goodwill,
		"intangibles":              s.Intangibles,
		"long_term_investments":    s.LongTermInvestments,
		"deferred_tax_assets":      s.DeferredTaxAssets,
		"other_non_current_assets": s.OtherNonCurrentAssets,
		"total_non_current_assets": s.TotalNonCurrentAssets,
		"total_assets":             s.TotalAssets,

		"accounts_payable":          s.AccountsPayable,
		"accrued_liabilities":       s.AccruedLiabilities,
		"short_term_debt":           s.ShortTermDebt,
		"other_current_liabilities": s.OtherCurrentLiabilities,
		"total_current_liabilities": s.TotalCurrentLiabilities,

		"long_term_debt":                s.LongTermDebt,
		"deferred_tax_liabilities":      s.DeferredTaxLiabilities,
		"other_non_current_liabilities": s.OtherNonCurrentLiabilities,
		"total_non_current_liabilities": s.TotalNonCurrentLiabilities,
		"total_liabilities":             s.TotalLiabilities,

		"common_stock":      s.CommonStock,
		"preferred_stock":   s.PreferredStock,
		"retained_earnings": s.RetainedEarnings,
		"treasury_stock":    s.TreasuryStock,
		"aoci":              s.AOCI,
		"minority_interest": s.MinorityInterest,
		"total_equity":      s.TotalEquity,

		"dso":   s.DSO,
		"dsi":   s.DSI,
		"dpo":   s.DPO,
		"capex": s.CapEx,
	}
}

// ValidateSkeletonIDs returns list of valid skeleton node IDs
// AI cannot create nodes with these IDs
func ValidateSkeletonIDs() []string {
	return []string{
		"revenue", "cogs", "gross_profit", "sga", "selling_marketing", "general_admin",
		"rd", "other_operating", "operating_income", "interest_expense", "other_non_op",
		"income_before_tax", "tax_expense", "minority_interest_expense", "net_income",
		"depreciation",

		"cash", "short_term_investments", "accounts_receivable", "inventory",
		"other_current_assets", "total_current_assets",

		"ppe_at_cost", "accumulated_depreciation", "ppe_net", "goodwill", "intangibles",
		"long_term_investments", "deferred_tax_assets", "other_non_current_assets",
		"total_non_current_assets", "total_assets",

		"accounts_payable", "accrued_liabilities", "short_term_debt", "other_current_liabilities",
		"total_current_liabilities",

		"long_term_debt", "deferred_tax_liabilities", "other_non_current_liabilities",
		"total_non_current_liabilities", "total_liabilities",

		"common_stock", "preferred_stock", "retained_earnings", "treasury_stock",
		"aoci", "minority_interest", "total_equity",

		"dso", "dsi", "dpo", "capex",
	}
}

// IsSkeletonID checks if an ID is reserved for skeleton nodes
func IsSkeletonID(id string) bool {
	for _, skeletonID := range ValidateSkeletonIDs() {
		if id == skeletonID {
			return true
		}
	}
	return false
}
