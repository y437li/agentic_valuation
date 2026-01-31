package debate

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockAgent provides deterministic responses for testing
type MockAgent struct {
	role       AgentRole
	fixedReply string
	latency    time.Duration
}

func NewMockAgent(role AgentRole) *MockAgent {
	return &MockAgent{
		role:    role,
		latency: 500 * time.Millisecond,
	}
}

func (a *MockAgent) Role() AgentRole {
	return a.role
}

func (a *MockAgent) Name() string {
	return fmt.Sprintf("Mock %s", strings.Title(string(a.role)))
}

func (a *MockAgent) Generate(ctx context.Context, shared *SharedContext) (DebateMessage, error) {
	// Simulate "thinking" latency
	select {
	case <-time.After(a.latency):
	case <-ctx.Done():
		return DebateMessage{}, ctx.Err()
	}

	content := fmt.Sprintf("[%s Simulation] I have analyzed %s for FY%s. ", a.Role(), shared.Company, shared.FiscalYear)

	refs := []string{"[Mock Source 1](http://example.com)"}

	// Add role-specific flavor to mock content
	switch a.role {
	case RoleMacro:
		content += "GDP is growing at 2.5%, Interest rates stable. Inflation at 3%."
	case RoleSentiment:
		content += "Market sentiment is cautiously optimistic. Analyst buy ratings 70%."
	case RoleFundamental:
		content += "Revenue growth strong at 12% YoY. Margins expanding to 15%."
	case RoleSkeptic:
		content += "But what about the regulatory risks in the EV sector? Debt levels are concerning."
	case RoleOptimist:
		content += "Innovation pipeline is robust. Market share gains will offset debt risks."
	}

	return DebateMessage{
		AgentRole:  a.Role(),
		AgentName:  a.Name(),
		Content:    content,
		References: refs,
		Timestamp:  time.Now(),
	}, nil
}
