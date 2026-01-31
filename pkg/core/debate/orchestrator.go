package debate

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"agentic_valuation/pkg/core/agent"
	"agentic_valuation/pkg/core/utils"
)

// DebateOrchestrator manages the state and execution of a single debate
type DebateOrchestrator struct {
	ID            string
	Ticker        string
	Company       string
	FiscalYear    string
	IsSimulation  bool
	Mode          DebateMode // "automatic" or "interactive"
	Status        DebateStatus
	Phase         int // Current phase (1=Research, 2=Debate, 3=Synthesis)
	History       []DebateMessage
	SharedContext *SharedContext
	FinalReport   *FinalDebateReport
	UpdatedAt     time.Time

	AgentManager *agent.Manager
	Repo         *DebateRepo

	// Interactive mode support
	questionChan chan HumanQuestion // Channel for human questions
	resumeChan   chan bool          // Channel to signal debate resume

	// Streaming support
	subscribers []chan DebateMessage
	mu          sync.RWMutex
}

// NewOrchestrator creates a new debate instance
func NewOrchestrator(id, ticker, company, fiscalYear string, isSimulation bool, mode DebateMode, mgr *agent.Manager, repo *DebateRepo) *DebateOrchestrator {
	if mode == "" {
		mode = ModeAutomatic
	}
	return &DebateOrchestrator{
		ID:           id,
		Ticker:       ticker,
		Company:      company,
		FiscalYear:   fiscalYear,
		IsSimulation: isSimulation,
		Mode:         mode,
		Status:       StatusIdle,
		Phase:        0,
		AgentManager: mgr,
		Repo:         repo,
		questionChan: make(chan HumanQuestion, 10),
		resumeChan:   make(chan bool, 1),
		SharedContext: &SharedContext{
			Company:          company,
			FiscalYear:       fiscalYear,
			DebateHistory:    []DebateMessage{},
			HumanQuestions:   []HumanQuestion{},
			CurrentConsensus: make(map[string]AssumptionDraft),
		},
		UpdatedAt: time.Now(),
	}
}

// Subscribe adds a client channel for real-time updates
func (o *DebateOrchestrator) Subscribe() (chan DebateMessage, []DebateMessage) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create buffered channel to avoid blocking
	ch := make(chan DebateMessage, 100)
	o.subscribers = append(o.subscribers, ch)

	// Return current history for replay
	historyCopy := make([]DebateMessage, len(o.History))
	copy(historyCopy, o.History)

	return ch, historyCopy
}

