package edgar

import (
	"agentic_valuation/pkg/core/llm"
	"context"
)

// DeepSeekAIProvider wraps the core llm.DeepSeekProvider to implement edgar.AIProvider interface
// This is used for manual integration tests
type DeepSeekAIProvider struct {
	provider *llm.DeepSeekProvider
}

// Generate delegates to the underlying provider
func (p *DeepSeekAIProvider) Generate(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.provider.GenerateResponse(ctx, userPrompt, systemPrompt, map[string]interface{}{})
}
