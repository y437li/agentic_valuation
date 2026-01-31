package debate

import (
	"context"
	"sync"
	"time"

	"agentic_valuation/pkg/core/agent"

	"github.com/google/uuid"
)

// DebateManager is a singleton that manages all active background debates
type DebateManager struct {
	activeDebates map[string]*DebateOrchestrator
	repo          *DebateRepo
	agentManager  *agent.Manager
	mu            sync.RWMutex
}

var (
	instance *DebateManager
	once     sync.Once
)

// GetManager returns the singleton instance of DebateManager
func GetManager() *DebateManager {
	once.Do(func() {
		instance = &DebateManager{
			activeDebates: make(map[string]*DebateOrchestrator),
			repo:          NewDebateRepo(),
			// agentManager is nil initially, set via SetAgentManager
		}
		// Start background cleanup routine
		go instance.cleanup()
	})
	return instance
}

// SetAgentManager injects the global agent manager
func (m *DebateManager) SetAgentManager(mgr *agent.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentManager = mgr
}

// StartDebate initializes a new debate and runs it in a background goroutine
func (m *DebateManager) StartDebate(ticker, company, fiscalYear string, isSimulation bool, mode DebateMode) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	orchestrator := NewOrchestrator(id, ticker, company, fiscalYear, isSimulation, mode, m.agentManager, m.repo)
	m.activeDebates[id] = orchestrator

	// Run debate in background
	go func() {
		ctx := context.Background() // Separate context for background job
		orchestrator.Run(ctx)
	}()

	return id, nil
}

// GetDebate retrieves an existing orchestrator by ID
func (m *DebateManager) GetDebate(id string) (*DebateOrchestrator, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	orch, exists := m.activeDebates[id]
	return orch, exists
}

// GetActiveDebates returns a list of currently running debates
func (m *DebateManager) GetActiveDebates() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []string
	for id, orch := range m.activeDebates {
		if orch.Status == StatusRunning {
			active = append(active, id)
		}
	}
	return active
}

// cleanup removes completed debates older than 24 hours
func (m *DebateManager) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		m.mu.Lock()
		for id, orch := range m.activeDebates {
			if orch.Status != StatusRunning && time.Since(orch.UpdatedAt) > 24*time.Hour {
				delete(m.activeDebates, id)
			}
		}
		m.mu.Unlock()
	}
}
