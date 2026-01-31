package debate

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"agentic_valuation/pkg/core/agent"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// DebateAgent defines the interface for all participating agents
type DebateAgent interface {
	Role() AgentRole
	// Name returns the display name of the agent
	Name() string
	// Generate produces a contribution to the debate based on the current context
	Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error)
}

// BaseAgent provides common functionality for all agents using direct Gemini integration
type BaseAgent struct {
	role         AgentRole
	modelName    string
	client       *genai.Client
	systemPrompt string
}

// UniversalAgent provides functionality for agents using the global agent.Manager (Configurable Provider)
type UniversalAgent struct {
	role         AgentRole
	agentManager *agent.Manager
	systemPrompt string
}

func NewBaseAgent(ctx context.Context, role AgentRole, sysPrompt string) (*BaseAgent, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %v", err)
	}

	return &BaseAgent{
		role:         role,
		modelName:    "gemini-3-flash-preview", // Latest Gemini model with grounding
		client:       client,
		systemPrompt: sysPrompt,
	}, nil
}

func NewUniversalAgent(role AgentRole, mgr *agent.Manager, sysPrompt string) *UniversalAgent {
	return &UniversalAgent{
		role:         role,
		agentManager: mgr,
		systemPrompt: sysPrompt,
	}
}

func (a *BaseAgent) Role() AgentRole {
	return a.role
}

func (a *UniversalAgent) Role() AgentRole {
	return a.role
}

func (a *BaseAgent) Name() string {
	return GetAgentName(a.role)
}

func (a *UniversalAgent) Name() string {
	return GetAgentName(a.role)
}

func GetAgentName(role AgentRole) string {
	switch role {
	case RoleMacro:
		return "Macro Analyst"
	case RoleSentiment:
		return "Market Sentiment"
	case RoleFundamental:
		return "Fundamental Analyst"
	case RoleSkeptic:
		return "The Skeptic"
	case RoleOptimist:
		return "The Optimist"
	case RoleSynthesizer:
		return "Chief Investment Officer"
	default:
		return "Unknown Agent"
	}
}

// generateWithGrounding calls Gemini with Google Search Grounding enabled
func (a *BaseAgent) generateWithGrounding(ctx context.Context, prompt string) (string, []string, error) {
	model := a.client.GenerativeModel(a.modelName)

	// Enable Google Search Grounding
	model.Tools = []*genai.Tool{
		// {GoogleSearchRetrieval: &genai.GoogleSearchRetrieval{}},
	}
	model.SetTemperature(0.7)

	// Construct full prompt
	fullPrompt := fmt.Sprintf("%s\n\nTask: %s", a.systemPrompt, prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(fullPrompt))
	if err != nil {
		return "", nil, err
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "I have nothing to add.", nil, nil
	}

	// Extract text content
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			sb.WriteString(string(txt))
		}
	}
	content := sb.String()

	// Extract citations/grounding metadata
	// TODO: Upgrade github.com/google/generative-ai-go to support GoogleSearchRetrieval
	var references []string
	/*
		if resp.Candidates[0].GroundingMetadata != nil {
			for _, chunk := range resp.Candidates[0].GroundingMetadata.GroundingChunks {
				if chunk.Web != nil {
					references = append(references, fmt.Sprintf("[%s](%s)", chunk.Web.Title, chunk.Web.Uri))
				}
			}
		}
	*/

	return content, references, nil
}

// Generate implementation for UniversalAgent using agent.Manager
func (a *UniversalAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	return DebateMessage{}, fmt.Errorf("UniversalAgent.Generate should be overridden")
}

// MacroAgent Implementation (Keeps using BaseAgent/Gemini)
type MacroAgent struct {
	*BaseAgent
}

func NewMacroAgent(ctx context.Context) (*MacroAgent, error) {
	base, err := NewBaseAgent(ctx, RoleMacro, SystemPrompts[RoleMacro])
	if err != nil {
		return nil, err
	}
	return &MacroAgent{BaseAgent: base}, nil
}

func (a *MacroAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	prompt := fmt.Sprintf("Analyze the current macroeconomic environment relevant to %s for fiscal year %s. "+
		"Check interest rates, GDP growth, inflation, and relevant commodity prices.", shared.Company, shared.FiscalYear)

	content, refs, err := a.generateWithGrounding(ctx, prompt)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: refs,
		Timestamp:  time.Now(),
	}, nil
}

// SentimentAgent Implementation (Keeps using BaseAgent/Gemini)
type SentimentAgent struct {
	*BaseAgent
}

func NewSentimentAgent(ctx context.Context) (*SentimentAgent, error) {
	base, err := NewBaseAgent(ctx, RoleSentiment, SystemPrompts[RoleSentiment])
	if err != nil {
		return nil, err
	}
	return &SentimentAgent{BaseAgent: base}, nil
}