// Unsubscribe removes a client channel
func (o *DebateOrchestrator) Unsubscribe(ch chan DebateMessage) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i, sub := range o.subscribers {
		if sub == ch {
			o.subscribers = append(o.subscribers[:i], o.subscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

// broadcast sends a message to all active subscribers
func (o *DebateOrchestrator) broadcast(msg DebateMessage) {
	o.mu.Lock()

	o.History = append(o.History, msg)
	o.SharedContext.DebateHistory = append(o.SharedContext.DebateHistory, msg)
	o.UpdatedAt = time.Now()

	// Persist message asynchronously to avoid blocking broadcast
	if o.Repo != nil && !o.IsSimulation {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := o.Repo.AddMessage(ctx, o.ID, msg); err != nil {
				fmt.Printf("Error persisting message: %v\n", err)
			}
		}()
	}

	for _, ch := range o.subscribers {
		select {
		case ch <- msg:
		default:
			// Drop message if client is too slow to avoid blocking orchestrator
		}
	}

	o.mu.Unlock()
}

// Run executes the debate workflow
func (o *DebateOrchestrator) Run(ctx context.Context) {
	o.mu.Lock()
	o.Status = StatusRunning
	o.mu.Unlock()

	// Persist Status
	if o.Repo != nil && !o.IsSimulation {
		if err := o.Repo.UpdateStatus(ctx, o.ID, StatusRunning); err != nil {
			fmt.Printf("Error updating status: %v\n", err)
		}
	}

	// Initialize Material Pool (Single Source of Truth)
	if err := o.prepareMaterialPool(ctx); err != nil {
		o.failDebate("Failed to prepare Material Pool: " + err.Error())
		return
	}

	// ========================
	// Phase 0: Quantitative Baseline (Quant Agent)
	// ========================
	// Generate baselines before any debate starts
	quantAgent := NewRoleQuant("quant-001")
	if o.SharedContext.MaterialPool != nil {
		// 1. Generate Raw Baselines
		baselines := quantAgent.GenerateBaselineAssumptions(o.SharedContext.MaterialPool)
		o.SharedContext.BaselineAssumptions = baselines

		// 2. Broadcast Quant Agent Findings
		msg, err := quantAgent.Generate(ctx, o.SharedContext)
		if err == nil {
			o.broadcast(msg)
		} else {
			fmt.Printf("Quant Agent Error: %v\n", err)
		}
	}

	// Initialize Agents
	var macro, sentiment, fundamental, skeptic, optimist, synthesizer DebateAgent

	if o.IsSimulation {
		macro = NewMockAgent(RoleMacro)
		sentiment = NewMockAgent(RoleSentiment)
		fundamental = NewMockAgent(RoleFundamental)
		skeptic = NewMockAgent(RoleSkeptic)
		skeptic = NewMockAgent(RoleSkeptic)
		optimist = NewMockAgent(RoleOptimist)
		synthesizer = NewMockAgent(RoleSynthesizer)
	} else {
		// Research Agents: Use Universal Agent (configured in manager)
		macro = NewMacroUniversalAgent(o.AgentManager)
		sentiment = NewSentimentUniversalAgent(o.AgentManager)
		fundamental = NewFundamentalUniversalAgent(o.AgentManager)

		// Debate Agents: Use global configured provider (Qwen)
		skeptic = NewSkepticAgent(o.AgentManager)
		optimist = NewOptimistAgent(o.AgentManager)

		// Synthesizer
		synthesizer = NewSynthesizerAgent(o.AgentManager)
	}

	// Store agents for interactive Q&A
	agentMap := map[AgentRole]DebateAgent{
		RoleMacro:       macro,
		RoleSentiment:   sentiment,
		RoleFundamental: fundamental,
		RoleSkeptic:     skeptic,
		RoleOptimist:    optimist,
	}

	// ========================
	// Phase 1: Research Presentations
	// ========================
	o.mu.Lock()
	o.Phase = 1
	o.mu.Unlock()
	o.broadcast(SystemMessage("Starting Phase 1: Research Presentations"))

	o.executeAgentTurn(ctx, macro, 0)
	o.executeAgentTurn(ctx, sentiment, 0)
	o.executeAgentTurn(ctx, fundamental, 0)

	// Interactive: Pause after Phase 1 for Q&A
	if o.Mode == ModeInteractive {
		o.broadcast(SystemMessage("📋 Phase 1 Complete. You may now ask questions to any agent, or type 'continue' to proceed."))
		o.handleInteractivePhase(ctx, agentMap)
	}

	// ========================
	// Phase 2: Open Debate
	// ========================
	o.mu.Lock()
	o.Phase = 2
	o.mu.Unlock()
	o.broadcast(SystemMessage("Starting Phase 2: Open Debate"))

	// Running a shortened debate for initial version (e.g. 3 rounds instead of 10 for speed)
	for round := 1; round <= 3; round++ {
		o.broadcast(SystemMessage(fmt.Sprintf("--- Round %d ---", round)))

		o.executeAgentTurn(ctx, skeptic, round)
		o.executeAgentTurn(ctx, optimist, round)

		// Interactive: Allow questions between rounds
		if o.Mode == ModeInteractive {
			o.broadcast(SystemMessage(fmt.Sprintf("📋 Round %d Complete. Ask questions or 'continue'.", round)))
			o.handleInteractivePhase(ctx, agentMap)
		}
	}

	// ========================
	// Phase 3: Synthesis
	// ========================
	o.mu.Lock()
	o.Phase = 3
	o.mu.Unlock()

	o.generateFinalReport(ctx, synthesizer)

	// Broadcast Synthesizer's Executive Summary
	if o.FinalReport != nil && o.FinalReport.ExecutiveSummary != "" {
		o.broadcast(DebateMessage{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			Round:     99, // Special round for final report
			AgentRole: "synthesizer",
			AgentName: "Chief Investment Officer",
			Content:   o.FinalReport.ExecutiveSummary,
			Timestamp: time.Now(),
		})
	}

	o.mu.Lock()
	o.Status = StatusCompleted
	o.mu.Unlock()

	// Persist Final Report
	if o.Repo != nil && !o.IsSimulation {
		if err := o.Repo.SaveFinalReport(ctx, o.FinalReport); err != nil {
			fmt.Printf("Error saving final report: %v\n", err)
		}
	}

	o.broadcast(SystemMessage("Debate Completed. Consensus Reached."))
}

// handleInteractivePhase waits for human questions or resume signal
func (o *DebateOrchestrator) handleInteractivePhase(ctx context.Context, agentMap map[AgentRole]DebateAgent) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.resumeChan:
			o.broadcast(SystemMessage("Resuming debate..."))
			return
		case q := <-o.questionChan:
			o.processHumanQuestion(ctx, q, agentMap)
		}
	}
}

