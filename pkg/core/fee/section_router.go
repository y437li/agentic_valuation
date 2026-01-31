// Package fee - Section Router for financial table identification
package fee

import (
	"regexp"
	"strings"
)

// =============================================================================
// SECTION ROUTER - Fuzzy matching for financial table type detection
// =============================================================================

// TableMatcher contains patterns for identifying financial table types
type TableMatcher struct {
	patterns map[TableType][]string
	avoids   map[TableType][]string // Patterns that indicate wrong section
}

// NewTableMatcher creates a new matcher with SEC 10-K patterns
func NewTableMatcher() *TableMatcher {
	return &TableMatcher{
		patterns: map[TableType][]string{
			TableTypeBalanceSheet: {
				`(?i)consolidated\s+balance\s+sheets?`,
				`(?i)balance\s+sheets?`,
				`(?i)statements?\s+of\s+financial\s+position`,
				`(?i)condensed\s+balance\s+sheets?`,
			},
			TableTypeIncomeStatement: {
				`(?i)consolidated\s+statements?\s+of\s+operations`,
				`(?i)consolidated\s+statements?\s+of\s+income`,
				`(?i)statements?\s+of\s+operations`,
				`(?i)statements?\s+of\s+income`,
				`(?i)statements?\s+of\s+earnings`,
				`(?i)income\s+statements?`,
			},
			TableTypeCashFlow: {
				`(?i)consolidated\s+statements?\s+of\s+cash\s+flows?`,
				`(?i)statements?\s+of\s+cash\s+flows?`,
				`(?i)cash\s+flow\s+statements?`,
			},
			TableTypeComprehensiveIncome: {
				`(?i)consolidated\s+statements?\s+of\s+comprehensive\s+income`,
				`(?i)statements?\s+of\s+comprehensive\s+income`,
			},
			TableTypeEquity: {
				`(?i)consolidated\s+statements?\s+of\s+stockholders`,
				`(?i)consolidated\s+statements?\s+of\s+shareholders`,
				`(?i)statements?\s+of\s+changes\s+in\s+equity`,
				`(?i)statements?\s+of\s+equity`,
			},
		},
		avoids: map[TableType][]string{
			TableTypeBalanceSheet: {
				`(?i)parent\s+company`,
				`(?i)registrant\s+only`,
				`(?i)schedule\s+i`,
				`(?i)supplemental\s+consolidating`,
			},
			TableTypeIncomeStatement: {
				`(?i)parent\s+company`,
				`(?i)registrant\s+only`,
				`(?i)schedule\s+i`,
			},
			TableTypeCashFlow: {
				`(?i)parent\s+company`,
				`(?i)registrant\s+only`,
			},
		},
	}
}

// IdentifyTableType determines the type of a table from its title
func (m *TableMatcher) IdentifyTableType(title string, context string) TableType {
	// Check each table type
	for tableType, patterns := range m.patterns {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(title) {
				// Check if we should avoid this match
				if !m.shouldAvoid(tableType, title, context) {
					return tableType
				}
			}
		}
	}
	return TableTypeUnknown
}

// shouldAvoid checks if the match is in an excluded section
func (m *TableMatcher) shouldAvoid(tableType TableType, title, context string) bool {
	avoidPatterns, ok := m.avoids[tableType]
	if !ok {
		return false
	}

	combined := title + " " + context
	for _, pattern := range avoidPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(combined) {
			return true
		}
	}
	return false
}

// IsConsolidated checks if a table title indicates consolidated statements
func IsConsolidated(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "consolidated")
}

// =============================================================================
// ROW ANALYSIS - Identify subtotals, headers, and line item types
// =============================================================================

// RowClassifier classifies table rows by their purpose
type RowClassifier struct {
	totalPatterns  []string
	headerPatterns []string
}

// NewRowClassifier creates a classifier with common financial patterns
func NewRowClassifier() *RowClassifier {
	return &RowClassifier{
		totalPatterns: []string{
			`(?i)^total\s+`,
			`(?i)\s+total$`,
			`(?i)^subtotal`,
			`(?i)^net\s+`,
			`(?i)^gross\s+`,
		},
		headerPatterns: []string{
			`(?i)^assets\s*$`,
			`(?i)^liabilities\s*$`,
			`(?i)^equity\s*$`,
			`(?i)^current\s+assets\s*:?\s*$`,
			`(?i)^noncurrent\s+assets\s*:?\s*$`,
			`(?i)^current\s+liabilities\s*:?\s*$`,
			`(?i)^revenues?\s*:?\s*$`,
			`(?i)^expenses?\s*:?\s*$`,
			`(?i)^operating\s+activities\s*:?\s*$`,
			`(?i)^investing\s+activities\s*:?\s*$`,
			`(?i)^financing\s+activities\s*:?\s*$`,
		},
	}
}

// ClassifyRow determines if a row is a total, header, or regular line item
func (c *RowClassifier) ClassifyRow(label string) (isTotal bool, isHeader bool) {
	label = strings.TrimSpace(label)

	for _, pattern := range c.totalPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(label) {
			return true, false
		}
	}

	for _, pattern := range c.headerPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(label) {
			return false, true
		}
	}

	return false, false
}

// DetectIndentLevel estimates indentation level from label formatting
func DetectIndentLevel(label string, originalText string) int {
	// Count leading spaces in original text
	spaces := 0
	for _, ch := range originalText {
		if ch == ' ' || ch == '\t' {
			spaces++
		} else {
			break
		}
	}

	// Convert to indent level (rough heuristic: 2-3 spaces per level)
	return spaces / 3
}