func (a *SentimentAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	prompt := fmt.Sprintf("What is the current market sentiment for %s? "+
		"Search for recent analyst notes, news headlines, and general market mood.", shared.Company)

	content, refs, err := a.generateWithGrounding(ctx, prompt)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: refs,
		Timestamp:  time.Now(),
	}, nil
}

// FundamentalAgent (Placeholder, will use Gemini for now)
type FundamentalAgent struct {
	*BaseAgent
}

func NewFundamentalAgent(ctx context.Context) (*FundamentalAgent, error) {
	base, err := NewBaseAgent(ctx, RoleFundamental, SystemPrompts[RoleFundamental])
	if err != nil {
		return nil, err
	}
	return &FundamentalAgent{BaseAgent: base}, nil
}

func (a *FundamentalAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	prompt := fmt.Sprintf("Analyze the fundamental business model and key financial drivers for %s. "+
		"Focus on revenue sources, margins, and competitive advantages.", shared.Company)

	content, refs, err := a.generateWithGrounding(ctx, prompt)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: refs,
		Timestamp:  time.Now(),
	}, nil
}

// SkepticAgent Implementation (Switches to UniversalAgent)
type SkepticAgent struct {
	*UniversalAgent
}

func NewSkepticAgent(mgr *agent.Manager) *SkepticAgent {
	base := NewUniversalAgent(RoleSkeptic, mgr, SystemPrompts[RoleSkeptic])
	return &SkepticAgent{UniversalAgent: base}
}

func (a *SkepticAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	var contextSummary strings.Builder
	for _, msg := range shared.DebateHistory {
		contextSummary.WriteString(fmt.Sprintf("%s: %s\n", msg.AgentName, msg.Content))
	}

	prompt := fmt.Sprintf("Review the debate so far for %s:\n\n%s\n\n"+
		"Identify weaknesses, over-optimism, or missing risks. Challenge specific points made by others.",
		shared.Company, contextSummary.String())

	// Use Agent Manager to execute prompt with configured provider
	content, err := a.agentManager.ExecutePrompt("skeptic", prompt, a.systemPrompt, nil)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: nil,
		Timestamp:  time.Now(),
	}, nil
}

// OptimistAgent Implementation (Switches to UniversalAgent)
type OptimistAgent struct {
	*UniversalAgent
}

func NewOptimistAgent(mgr *agent.Manager) *OptimistAgent {
	base := NewUniversalAgent(RoleOptimist, mgr, SystemPrompts[RoleOptimist])
	return &OptimistAgent{UniversalAgent: base}
}

func (a *OptimistAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	var contextSummary strings.Builder
	for _, msg := range shared.DebateHistory {
		contextSummary.WriteString(fmt.Sprintf("%s: %s\n", msg.AgentName, msg.Content))
	}

	prompt := fmt.Sprintf("Review the debate so far for %s:\n\n%s\n\n"+
		"Highlight growth opportunities, defend against skeptic criticisms, and focus on upside potential.",
		shared.Company, contextSummary.String())

	content, err := a.agentManager.ExecutePrompt("optimist", prompt, a.systemPrompt, nil)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: nil,
		Timestamp:  time.Now(),
	}, nil
}

// MacroUniversalAgent - Uses global provider instead of direct Gemini
type MacroUniversalAgent struct {
	*UniversalAgent
}

func NewMacroUniversalAgent(mgr *agent.Manager) *MacroUniversalAgent {
	base := NewUniversalAgent(RoleMacro, mgr, SystemPrompts[RoleMacro])
	return &MacroUniversalAgent{UniversalAgent: base}
}

func (a *MacroUniversalAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	prompt := fmt.Sprintf("Analyze the macroeconomic environment affecting %s for fiscal year %s. "+
		"Cover GDP outlook, interest rates, inflation trends, commodity prices, and policy risks. "+
		"Provide specific data and forecasts relevant to this company's industry.",
		shared.Company, shared.FiscalYear)

	options := map[string]interface{}{
		"google_search": true,
	}
	content, err := a.agentManager.ExecutePrompt("macro", prompt, a.systemPrompt, options)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: nil,
		Timestamp:  time.Now(),
	}, nil
}

// SentimentUniversalAgent - Uses global provider instead of direct Gemini
type SentimentUniversalAgent struct {
	*UniversalAgent
}

func NewSentimentUniversalAgent(mgr *agent.Manager) *SentimentUniversalAgent {
	base := NewUniversalAgent(RoleSentiment, mgr, SystemPrompts[RoleSentiment])
	return &SentimentUniversalAgent{UniversalAgent: base}
}

