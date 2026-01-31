package assistant

import (
	"agentic_valuation/pkg/core/agent"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handler provides HTTP handlers for AI Assistant functionality
type Handler struct {
	agentMgr *agent.Manager
}

// NewHandler creates a new assistant handler
func NewHandler(mgr *agent.Manager) *Handler {
	return &Handler{agentMgr: mgr}
}

// NavigationRequest represents the user's natural language query
type NavigationRequest struct {
	Message        string `json:"message"`
	CurrentSection string `json:"current_section,omitempty"`
}

// NavigationResponse contains the LLM's parsed intent
type NavigationResponse struct {
	Intent          string           `json:"intent"` // "navigate", "query", "action", "chat"
	TargetSection   string           `json:"target_section,omitempty"`
	SectionLabel    string           `json:"section_label,omitempty"`
	Confidence      float64          `json:"confidence"`
	Explanation     string           `json:"explanation"`
	DataPointsFound []string         `json:"data_points_found,omitempty"`
	Suggestions     []NavigationHint `json:"suggestions,omitempty"`
}

// NavigationHint provides alternative navigation suggestions
type NavigationHint struct {
	SectionID string `json:"section_id"`
	Label     string `json:"label"`
	Relevance string `json:"relevance"`
}

// NavigationRegistry defines all navigable sections for LLM context
const NavigationRegistry = `
Available sections in the TIED Agentic Valuation Platform:

DATA:
- data-mapping: Data Mapping - View and validate mapped financial data from SEC filings

ANALYSIS:
- financial-statements: Financial Statements - View Income Statement, Balance Sheet, Cash Flow Statement. Data: revenue, net income, total assets, liabilities, operating cash flow
- common-size: Ratio Analysis - Common size analysis and key financial ratios. Data: gross margin, operating margin, net margin, current ratio, debt to equity
- dupont: DuPont Analysis - Decompose ROE into profit margin, asset turnover, leverage. Data: ROE, profit margin, asset turnover, equity multiplier
- growth-rates: Growth Rates - Year-over-year and CAGR growth analysis. Data: revenue growth, earnings growth, CAGR

VALUATION:
- valuation-comparison: Valuation Overview - Compare DCF, DDM, RIM models with triangulation. Data: DCF value, DDM value, RIM value, target price, upside/downside
- assumptions: Assumption Nodes - Manage valuation assumptions with agent debates. Data: revenue assumptions, WACC, terminal growth rate
- forecast: Forecast - Revenue forecasting with product-level bottom-up modeling. Data: revenue projections, segment breakdown
- model-details: Model Details - Detailed DCF with WACC calculation and terminal value. Data: WACC components, terminal value, FCF projections
- sensitivity: Sensitivity Analysis - Monte Carlo simulation and sensitivity tables. Data: sensitivity matrix, probability distribution
- roundtable: Roundtable - View multi-agent debate transcripts and consensus. Data: debate transcript, agent positions, final recommendation

OTHER:
- audit: Audit Trail - Complete audit trail of all changes. Data: change history, data lineage
- settings: Settings - Configure LLM providers and system settings
`

// HandleNavigationIntent parses user message and returns navigation intent
func (h *Handler) HandleNavigationIntent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NavigationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build prompt for LLM
	systemPrompt := fmt.Sprintf(`You are a navigation assistant for the TIED Agentic Valuation Platform.
Your job is to understand the user's intent and determine if they want to navigate to a specific section.

%s

User's current section: %s

Analyze the user's message and respond with a JSON object:
{
  "intent": "navigate" | "query" | "chat",
  "target_section": "<section_id if intent is navigate>",
  "section_label": "<human readable label>",
  "confidence": <0.0-1.0>,
  "explanation": "<brief explanation in the same language as user's message>",
  "data_points_found": ["<relevant data points the user might be looking for>"],
  "suggestions": [{"section_id": "...", "label": "...", "relevance": "..."}]
}

Rules:
1. If user clearly wants to go somewhere, set intent="navigate" and provide target_section
2. If user is asking about data that exists in a specific section, set intent="query" and suggest the section
3. If user is just chatting or asking general questions, set intent="chat"
4. Always respond in the same language as the user's message
5. Return ONLY valid JSON, no markdown or extra text`, NavigationRegistry, req.CurrentSection)

	userPrompt := req.Message

	// Call LLM
	resp, err := h.agentMgr.ExecutePrompt("assistant", userPrompt, systemPrompt, nil)
	if err != nil {
		// Fallback to simple keyword matching
		fallbackResp := h.fallbackKeywordMatch(req.Message)
		json.NewEncoder(w).Encode(fallbackResp)
		return
	}

	// Parse LLM response
	var navResp NavigationResponse
	cleanResp := strings.TrimPrefix(resp, "```json")
	cleanResp = strings.TrimSuffix(cleanResp, "```")
	cleanResp = strings.TrimSpace(cleanResp)

	if err := json.Unmarshal([]byte(cleanResp), &navResp); err != nil {
		// Return raw response if parsing fails
		navResp = NavigationResponse{
			Intent:      "chat",
			Explanation: resp,
			Confidence:  0.5,
		}
	}

	json.NewEncoder(w).Encode(navResp)
}

// fallbackKeywordMatch provides basic keyword-based navigation when LLM is unavailable
func (h *Handler) fallbackKeywordMatch(message string) NavigationResponse {
	msg := strings.ToLower(message)

	keywords := map[string]struct{ id, label string }{
		"financial":   {"financial-statements", "Financial Statements"},
		"财务报表":        {"financial-statements", "Financial Statements"},
		"valuation":   {"valuation-comparison", "Valuation Overview"},
		"估值":          {"valuation-comparison", "Valuation Overview"},
		"dcf":         {"valuation-comparison", "Valuation Overview"},
		"sensitivity": {"sensitivity", "Sensitivity Analysis"},
		"敏感性":         {"sensitivity", "Sensitivity Analysis"},
		"monte carlo": {"sensitivity", "Sensitivity Analysis"},
		"roundtable":  {"roundtable", "Roundtable"},
		"辩论":          {"roundtable", "Roundtable"},
		"debate":      {"roundtable", "Roundtable"},
		"dupont":      {"dupont", "DuPont Analysis"},
		"杜邦":          {"dupont", "DuPont Analysis"},
		"growth":      {"growth-rates", "Growth Rates"},
		"增长":          {"growth-rates", "Growth Rates"},
		"ratio":       {"common-size", "Ratio Analysis"},
		"比率":          {"common-size", "Ratio Analysis"},
		"forecast":    {"forecast", "Forecast"},
		"预测":          {"forecast", "Forecast"},
		"assumption":  {"assumptions", "Assumption Nodes"},
		"假设":          {"assumptions", "Assumption Nodes"},
		"audit":       {"audit", "Audit Trail"},
		"审计":          {"audit", "Audit Trail"},
		"settings":    {"settings", "Settings"},
		"设置":          {"settings", "Settings"},
	}

	for kw, section := range keywords {
		if strings.Contains(msg, kw) {
			return NavigationResponse{
				Intent:        "navigate",
				TargetSection: section.id,
				SectionLabel:  section.label,
				Confidence:    0.8,
				Explanation:   fmt.Sprintf("检测到关键词 '%s'，建议跳转到 %s", kw, section.label),
			}
		}
	}

	return NavigationResponse{
		Intent:      "chat",
		Confidence:  1.0,
		Explanation: "No navigation intent detected",
	}
}
