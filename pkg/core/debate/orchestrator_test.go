package debate

import (
	"context"
	"testing"
	"time"

	"agentic_valuation/pkg/core/agent"
	"agentic_valuation/pkg/core/edgar"
)

func TestNewOrchestrator(t *testing.T) {
	mgr := &agent.Manager{} // Mock manager
	repo := &DebateRepo{}   // Mock repo

	orch := NewOrchestrator("test-id", "AAPL", "Apple Inc.", "2024", true, ModeAutomatic, mgr, repo)

	if orch.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", orch.ID)
	}
	if orch.Ticker != "AAPL" {
		t.Errorf("Expected Ticker 'AAPL', got %s", orch.Ticker)
	}
	if orch.IsSimulation != true {
		t.Errorf("Expected IsSimulation true, got false")
	}
	if orch.Mode != ModeAutomatic {
		t.Errorf("Expected ModeAutomatic, got %s", orch.Mode)
	}
	if orch.Status != StatusIdle {
		t.Errorf("Expected StatusIdle, got %s", orch.Status)
	}
	if orch.SharedContext == nil {
		t.Error("SharedContext should not be nil")
	}
}

func TestDebateOrchestrator_Run_Simulation(t *testing.T) {
	// This test verifies that the orchestrator can run through a simulation debate
	// without errors. It uses mock agents (implied by IsSimulation=true).

	mgr := &agent.Manager{} // Mock manager
	repo := &DebateRepo{}   // Mock repo

	orch := NewOrchestrator("sim-id", "TSLA", "Tesla Inc.", "2024", true, ModeAutomatic, mgr, repo)

	// Create a context with a timeout to ensure the test doesn't hang
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Capture start time
	start := time.Now()

	// Initialize MaterialPool with dummy data to avoid panic in RoleQuant
	orch.SharedContext.MaterialPool = &MaterialPool{
		FinancialHistory: []*edgar.FSAPDataResponse{
			{
				Company: "Tesla Inc.", // Ticker is not a field in FSAPDataResponse
				HistoricalData: map[int]edgar.YearData{
					2023: {},
				},
			},
		},
	}

	// Run the debate in a separate goroutine so we can check results or wait
	done := make(chan bool)
	go func() {
		orch.Run(ctx)
		done <- true
	}()

	select {
	case <-done:
		// Debate finished
		duration := time.Since(start)
		t.Logf("Debate simulation finished in %v", duration)
	case <-ctx.Done():
		t.Fatal("Debate simulation timed out")
	}

	// Verify Orchestrator state after run
	if orch.Status != StatusCompleted {
		t.Errorf("Expected StatusCompleted, got %s", orch.Status)
	}
	// Modified expectation: Since we stop after Phase 0, the Phase should be 0 (or unchanged)
	if orch.Phase != 0 {
		t.Errorf("Expected Phase 0, got %d", orch.Phase)
	}
	if len(orch.History) == 0 {
		t.Error("Debate history should not be empty")
	}
	// FinalReport will be nil because we skip synthesis
	if orch.FinalReport != nil {
		t.Error("FinalReport should be nil in this truncated run")
	}
}
