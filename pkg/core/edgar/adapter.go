package edgar

import (
	"agentic_valuation/pkg/core/llm"
	"context"
)

// LLMAdapter bridges llm.Provider interface to edgar.AIProvider interface
type LLMAdapter struct {
	provider llm.Provider
}

// NewLLMAdapter creates a new adapter wrapping an llm.Provider
func NewLLMAdapter(provider llm.Provider) *LLMAdapter {
	return &LLMAdapter{provider: provider}
}

// Generate implements edgar.AIProvider interface
func (a *LLMAdapter) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	// llm.Provider.GenerateResponse has (prompt, systemPrompt) order
	// edgar.AIProvider.Generate has (systemPrompt, userPrompt) order
	return a.provider.GenerateResponse(ctx, userPrompt, systemPrompt, nil)
}
