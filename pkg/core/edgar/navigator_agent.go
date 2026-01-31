// Package edgar - Navigator Agent for TOC parsing
package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agentic_valuation/pkg/core/prompt"
)

// =============================================================================
// NAVIGATOR AGENT - Parses TOC to identify section locations
// =============================================================================

// SectionLocation represents location info for a document section
type SectionLocation struct {
	Title    string `json:"title"`
	PageRaw  any    `json:"page,omitempty"` // Can be int, string, or null
	Anchor   string `json:"anchor,omitempty"`
	LineHint int    `json:"line_hint,omitempty"` // Approximate line number for iXBRL
}

// Page returns the page number as int (0 if not available or invalid)
func (s *SectionLocation) Page() int {
	if s == nil || s.PageRaw == nil {
		return 0
	}
	switch v := s.PageRaw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		// Try to parse string as int
		var page int
		fmt.Sscanf(v, "%d", &page)
		return page
	default:
		return 0
	}
}

// SectionMap maps standard section names to their locations
type SectionMap struct {
	Business         *SectionLocation `json:"business,omitempty"`
	RiskFactors      *SectionLocation `json:"risk_factors,omitempty"`
	MarketRisk       *SectionLocation `json:"market_risk,omitempty"`
	LegalProceedings *SectionLocation `json:"legal_proceedings,omitempty"`
	Controls         *SectionLocation `json:"controls,omitempty"`
	MDA              *SectionLocation `json:"mda,omitempty"`
	BalanceSheet     *SectionLocation `json:"balance_sheet,omitempty"`
	IncomeStatement  *SectionLocation `json:"income_statement,omitempty"`
	CashFlow         *SectionLocation `json:"cash_flow,omitempty"`
	Notes            *SectionLocation `json:"notes,omitempty"`
}

// NavigatorAgent uses LLM to parse dynamic TOC and identify sections
type NavigatorAgent struct {
	provider AIProvider
}

// NewNavigatorAgent creates a new navigator agent
func NewNavigatorAgent(provider AIProvider) *NavigatorAgent {
	return &NavigatorAgent{provider: provider}
}

// ParseTOC analyzes Table of Contents and returns section locations
func (a *NavigatorAgent) ParseTOC(ctx context.Context, tocContent string) (*SectionMap, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	systemPrompt, userPrompt := a.buildTOCPrompt(tocContent)

	response, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM query failed: %w", err)
	}

	return a.parseResponse(response)
}

// buildTOCPrompt creates the prompts for TOC analysis
// Tries to load from prompt library first, falls back to hardcoded if not found
func (a *NavigatorAgent) buildTOCPrompt(tocContent string) (string, string) {
	// Try to load prompt from JSON file
	if pt, err := prompt.Get().GetPrompt("extraction.navigator.toc"); err == nil {
		// Use prompt from library
		ctx := prompt.NewContext().
			Set("TOCContent", tocContent)
		userPrompt, _ := prompt.RenderUserPrompt(pt, ctx)
		return pt.SystemPrompt, userPrompt
	}

	// Fallback to hardcoded prompt
	systemPrompt := "You are a Senior Financial Analyst. Analyze 10-K Table of Contents to identify key sections."

	userPrompt := fmt.Sprintf(`Analyze this Table of Contents and identify the locations of key sections:

TABLE OF CONTENTS:
%s

For each section, provide title, page number, and anchor if available.

Return JSON only:
{
  "business": {"title": "...", "page": ..., "anchor": "..."},
  "risk_factors": {"title": "...", "page": ..., "anchor": "..."},
  "market_risk": {"title": "...", "page": ..., "anchor": "..."},
  "legal_proceedings": {"title": "...", "page": ..., "anchor": "..."},
  "controls": {"title": "...", "page": ..., "anchor": "..."},
  "mda": {"title": "...", "page": ..., "anchor": "..."},
  "balance_sheet": {"title": "...", "page": ..., "anchor": "..."},
  "income_statement": {"title": "...", "page": ..., "anchor": "..."},
  "cash_flow": {"title": "...", "page": ..., "anchor": "..."},
  "notes": {"title": "...", "page": ..., "anchor": "..."}
}

Notes:
- Use null for sections not found
- anchor should be the HTML anchor ID if available`, tocContent)

	return systemPrompt, userPrompt
}

// parseResponse extracts SectionMap from LLM response
func (a *NavigatorAgent) parseResponse(response string) (*SectionMap, error) {
	// Extract JSON from response (may have markdown wrapper)
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}
	jsonStr := response[jsonStart : jsonEnd+1]

	var result SectionMap
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &result, nil
}
