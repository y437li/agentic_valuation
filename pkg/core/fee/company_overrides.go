// Package fee - Company Override Mapping System
// Provides layered mapping: Generic GAAP → Industry → Company-specific overrides
package fee

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// =============================================================================
// LAYERED MAPPING ARCHITECTURE
// =============================================================================
//
// Layer 1: Generic GAAP patterns (section_router.go - always applied)
// Layer 2: Industry templates (banking, insurance, technology, etc.)
// Layer 3: Company-specific overrides (per CIK)
//
// Resolution order: Company → Industry → Generic

// IndustryType identifies the industry for template selection
type IndustryType string

const (
	IndustryGeneral    IndustryType = "general"
	IndustryBanking    IndustryType = "banking"
	IndustryInsurance  IndustryType = "insurance"
	IndustryTechnology IndustryType = "technology"
	IndustryRetail     IndustryType = "retail"
	IndustryEnergy     IndustryType = "energy"
	IndustryHealthcare IndustryType = "healthcare"
	IndustryREIT       IndustryType = "reit"
)

// =============================================================================
// COMPANY OVERRIDE CONFIGURATION
// =============================================================================

// CompanyOverride contains company-specific mapping overrides
type CompanyOverride struct {
	CIK           string            `json:"cik"`
	Ticker        string            `json:"ticker"`
	CompanyName   string            `json:"company_name"`
	Industry      IndustryType      `json:"industry"`
	LabelMappings map[string]string `json:"label_mappings"` // "Automotive receivables" → "accounts_receivable_net"
	SkipLabels    []string          `json:"skip_labels"`    // Labels to ignore (e.g., "Deferred revenue - non-automotive")
	Notes         string            `json:"notes"`          // Why this override exists
	LastUpdated   string            `json:"last_updated"`
}

// IndustryTemplate contains industry-specific patterns
type IndustryTemplate struct {
	Industry      IndustryType        `json:"industry"`
	Description   string              `json:"description"`
	LabelMappings map[string]string   `json:"label_mappings"`
	ExtraPatterns map[string][]string `json:"extra_patterns"` // Additional patterns for this industry
}

// =============================================================================
// OVERRIDE REGISTRY
// =============================================================================

// OverrideRegistry manages all override configurations
type OverrideRegistry struct {
	mu                sync.RWMutex
	companyOverrides  map[string]*CompanyOverride // CIK → Override
	industryTemplates map[IndustryType]*IndustryTemplate
	configPath        string
}

// NewOverrideRegistry creates a new registry, optionally loading from disk
func NewOverrideRegistry(configPath string) *OverrideRegistry {
	reg := &OverrideRegistry{
		companyOverrides:  make(map[string]*CompanyOverride),
		industryTemplates: make(map[IndustryType]*IndustryTemplate),
		configPath:        configPath,
	}

	// Initialize default industry templates
	reg.initDefaultTemplates()

	// Load saved overrides if config path provided
	if configPath != "" {
		reg.loadFromDisk()
	}

	return reg
}

// initDefaultTemplates sets up built-in industry templates
func (r *OverrideRegistry) initDefaultTemplates() {
	r.industryTemplates[IndustryBanking] = &IndustryTemplate{
		Industry:    IndustryBanking,
		Description: "Banks, Credit Unions, Financial Institutions",
		LabelMappings: map[string]string{
			"Loans and leases":            "finance_div_loans_leases_st",
			"Loans and lease financing":   "finance_div_loans_leases_st",
			"Net loans":                   "finance_div_loans_leases_st",
			"Interest-bearing deposits":   "cash_and_equivalents",
			"Investment securities":       "short_term_investments",
			"Allowance for loan losses":   "allowance_for_doubtful_accounts",
			"Trading assets":              "short_term_investments",
			"Deposits":                    "notes_payable_short_term_debt",
			"Federal funds purchased":     "notes_payable_short_term_debt",
			"Interest income":             "revenues",
			"Interest expense":            "interest_expense",
			"Provision for credit losses": "cost_of_goods_sold",
			"Net interest income":         "gross_profit",
		},
	}

	r.industryTemplates[IndustryInsurance] = &IndustryTemplate{
		Industry:    IndustryInsurance,
		Description: "Insurance Companies, Reinsurers",
		LabelMappings: map[string]string{
			"Premiums earned":                 "revenues",
			"Net premiums earned":             "revenues",
			"Policy liabilities":              "other_noncurrent_liabilities_1",
			"Unpaid losses and loss expenses": "accrued_liabilities",
			"Deferred policy acquisition":     "other_current_assets_1",
			"Investment income":               "other_income",
			"Claims and benefits":             "cost_of_goods_sold",
		},
	}

	r.industryTemplates[IndustryREIT] = &IndustryTemplate{
		Industry:    IndustryREIT,
		Description: "Real Estate Investment Trusts",
		LabelMappings: map[string]string{
			"Real estate investments":         "ppe_net",
			"Real estate held for investment": "ppe_net",
			"Rental income":                   "revenues",
			"Rental revenues":                 "revenues",
			"Funds from operations":           "operating_cash_flow",
		},
	}

	r.industryTemplates[IndustryTechnology] = &IndustryTemplate{
		Industry:    IndustryTechnology,
		Description: "Technology, Software, SaaS Companies",
		LabelMappings: map[string]string{
			"Subscription revenue":     "revenues",
			"License revenue":          "revenues",
			"Stock-based compensation": "stock_based_compensation",
			"Capitalized software":     "intangibles",
			"Research and development": "rd_expenses",
		},
	}
}

// =============================================================================
// LOOKUP METHODS
// =============================================================================

// GetCompanyOverride returns override for a specific company
func (r *OverrideRegistry) GetCompanyOverride(cik string) *CompanyOverride {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.companyOverrides[padCIK(cik)]
}