// processHumanQuestion routes a human question to the target agent
func (o *DebateOrchestrator) processHumanQuestion(ctx context.Context, q HumanQuestion, agentMap map[AgentRole]DebateAgent) {
	// Broadcast the human question
	o.broadcast(DebateMessage{
		ID:        q.ID,
		Round:     o.Phase,
		AgentRole: RoleHuman,
		AgentName: "Human Analyst",
		Content:   fmt.Sprintf("[To %s] %s", q.TargetAgent, q.Question),
		Timestamp: q.AskedAt,
	})

	// Find target agent
	agent, ok := agentMap[q.TargetAgent]
	if !ok {
		o.broadcast(SystemMessage(fmt.Sprintf("Unknown agent: %s", q.TargetAgent)))
		return
	}

	// Generate response
	o.broadcast(SystemMessage(fmt.Sprintf("%s is thinking...", agent.Name())))

	// Temporarily add the question to context for agent to see
	o.mu.Lock()
	o.SharedContext.HumanQuestions = append(o.SharedContext.HumanQuestions, q)
	o.mu.Unlock()

	msg, err := agent.Generate(ctx, o.SharedContext)
	if err != nil {
		o.broadcast(SystemMessage(fmt.Sprintf("Error from %s: %v", agent.Name(), err)))
		return
	}

	msg.Round = o.Phase
	msg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	o.broadcast(msg)

	// Update question with response
	o.mu.Lock()
	for i := range o.SharedContext.HumanQuestions {
		if o.SharedContext.HumanQuestions[i].ID == q.ID {
			o.SharedContext.HumanQuestions[i].Response = msg.Content
			o.SharedContext.HumanQuestions[i].RespondedAt = time.Now()
			break
		}
	}
	o.mu.Unlock()
}

// SubmitHumanQuestion allows external callers to submit a question
func (o *DebateOrchestrator) SubmitHumanQuestion(targetAgent AgentRole, question string) {
	q := HumanQuestion{
		ID:          fmt.Sprintf("hq-%d", time.Now().UnixNano()),
		TargetAgent: targetAgent,
		Question:    question,
		AskedAt:     time.Now(),
	}
	o.questionChan <- q
}

// ResumeDebate signals the orchestrator to continue to next phase
func (o *DebateOrchestrator) ResumeDebate() {
	select {
	case o.resumeChan <- true:
	default:
		// Channel full, ignore
	}
}

