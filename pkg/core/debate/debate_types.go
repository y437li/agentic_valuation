package debate

import (
	"agentic_valuation/pkg/core/calc"
	"agentic_valuation/pkg/core/edgar"
	"time"
)

// AgentRole defines the specific persona of an agent
type AgentRole string

const (
	RoleMacro       AgentRole = "macro"
	RoleSentiment   AgentRole = "sentiment"
	RoleFundamental AgentRole = "fundamentals"
	RoleSkeptic     AgentRole = "skeptic"
	RoleOptimist    AgentRole = "optimist"
	RoleSynthesizer AgentRole = "synthesizer" // Final report generator
	RoleModerator   AgentRole = "moderator"   // System notifications
	RoleHuman       AgentRole = "human"       // Human participant
)

// DebateMode defines the execution mode
type DebateMode string

const (
	ModeAutomatic   DebateMode = "automatic"   // Fully automated
	ModeInteractive DebateMode = "interactive" // Human can participate
)

// DebateMessage represents a single message in the debate stream
type DebateMessage struct {
	ID         string    `json:"id"`
	Round      int       `json:"round"`
	AgentRole  AgentRole `json:"agent_role"`
	AgentName  string    `json:"agent_name"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	References []string  `json:"references,omitempty"` // Web sources or data citations
}

// SharedContext holds all conclusions and facts shared between agents
type SharedContext struct {
	Company                string                     `json:"company"`
	FiscalYear             string                     `json:"fiscal_year"`
	MacroFindings          *MacroResearch             `json:"macro_findings,omitempty"`
	SentimentData          *SentimentResearch         `json:"sentiment_findings,omitempty"`
	FundamentalData        *FundamentalResearch       `json:"fundamental_findings,omitempty"`
	DebateHistory          []DebateMessage            `json:"debate_history"`
	HumanQuestions         []HumanQuestion            `json:"human_questions,omitempty"`          // Human Q&A records
	ReportGenerationPrompt string                     `json:"report_generation_prompt,omitempty"` // Override prompt for synthesis/extraction
	CurrentConsensus       map[string]AssumptionDraft `json:"current_consensus"`

	// Single Source of Truth
	MaterialPool *MaterialPool `json:"material_pool,omitempty"`

	// Baseline Assumptions (Quantitative Pre-Debate)
	BaselineAssumptions map[string]float64 `json:"baseline_assumptions,omitempty"`
}

// MaterialPool aggregates all quantitative and qualitative intelligence
// effectively replacing the fragmented fields above
type MaterialPool struct {
	// 1. Quantitative Foundation (from Extraction + Calc Engine)
	FinancialHistory  []*edgar.FSAPDataResponse        `json:"financial_history"`   // Raw atomic data
	CommonSizeHistory map[int]*calc.CommonSizeAnalysis `json:"common_size_history"` // Vertical/Growth Analysis by Year
	ThreeLevelHistory map[int]*calc.ThreeLevelAnalysis `json:"three_level_history"` // 3-Level Depth (Growth, Returns, Risk)
	ImpliedMetrics    map[int]calc.ImpliedMetrics      `json:"implied_metrics"`     // Back-solved technical assumptions

	// 2. Qualitative Intelligence (from Strategy/Risk/Segment Agents)
	BusinessStrategy     *edgar.StrategyAnalysis `json:"business_strategy"`
	RiskProfile          *edgar.RiskAnalysis     `json:"risk_profile"`
	CapitalAllocation    *edgar.CapitalAnalysis  `json:"capital_allocation"`
	QualitativeSegments  *edgar.SegmentAnalysis  `json:"qualitative_segments"`
	QuantitativeSegments *edgar.SegmentAnalysis  `json:"quantitative_segments,omitempty"`

	// 3. Market Context (from Research Phase)
	MacroTrends       *MacroResearch     `json:"macro_trends"`
	MarketSentiment   *SentimentResearch `json:"market_sentiment"`
	TranscriptHistory []Transcript       `json:"transcript_history,omitempty"` // Recently added: Earnings Calls
}

type Transcript struct {
	Ticker        string `json:"ticker"`
	CompanyName   string `json:"company_name"`
	Date          string `json:"date"`
	Content       string `json:"content"` // Full text or summarized
	FiscalQuarter string `json:"fiscal_quarter"`
	Source        string `json:"source"`
}

// HumanQuestion represents a question asked by human to a specific agent
type HumanQuestion struct {
	ID          string    `json:"id"`
	TargetAgent AgentRole `json:"target_agent"`
	Question    string    `json:"question"`
	Response    string    `json:"response"`
	AskedAt     time.Time `json:"asked_at"`
	RespondedAt time.Time `json:"responded_at,omitempty"`
}

// AssumptionDraft is a work-in-progress atomized assumption value
// Each draft links to a parent assumption ID for hierarchical tracking
type AssumptionDraft struct {
	ParameterName      string      `json:"parameter_name"`    // e.g., "Revenue Growth FY2025"
	ParentAssumptionID string      `json:"parent_assumption"` // e.g., "rev-growth" (links to frontend store)
	Value              float64     `json:"value"`
	Unit               string      `json:"unit"` // e.g., "%", "USD", "x"
	Rationale          string      `json:"rationale"`
	Confidence         float64     `json:"confidence"`  // 0-1
	ProposedByAgent    AgentRole   `json:"proposed_by"` // Which agent proposed this value
	SupportedBy        []AgentRole `json:"supported_by"`
	ChallengedBy       []AgentRole `json:"challenged_by"`
	SourceURLs         []string    `json:"source_urls"` // Trackable URLs backing this assumption
}

// Research Structures for Phase 1

type MacroResearch struct {
	GDPForecast     string   `json:"gdp_forecast"`
	InterestRates   string   `json:"interest_rates"`
	Inflation       string   `json:"inflation"`
	CommodityTrends []string `json:"commodity_trends"`
	PolicyRisks     []string `json:"policy_risks"`
	Summary         string   `json:"summary"`
}

type SentimentResearch struct {
	OverallSentiment string   `json:"overall_sentiment"` // Bullish, Bearish, Neutral
	NewsHighlights   []string `json:"news_highlights"`
	AnalystConsensus string   `json:"analyst_consensus"`
	Summary          string   `json:"summary"`
}

type FundamentalResearch struct {
	RevenueCAGR     float64  `json:"revenue_cagr"`
	GrossMarginAvg  float64  `json:"gross_margin_avg"`
	OperatingMargin float64  `json:"operating_margin"`
	KeyDrivers      []string `json:"key_drivers"`
	Summary         string   `json:"summary"`
}

// DebateStatus enumerates the lifecycle states of a debate
type DebateStatus string

const (
	StatusIdle      DebateStatus = "idle"
	StatusRunning   DebateStatus = "running"
	StatusCompleted DebateStatus = "completed"
	StatusFailed    DebateStatus = "failed"
)

// FinalDebateReport aggregates the final consensus and key insights
type FinalDebateReport struct {
	DebateID         string                      `json:"debate_id"`
	Company          string                      `json:"company"`
	FiscalYear       string                      `json:"fiscal_year"`
	CompletionTime   time.Time                   `json:"completion_time"`
	Assumptions      map[string]AssumptionResult `json:"assumptions"`
	ExecutiveSummary string                      `json:"executive_summary"`
	KeyRisks         []string                    `json:"key_risks"`
	KeyOpportunities []string                    `json:"key_opportunities"`
}

// AssumptionResult is the finalized atomized value for a parameter
// Ready to be pushed to the frontend assumption store
type AssumptionResult struct {
	ParentAssumptionID string   `json:"parent_assumption"` // Links to frontend store ID (e.g., "rev-growth")
	Value              float64  `json:"value"`
	Unit               string   `json:"unit"`
	Confidence         float64  `json:"confidence"` // 0-1
	Rationale          string   `json:"rationale"`
	FinalizedByAgent   string   `json:"finalized_by"` // "CIO" or specific agent
	Sources            []string `json:"sources"`      // Agent names
	SourceURLs         []string `json:"source_urls"`  // Trackable URLs
}