func (a *SentimentUniversalAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	prompt := fmt.Sprintf("Analyze market sentiment for %s. "+
		"Search for recent analyst notes, news headlines, social media sentiment, and general market mood. "+
		"Summarize the overall sentiment (bullish/bearish/neutral) with supporting evidence. ",
		shared.Company)

	// Inject Transcript if available
	if shared.MaterialPool != nil && len(shared.MaterialPool.TranscriptHistory) > 0 {
		latest := shared.MaterialPool.TranscriptHistory[0]
		// Truncate to avoid context limit issues, though Flash handles large context well.
		// 50k chars covers most transcripts fully.
		content := latest.Content
		if len(content) > 50000 {
			content = content[:50000] + "...(truncated)"
		}

		prompt += fmt.Sprintf("\n\n=== EARNINGS CALL TRANSCRIPT REFERENCE (%s) ===\n"+
			"Use the following transcript (especially the Q&A section) to identify analyst concerns and management tone:\n\n%s\n"+
			"=== END TRANSCRIPT ===\n", latest.Date, content)
	}

	options := map[string]interface{}{
		"google_search": true,
	}
	content, err := a.agentManager.ExecutePrompt("sentiment", prompt, a.systemPrompt, options)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: nil,
		Timestamp:  time.Now(),
	}, nil
}

// FundamentalUniversalAgent - Uses global provider instead of direct Gemini
type FundamentalUniversalAgent struct {
	*UniversalAgent
}

func NewFundamentalUniversalAgent(mgr *agent.Manager) *FundamentalUniversalAgent {
	base := NewUniversalAgent(RoleFundamental, mgr, SystemPrompts[RoleFundamental])
	return &FundamentalUniversalAgent{UniversalAgent: base}
}

func (a *FundamentalUniversalAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	prompt := fmt.Sprintf("Analyze the fundamental business model and key financial drivers for %s. "+
		"Focus on revenue sources, margins, competitive advantages, and growth catalysts. "+
		"Provide specific metrics and insights for valuation purposes. ",
		shared.Company)

	// Inject Transcript if available
	if shared.MaterialPool != nil && len(shared.MaterialPool.TranscriptHistory) > 0 {
		latest := shared.MaterialPool.TranscriptHistory[0]
		// Truncate to avoid context limit issues
		content := latest.Content
		if len(content) > 50000 {
			content = content[:50000] + "...(truncated)"
		}

		prompt += fmt.Sprintf("\n\n=== EARNINGS CALL TRANSCRIPT REFERENCE (%s) ===\n"+
			"Use the following transcript (especially the Management Prepared Remarks) to extract strategic guidance and future outlook:\n\n%s\n"+
			"=== END TRANSCRIPT ===\n", latest.Date, content)
	}

	options := map[string]interface{}{
		"google_search": true,
	}
	content, err := a.agentManager.ExecutePrompt("fundamentals", prompt, a.systemPrompt, options)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: nil,
		Timestamp:  time.Now(),
	}, nil
}

// SynthesizerAgent Implementation
type SynthesizerAgent struct {
	*UniversalAgent
}

func NewSynthesizerAgent(mgr *agent.Manager) *SynthesizerAgent {
	base := NewUniversalAgent(RoleSynthesizer, mgr, SystemPrompts[RoleSynthesizer])
	return &SynthesizerAgent{UniversalAgent: base}
}

func (a *SynthesizerAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	// 1. Check for Override Prompt (e.g., JSON Extraction)
	if shared.ReportGenerationPrompt != "" {
		content, err := a.agentManager.ExecutePrompt("synthesizer", shared.ReportGenerationPrompt, a.systemPrompt, nil)
		if err != nil {
			return DebateMessage{}, err
		}
		return DebateMessage{
			AgentRole:  a.Role(),
			AgentName:  a.Name(),
			Content:    content,
			References: nil,
			Timestamp:  time.Now(),
		}, nil
	}

	// 2. Default Behavior: Generate Final Report
	// Synthesizer needs the FULL context in JSON format to do its job
	// We'll approximate this by feeding the transcript

	var contextSummary strings.Builder
	contextSummary.WriteString("## Debate Transcript\n")
	for _, msg := range shared.DebateHistory {
		contextSummary.WriteString(fmt.Sprintf("%s (%s): %s\n\n", msg.AgentName, msg.AgentRole, msg.Content))
	}

	// Also include consensus if available (currently it's just a map)
	if len(shared.CurrentConsensus) > 0 {
		contextSummary.WriteString("\n## Current Consensus Board\n")
		for k, v := range shared.CurrentConsensus {
			contextSummary.WriteString(fmt.Sprintf("- %s: %.2f (Confidence: %.2f)\n", k, v.Value, v.Confidence))
		}
	}

	prompt := fmt.Sprintf("Generate the Final Debate Report for %s FY%s based on the following context:\n\n%s",
		shared.Company, shared.FiscalYear, contextSummary.String())

	content, err := a.agentManager.ExecutePrompt("synthesizer", prompt, a.systemPrompt, nil)
	if err != nil {
		return DebateMessage{}, err
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: nil,
		Timestamp:  time.Now(),
	}, nil
}