// generateFinalReport creates the executive summary and JSON data
func (o *DebateOrchestrator) generateFinalReport(ctx context.Context, synthesizer DebateAgent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 2. Generate Summary
	var msg DebateMessage
	var err error

	// ---------------------------------------------------------
	// STAGE 3.1: Human-Readable Report (Markdown)
	// ---------------------------------------------------------
	fmt.Printf("Orchestrator %s: Generating Final Board Report (Marketing Layer)...\n", o.ID)

	// Helper functions for context building
	formatTranscript := func(history []DebateMessage) string {
		var sb strings.Builder
		for _, msg := range history {
			sb.WriteString(fmt.Sprintf("[%s] %s (%s): %s\n", msg.Timestamp.Format("15:04:05"), msg.AgentName, msg.AgentRole, msg.Content))
		}
		return sb.String()
	}

	formatConsensus := func(consensus map[string]AssumptionDraft) string {
		var sb strings.Builder
		for id, draft := range consensus {
			sb.WriteString(fmt.Sprintf("  - %s: Value=%.2f, Unit=%s, Confidence=%.2f, Rationale=%s\n", id, draft.Value, draft.Unit, draft.Confidence, draft.Rationale))
		}
		return sb.String()
	}

	synthesisContextString := fmt.Sprintf(
		"Current Date: %s\nCompany: %s (%s, FY%s)\n\n=== DEBATE TRANSCRIPT ===\n%s\n\n=== CONSENSUS STATE ===\n%s",
		time.Now().Format("2006-01-02"),
		o.Company, o.Ticker, o.FiscalYear,
		formatTranscript(o.History),
		formatConsensus(o.SharedContext.CurrentConsensus),
	)

	// Create temporary SharedContext for report generation
	synthesisSharedContext := &SharedContext{
		ReportGenerationPrompt: synthesisContextString,
		DebateHistory:          o.SharedContext.DebateHistory,
		CurrentConsensus:       o.SharedContext.CurrentConsensus,
	}

	// Use the adapted SharedContext (assuming Generate supports ReportGenerationPrompt field)
	msg, err = synthesizer.Generate(ctx, synthesisSharedContext)

	var executiveSummary string
	if err != nil {
		executiveSummary = "Failed to generate summary: " + err.Error()
		fmt.Printf("Synthesizer Error: %v\n", err)
	} else {
		executiveSummary = msg.Content
	}

	// executiveSummary is now pure Markdown (as per new prompt)

	// ---------------------------------------------------------
	// STAGE 3.2: Machine-Readable Data (JSON)
	// ---------------------------------------------------------
	fmt.Printf("Orchestrator %s: Extracting Structured Data (Data Layer)...\n", o.ID)

	parsedAssumptions := make(map[string]AssumptionResult)

	// Prepare context for the JSON Extractor: The generated report + original consensus
	// Note: SynthesizerJSONPrompt should be defined in valid Go syntax in prompts.go,
	// here we refer to it. If it's not exported, we use a local definition for now.
	const LocalSynthesizerJSONPrompt = `
You are an expert financial analyst. Your task is to extract key financial assumptions from the provided report and consensus state.
Output the assumptions in a JSON array format.

CRITICAL: You MUST extract or infer values for the following REQUIRED IDs:
1. "rev_growth" (Revenue Growth Rate, decimal, e.g. 0.05 for 5%)
2. "cogs_pct" (Cost of Goods Sold as % of Revenue, decimal)
3. "sga_pct" (SG&A as % of Revenue, decimal)
4. "rd_pct" (R&D as % of Revenue, decimal)
5. "tax_rate" (Effective Tax Rate, decimal)
6. "wacc" (Weighted Average Cost of Capital, decimal)
7. "terminal_growth" (Terminal Growth Rate, decimal)

Each assumption object must have:
- parent_assumption_id: One of the required IDs above.
- value: The numerical value (decimal).
- unit: "decimal" or "USD".
- confidence: A confidence score (0.0 to 1.0).
- rationale: A brief explanation based on the debate.
- finalized_by: The agent or source that finalized this assumption.

Here is the report and consensus:

{{REPORT_CONTENT}}

---
CONSENSUS STATE:
{{CONSENSUS_STATE}}
---

Please provide only the JSON array, enclosed in ` + "```json ... ```" + `.
`
	jsonExtractionContextString := strings.Replace(LocalSynthesizerJSONPrompt, "{{REPORT_CONTENT}}", executiveSummary, 1)
	jsonExtractionContextString = strings.Replace(jsonExtractionContextString, "{{CONSENSUS_STATE}}", formatConsensus(o.SharedContext.CurrentConsensus), 1)

	// Create a temporary SharedContext for the JSON extraction prompt
	jsonExtractionSharedContext := &SharedContext{
		ReportGenerationPrompt: jsonExtractionContextString,
		DebateHistory:          o.SharedContext.DebateHistory,
		CurrentConsensus:       o.SharedContext.CurrentConsensus,
	}

	jsonResponseMsg, jsonErr := synthesizer.Generate(ctx, jsonExtractionSharedContext)

	if jsonErr != nil {
		fmt.Printf("JSON Extraction Failed: %v\n", jsonErr)
	} else {
		// Parse the output
		jsonStr := jsonResponseMsg.Content
		// Strip markdown fences if present
		jsonBlockPattern := regexp.MustCompile("(?s)```json\\s*(.*?)\\s*```")
		matches := jsonBlockPattern.FindStringSubmatch(jsonStr)
		if len(matches) > 1 {
			jsonStr = matches[1]
		}

		type JSONAssumption struct {
			ParentID    string   `json:"parent_assumption_id"`
			Value       float64  `json:"value"`
			Unit        string   `json:"unit"`
			Confidence  float64  `json:"confidence"`
			Rationale   string   `json:"rationale"`
			FinalizedBy string   `json:"finalized_by"`
			Sources     []string `json:"sources"`
			SourceURLs  []string `json:"source_urls"`
		}
		type JSONPayload struct {
			Assumptions []JSONAssumption `json:"assumptions"`
		}

		var payload JSONPayload

		// Use the specialized utils.SmartParse to handle repair, comments, and lenient syntax
		_, err := utils.SmartParse(jsonStr, &payload)
		if err != nil {
			// Fallback: The LLM might have returned a top-level array instead of an object
			var arrayPayload []JSONAssumption
			if _, err2 := utils.SmartParse(jsonStr, &arrayPayload); err2 == nil {
				payload.Assumptions = arrayPayload
			} else {
				fmt.Printf("JSON SmartParse Failed: %v\nInput Fragment: %s\n", err, jsonStr[:min(len(jsonStr), 100)]+"...")
			}
		}

		if len(payload.Assumptions) > 0 {
			fmt.Printf("Successfully parsed %d assumptions from Synthesizer JSON layer\n", len(payload.Assumptions))
			for _, asm := range payload.Assumptions {
				parsedAssumptions[asm.ParentID] = AssumptionResult{
					ParentAssumptionID: asm.ParentID,
					Value:              asm.Value,
					Unit:               asm.Unit,
					Confidence:         asm.Confidence,
					Rationale:          asm.Rationale,
					FinalizedByAgent:   asm.FinalizedBy,
					Sources:            asm.Sources,
					SourceURLs:         asm.SourceURLs,
				}
			}
		}
	}

	report := &FinalDebateReport{
		DebateID:         o.ID,
		Company:          o.Company,
		FiscalYear:       o.FiscalYear,
		CompletionTime:   time.Now(),
		Assumptions:      parsedAssumptions, // Primary source: Synthesizer LLM JSON
		ExecutiveSummary: executiveSummary,
		KeyRisks:         []string{"See Executive Summary for risks"},
		KeyOpportunities: []string{"See Executive Summary for opportunities"},
	}

	// 4. Fallback: If LLM JSON failed or was empty, use the Debate Consensus State
	if len(report.Assumptions) == 0 {
		fmt.Println("Warning: No assumptions parsed from Synthesizer JSON. Falling back to Consensus Board.")
		for k, draft := range o.SharedContext.CurrentConsensus {
			report.Assumptions[k] = AssumptionResult{
				ParentAssumptionID: draft.ParentAssumptionID,
				Value:              draft.Value,
				Unit:               draft.Unit,
				Confidence:         draft.Confidence,
				Rationale:          draft.Rationale,
				FinalizedByAgent:   "Consensus Fallback",
				Sources:            []string{"Debate Consensus"},
			}
		}
	}

	o.FinalReport = report
}