// GetIndustryTemplate returns template for an industry
func (r *OverrideRegistry) GetIndustryTemplate(industry IndustryType) *IndustryTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.industryTemplates[industry]
}

// ResolveMapping finds the best FSAP variable for a label using layered lookup
func (r *OverrideRegistry) ResolveMapping(cik string, label string) (fsapVar string, source string, found bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalizedLabel := normalizeLabel(label)

	// Layer 1: Company-specific override
	if override := r.companyOverrides[padCIK(cik)]; override != nil {
		// Check skip list
		for _, skip := range override.SkipLabels {
			if strings.EqualFold(normalizedLabel, normalizeLabel(skip)) {
				return "", "skip", true // Explicitly skip this label
			}
		}
		// Check mappings
		for labelPattern, fsap := range override.LabelMappings {
			if strings.EqualFold(normalizedLabel, normalizeLabel(labelPattern)) {
				return fsap, "company_override", true
			}
		}

		// Layer 2: Industry template (if company has industry set)
		if template := r.industryTemplates[override.Industry]; template != nil {
			for labelPattern, fsap := range template.LabelMappings {
				if strings.EqualFold(normalizedLabel, normalizeLabel(labelPattern)) {
					return fsap, "industry_template", true
				}
			}
		}
	}

	// Layer 3: No override found, fall back to generic patterns
	return "", "generic", false
}

// =============================================================================
// MANAGEMENT METHODS
// =============================================================================

// AddCompanyOverride adds or updates a company override
func (r *OverrideRegistry) AddCompanyOverride(override *CompanyOverride) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.companyOverrides[padCIK(override.CIK)] = override
}

// RemoveCompanyOverride removes a company override
func (r *OverrideRegistry) RemoveCompanyOverride(cik string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.companyOverrides, padCIK(cik))
}

// ListCompanyOverrides returns all company overrides
func (r *OverrideRegistry) ListCompanyOverrides() []*CompanyOverride {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*CompanyOverride, 0, len(r.companyOverrides))
	for _, o := range r.companyOverrides {
		result = append(result, o)
	}
	return result
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// SaveToDisk persists all overrides to disk
func (r *OverrideRegistry) SaveToDisk() error {
	if r.configPath == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	data := struct {
		CompanyOverrides  []*CompanyOverride  `json:"company_overrides"`
		IndustryTemplates []*IndustryTemplate `json:"industry_templates"`
	}{
		CompanyOverrides:  r.ListCompanyOverrides(),
		IndustryTemplates: make([]*IndustryTemplate, 0),
	}

	for _, t := range r.industryTemplates {
		data.IndustryTemplates = append(data.IndustryTemplates, t)
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.configPath, bytes, 0644)
}

// loadFromDisk loads overrides from disk
func (r *OverrideRegistry) loadFromDisk() error {
	if r.configPath == "" {
		return nil
	}

	bytes, err := os.ReadFile(r.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file yet, ok
		}
		return err
	}

	var data struct {
		CompanyOverrides  []*CompanyOverride  `json:"company_overrides"`
		IndustryTemplates []*IndustryTemplate `json:"industry_templates"`
	}

	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	for _, o := range data.CompanyOverrides {
		r.companyOverrides[padCIK(o.CIK)] = o
	}

	for _, t := range data.IndustryTemplates {
		r.industryTemplates[t.Industry] = t
	}

	return nil
}

// =============================================================================
// HELPERS
// =============================================================================

// normalizeLabel cleans up a label for comparison
func normalizeLabel(label string) string {
	// Lowercase, trim, collapse whitespace
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.Join(strings.Fields(label), " ")
	// Remove trailing commas, colons, etc.
	label = strings.TrimRight(label, ",:;")
	return label
}

// padCIK pads CIK to 10 digits
func padCIK(cik string) string {
	cik = strings.TrimLeft(cik, "0")
	for len(cik) < 10 {
		cik = "0" + cik
	}
	return cik
}

// =============================================================================
// DEFAULT OVERRIDES (EXAMPLE)
// =============================================================================

// GetDefaultConfigPath returns the default path for override config
func GetDefaultConfigPath() string {
	// Store in project data directory
	return filepath.Join("data", "fee_overrides.json")
}

// CreateExampleOverrides creates some example overrides for testing
func CreateExampleOverrides() []*CompanyOverride {
	return []*CompanyOverride{
		{
			CIK:         "0000037996",
			Ticker:      "F",
			CompanyName: "Ford Motor Company",
			Industry:    IndustryGeneral,
			LabelMappings: map[string]string{
				"Ford Credit receivables":                "finance_div_loans_leases_st",
				"Automotive receivables":                 "accounts_receivable_net",
				"Ford Credit debt":                       "long_term_debt",
				"Dealer and customer financing":          "finance_div_loans_leases_lt",
				"Cash and cash equivalents (Automotive)": "cash_and_equivalents",
			},
			SkipLabels: []string{
				"Deferred revenue (Ford Credit)",
			},
			Notes:       "Ford has significant Ford Credit segment requiring special handling",
			LastUpdated: "2026-01-19",
		},
		{
			CIK:         "0000320193",
			Ticker:      "AAPL",
			CompanyName: "Apple Inc.",
			Industry:    IndustryTechnology,
			LabelMappings: map[string]string{
				"Marketable securities – current":     "short_term_investments",
				"Marketable securities – non-current": "long_term_investments",
				"Vendor non-trade receivables":        "other_current_assets_1",
				"Term debt":                           "long_term_debt",
			},
			Notes:       "Apple has distinct marketable securities categories",
			LastUpdated: "2026-01-19",
		},
	}
}