// =============================================================================
// FINANCIAL LINE ITEM MATCHER - Map row labels to FSAP variables
// =============================================================================

// FSAPMappingCandidate represents a possible mapping from row to FSAP variable
type FSAPMappingCandidate struct {
	FSAPVariable string  // e.g., "cash_and_equivalents"
	Confidence   float64 // 0.0 - 1.0
	RowLabel     string  // Original row label
	MatchedRule  string  // Which pattern matched
}

// FSAPMapper maps financial line items to FSAP schema variables
type FSAPMapper struct {
	bsAssetPatterns     map[string][]string
	bsLiabilityPatterns map[string][]string
	bsEquityPatterns    map[string][]string
	isPatterns          map[string][]string
	cfPatterns          map[string][]string
}

// NewFSAPMapper creates a mapper with comprehensive patterns
func NewFSAPMapper() *FSAPMapper {
	return &FSAPMapper{
		bsAssetPatterns: map[string][]string{
			"cash_and_equivalents": {
				`(?i)^cash\s+and\s+(cash\s+)?equivalents`,
				`(?i)^cash$`,
			},
			"short_term_investments": {
				`(?i)marketable\s+securities`,
				`(?i)short.?term\s+investments`,
				`(?i)available.?for.?sale\s+securities`,
			},
			"accounts_receivable_net": {
				`(?i)accounts?\s+receivable`,
				`(?i)trade\s+receivables?`,
				`(?i)receivables?,?\s*net`,
			},
			"inventories": {
				`(?i)^inventor(y|ies)`,
			},
			"ppe_net": {
				`(?i)property.*(net|less)`,
				`(?i)property,?\s+plant`,
			},
			"goodwill": {
				`(?i)^goodwill$`,
			},
			"intangibles": {
				`(?i)intangible\s+assets`,
				`(?i)definite.?lived\s+intangibles`,
			},
		},
		bsLiabilityPatterns: map[string][]string{
			"accounts_payable": {
				`(?i)accounts?\s+payable`,
				`(?i)trade\s+payables?`,
			},
			"accrued_liabilities": {
				`(?i)accrued\s+(liabilities|expenses)`,
				`(?i)other\s+accrued`,
			},
			"long_term_debt": {
				`(?i)long.?term\s+debt`,
				`(?i)debt,?\s+non.?current`,
			},
		},
		bsEquityPatterns: map[string][]string{
			"common_stock": {
				`(?i)common\s+(stock|shares)`,
			},
			"retained_earnings": {
				`(?i)retained\s+earnings`,
				`(?i)accumulated\s+(deficit|earnings)`,
			},
		},
		isPatterns: map[string][]string{
			"revenues": {
				`(?i)^revenues?$`,
				`(?i)^net\s+(sales|revenues?)`,
				`(?i)^total\s+revenues?$`,
			},
			"cost_of_goods_sold": {
				`(?i)cost\s+of\s+(goods\s+)?sold`,
				`(?i)cost\s+of\s+revenues?`,
				`(?i)costs?\s+of\s+sales`,
			},
			"net_income": {
				`(?i)^net\s+income`,
				`(?i)net\s+(income|loss)$`,
			},
		},
		cfPatterns: map[string][]string{
			"depreciation_amortization": {
				`(?i)depreciation\s+(and|&)\s+amortization`,
				`(?i)depreciation`,
			},
			"capex": {
				`(?i)capital\s+expenditures?`,
				`(?i)purchases?\s+of\s+property`,
			},
		},
	}
}

// MapRowToFSAP finds the best FSAP variable match for a row label
func (m *FSAPMapper) MapRowToFSAP(label string, tableType TableType) []FSAPMappingCandidate {
	var candidates []FSAPMappingCandidate

	var patterns map[string][]string
	switch tableType {
	case TableTypeBalanceSheet:
		// Check all BS patterns
		for variable, pats := range m.bsAssetPatterns {
			if conf, rule := matchPatterns(label, pats); conf > 0 {
				candidates = append(candidates, FSAPMappingCandidate{
					FSAPVariable: variable,
					Confidence:   conf,
					RowLabel:     label,
					MatchedRule:  rule,
				})
			}
		}
		for variable, pats := range m.bsLiabilityPatterns {
			if conf, rule := matchPatterns(label, pats); conf > 0 {
				candidates = append(candidates, FSAPMappingCandidate{
					FSAPVariable: variable,
					Confidence:   conf,
					RowLabel:     label,
					MatchedRule:  rule,
				})
			}
		}
		for variable, pats := range m.bsEquityPatterns {
			if conf, rule := matchPatterns(label, pats); conf > 0 {
				candidates = append(candidates, FSAPMappingCandidate{
					FSAPVariable: variable,
					Confidence:   conf,
					RowLabel:     label,
					MatchedRule:  rule,
				})
			}
		}
	case TableTypeIncomeStatement:
		patterns = m.isPatterns
	case TableTypeCashFlow:
		patterns = m.cfPatterns
	}

	if patterns != nil {
		for variable, pats := range patterns {
			if conf, rule := matchPatterns(label, pats); conf > 0 {
				candidates = append(candidates, FSAPMappingCandidate{
					FSAPVariable: variable,
					Confidence:   conf,
					RowLabel:     label,
					MatchedRule:  rule,
				})
			}
		}
	}

	return candidates
}

// matchPatterns checks if label matches any pattern, returns confidence
func matchPatterns(label string, patterns []string) (confidence float64, matchedRule string) {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(label) {
			// First pattern = highest confidence
			return 0.9, pattern
		}
	}
	return 0, ""
}
