package edgar

import (
	"agentic_valuation/pkg/core/prompt"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// =============================================================================
// DATA MODELS (CONTRACTS)
// =============================================================================

// QualitativeInsights centralizes all non-financial agent outputs
type QualitativeInsights struct {
	Strategy          StrategyAnalysis `json:"strategy"`
	CapitalAllocation CapitalAnalysis  `json:"capital_allocation"`
	Segments          SegmentAnalysis  `json:"segments"`
	Risks             RiskAnalysis     `json:"risks"`
}

// RiskAnalysis - Output from Risk Agent
type RiskAnalysis struct {
	TopRisks            []RiskFactor `json:"top_risks"`
	QuantitativeSummary string       `json:"quantitative_summary"` // Item 7A
	LegalProceedings    string       `json:"legal_proceedings"`    // Item 3
	ControlWeaknesses   string       `json:"control_weaknesses"`   // Item 9A
	CybersecurityRisk   string       `json:"cybersecurity_risk"`   // Specific SEC focus
}

type RiskFactor struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Category string `json:"category"` // e.g. Operational, Financial, Regulatory
}

// StrategyAnalysis - Output from Strategy Agent
type StrategyAnalysis struct {
	GrowthTargets          []string `json:"growth_targets"`
	NewBusinessInitiatives []string `json:"new_business_initiatives"`
	ManagementConfidence   int      `json:"management_confidence"` // 0-100
	RiskAssessment         string   `json:"risk_assessment"`
}

// CapitalAnalysis - Output from Capital Allocation Agent
type CapitalAnalysis struct {
	ShareBuybackProgram string `json:"share_buyback_program"`
	DividendPolicy      string `json:"dividend_policy"`
	MAStrategy          string `json:"ma_strategy"`
}

// SegmentAnalysis - Output from Segment Agent
type SegmentAnalysis struct {
	Segments            []StandardizedSegment `json:"segments"`
	GeographicBreakdown []GeoRegion           `json:"geographic_breakdown"`
}

type StandardizedSegment struct {
	Name             string  `json:"name"`
	StandardizedType string  `json:"standardized_type"` // Product, Service, Geo, Hybrid
	RevenueShare     float64 `json:"revenue_share"`     // Percentage 0-100 (Qualitative Estimate)
	MarginProfile    string  `json:"margin_profile"`    // Qualitative description

	// Quantitative Data (Extracted from Note 25)
	Revenues        *FSAPValue `json:"revenues,omitempty"`
	OperatingIncome *FSAPValue `json:"operating_income,omitempty"`
	Assets          *FSAPValue `json:"assets,omitempty"`
	Depreciation    *FSAPValue `json:"depreciation,omitempty"`
	CapEx           *FSAPValue `json:"capex,omitempty"`
}

type GeoRegion struct {
	Region   string     `json:"region"`
	Share    float64    `json:"share"` // Percentage 0-100
	Revenues *FSAPValue `json:"revenues,omitempty"`
}

// =============================================================================
// AGENT IMPLEMENTATIONS
// =============================================================================

// StrategyAgent analyzes MD&A for forward-looking vision
type StrategyAgent struct {
	provider AIProvider
}

func NewStrategyAgent(provider AIProvider) *StrategyAgent {
	return &StrategyAgent{provider: provider}
}