func (o *DebateOrchestrator) executeAgentTurn(ctx context.Context, agent DebateAgent, round int) {
	// Create a sub-context with timeout for each turn to prevent stalling
	turnCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	o.broadcast(SystemMessage(fmt.Sprintf("%s is thinking...", agent.Name())))

	msg, err := agent.Generate(turnCtx, o.SharedContext)
	if err != nil {
		// Log error but continue debate
		errMsg := SystemMessage(fmt.Sprintf("Error from %s: %v", agent.Name(), err))
		o.broadcast(errMsg)
		return
	}

	msg.Round = round
	msg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	o.broadcast(msg)
}

func (o *DebateOrchestrator) failDebate(reason string) {
	o.mu.Lock()
	o.Status = StatusFailed
	o.mu.Unlock()

	if o.Repo != nil && !o.IsSimulation {
		// Use background context as original context might be canceled
		go o.Repo.UpdateStatus(context.Background(), o.ID, StatusFailed)
	}

	o.broadcast(SystemMessage("Debate Failed: " + reason))
}

func (o *DebateOrchestrator) prepareMaterialPool(ctx context.Context) error {
	// Implementation Plan:
	// 1. Retrieve Primary FSAP Data (o.Company, o.FiscalYear) from DB/Repo
	// 2. Retrieve Historical FSAP Data (Previous 5 Years)
	// 3. Retrieve Qualitative Artifacts (Strategy, Risk, Segmentation)
	// 4. Assemble using MaterialPoolBuilder:
	//    builder := NewMaterialPoolBuilder(primary, history)
	//    builder.WithBusinessStrategy(strategy).WithRiskProfile(risk)...
	//    mp, err := builder.Build()

	// Current Stub:
	// Assuming upstream process has pre-populated or we load here.

	// VALIDATION: Create empty pool if nil, so we can attach transcripts
	if o.SharedContext.MaterialPool == nil {
		o.SharedContext.MaterialPool = &MaterialPool{}
	}

	// 5. Integrate Earnings Call Transcripts
	// Use Ticker to find the transcript file (e.g., AAPL.json)
	loader := NewTranscriptLoader("batch_data/transcripts")
	target := o.Ticker
	if target == "" {
		target = o.Company // Fallback if Ticker is missing, though unlikely
	}
	transcripts, err := loader.LoadTranscriptsForTicker(target)
	if err != nil {
		// Log warning but don't fail the debate, as transcripts are optional
		fmt.Printf("Warning: Could not load transcripts for %s (%s): %v\n", target, o.Company, err)
	} else {
		o.SharedContext.MaterialPool.TranscriptHistory = transcripts
		fmt.Printf("Successfully loaded %d transcripts for %s\n", len(transcripts), o.Company)
	}

	return nil
}

func SystemMessage(content string) DebateMessage {
	return DebateMessage{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		AgentRole: RoleModerator,
		AgentName: "System",
		Content:   content,
		Timestamp: time.Now(),
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