func (a *StrategyAgent) Analyze(ctx context.Context, mdaText string) (*StrategyAnalysis, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Truncate logic (MD&A can be huge, take first 20k and last 20k chars usually covers it,
	// but for simplicity here we just take the first 40k)
	if len(mdaText) > 40000 {
		mdaText = mdaText[:40000] + "... [truncated]"
	}

	// Try to load from prompt library, with fallback
	systemPrompt := getStrategyPrompt()

	userPrompt := fmt.Sprintf(`Analyze the following text and extract:
1. Growth Targets: specific CAGR, revenue, or margin goals.
2. New Business Initiatives: specific new products or markets being entered.
3. Management Confidence Score: 0-100 based on certainty of language (e.g. "we expect" vs "we hope").
4. Risk Assessment: top 3 execution risks mentioned.

Text:
%s

Return JSON:
{
  "growth_targets": ["target 1", "target 2"],
  "new_business_initiatives": ["init 1", "init 2"],
  "management_confidence": 85,
  "risk_assessment": "summary string"
}`, mdaText)

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var result StrategyAnalysis
	if err := parseJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CapitalAllocationAgent analyzes MD&A and Liquidity sections
type CapitalAllocationAgent struct {
	provider AIProvider
}

func NewCapitalAllocationAgent(provider AIProvider) *CapitalAllocationAgent {
	return &CapitalAllocationAgent{provider: provider}
}

func (a *CapitalAllocationAgent) Analyze(ctx context.Context, mdaText string) (*CapitalAnalysis, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	if len(mdaText) > 40000 {
		mdaText = mdaText[:40000] + "... [truncated]"
	}

	// Try to load from prompt library, with fallback
	systemPrompt := getCapitalAllocationPrompt()

	userPrompt := fmt.Sprintf(`Analyze the text for capital allocation priorities:
1. Share Buybacks: authorized amounts, remaining capacity, intent.
2. Dividends: current policy, payout growth intent.
3. M&A Strategy: organic growth vs acquisitions.

Text:
%s

Return JSON:
{
  "share_buyback_program": "string summary",
  "dividend_policy": "string summary",
  "ma_strategy": "string summary"
}`, mdaText)

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var result CapitalAnalysis
	if err := parseJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SegmentAgent normalizes diverse reporting segments
type SegmentAgent struct {
	provider AIProvider
}

func NewSegmentAgent(provider AIProvider) *SegmentAgent {
	return &SegmentAgent{provider: provider}
}

func (a *SegmentAgent) Analyze(ctx context.Context, item1Text string, mdaText string) (*SegmentAnalysis, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Combine Item 1 (Business) and MD&A (Results of Operations)
	// Both define segments
	combinedText := "ITEM 1 BUSINESS:\n" + truncate(item1Text, 20000) + "\n\nMD&A:\n" + truncate(mdaText, 20000)

	// Try to load from prompt library, with fallback
	systemPrompt := getSegmentPrompt()

	userPrompt := fmt.Sprintf(`Identify the company's reporting segments and geographic regions.

For segments:
- Standardized Type: "Product", "Service", "Geo", or "Hybrid"
- Revenue Share: approximate %% of total revenue (estimate from text if not explicit)

Text:
%s

Return JSON:
{
  "segments": [
    {"name": "Hardware", "standardized_type": "Product", "revenue_share": 60.5, "margin_profile": "High margin"},
    {"name": "Services", "standardized_type": "Service", "revenue_share": 39.5, "margin_profile": "Recurring revenue"}
  ],
  "geographic_breakdown": [
    {"region": "North America", "share": 45.0},
    {"region": "International", "share": 55.0}
  ]
}`, combinedText)

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var result SegmentAnalysis
	if err := parseJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Helper util
func parseJSON(resp string, v interface{}) error {
	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	start := strings.Index(cleanJson, "{")
	end := strings.LastIndex(cleanJson, "}")
	if start >= 0 && end > start {
		cleanJson = cleanJson[start : end+1]
	}

	return json.Unmarshal([]byte(cleanJson), v)
}

func truncate(s string, limit int) string {
	if len(s) > limit {
		return s[:limit] + "... [truncated]"
	}
	return s
}

// RiskAgent analyzes Item 1A (Risk Factors)
type RiskAgent struct {
	provider AIProvider
}

func NewRiskAgent(provider AIProvider) *RiskAgent {
	return &RiskAgent{provider: provider}
}

func (a *RiskAgent) Analyze(ctx context.Context, riskText string) (*RiskAnalysis, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Just a simple truncation for now as requested - "Let LLM decide from the text"
	// We rely on the text being the section extracted by the parser via TOC guidance.
	if len(riskText) > 50000 {
		riskText = riskText[:50000] + "... [truncated]"
	}

	// Try to load from prompt library, with fallback
	systemPrompt := getRiskPrompt()

	userPrompt := fmt.Sprintf(`Analyze the following text and extract:
1. Top Risks: The 3-5 most material risk factors (Item 1A). Title, 1-sentence summary, category.
2. Legal Proceedings: Summarize material pending litigation (Item 3). If none, say "None".
3. Control Weaknesses: Summarize any material weaknesses in internal controls (Item 9A). If effective, say "Controls Effective".
4. Quantitative Exposure: Summarize interest rate or FX risk exposure (Item 7A).
5. Cybersecurity: Extract specific mention of cyber threats or breaches.

Text:
%s

Return JSON:
{
  "top_risks": [
    {"title": "Supply Chain", "summary": "...", "category": "Operational"}
  ],
  "legal_proceedings": "Summary of lawsuits...",
  "control_weaknesses": "Summary of weaknesses...",
  "quantitative_summary": "Market risk summary...",
  "cybersecurity_risk": "Cyber risk summary..."
}`, riskText)

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var result RiskAnalysis
	if err := parseJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// PROMPT LIBRARY HELPERS
// =============================================================================

// getStrategyPrompt returns the strategy agent system prompt
func getStrategyPrompt() string {
	if p, err := prompt.Get().GetSystemPrompt(prompt.PromptIDs.QualitativeStrategy); err == nil && p != "" {
		return p
	}
	// Fallback
	return `You are a Strategic Financial Analyst. Analyze the Management's Discussion and Analysis (MD&A) section.
Validation Step: First, verify this is MD&A content (narrative discussion). If it is a financial table or unrelated legal text, return JSON with all fields null/empty.
Identify explicit forward-looking targets, new initiatives, and assess management's tone.`
}

// getCapitalAllocationPrompt returns the capital allocation agent system prompt
func getCapitalAllocationPrompt() string {
	if p, err := prompt.Get().GetSystemPrompt(prompt.PromptIDs.QualitativeCapitalAllocation); err == nil && p != "" {
		return p
	}
	// Fallback
	return `You are a Capital Allocation Specialist. Analyze how the company deploys its free cash flow.
Validation Step: Verify the text discusses capital/liquidity (Dividends, Buybacks, M&A). If unrelated, return empty JSON.`
}

// getSegmentPrompt returns the segment agent system prompt
func getSegmentPrompt() string {
	if p, err := prompt.Get().GetSystemPrompt(prompt.PromptIDs.QualitativeSegment); err == nil && p != "" {
		return p
	}
	// Fallback
	return `You are a Data Normalization Expert.
Validation Step: Verify provided text contains Business Segment descriptions or Revenue Tables. If not, return empty segment list.
Your goal is to map the company's specific reporting segments into a standardized format and extract revenue breakdown.`
}

// getRiskPrompt returns the risk agent system prompt
func getRiskPrompt() string {
	if p, err := prompt.Get().GetSystemPrompt(prompt.PromptIDs.QualitativeRisk); err == nil && p != "" {
		return p
	}
	// Fallback
	return `You are a Risk Management Expert. Analyze the Risk Factors (Item 1A) section of a 10-K.
Validation Step: Verify the text contains actual Risk Factors or Legal/Control discussions. If it is just a Table of Contents or unrelated headers, return empty/null fields.
Summarize the most critical structural risks facing the entity. Focus on material threats, not generic boilerplate.`
}
