package llm

import (
	"context"
)

// Provider is the interface for all LLM providers.
type Provider interface {
	GenerateResponse(ctx context.Context, prompt string, systemPrompt string, options map[string]interface{}) (string, error)
	// AdaptInstructions transforms raw instructions into model-specific formats
	AdaptInstructions(rawInstructions string) string
}

type OpenAIProvider struct{}

func (p *OpenAIProvider) GenerateResponse(ctx context.Context, prompt string, systemPrompt string, options map[string]interface{}) (string, error) {
	// OpenAI specific API call logic
	return "Not implemented: OpenAI Response", nil
}

func (p *OpenAIProvider) AdaptInstructions(raw string) string {
	return "OpenAI Style: " + raw // Template for GPT-specific prompting
}

type KimiProvider struct{}

func (p *KimiProvider) GenerateResponse(ctx context.Context, prompt string, systemPrompt string, options map[string]interface{}) (string, error) {
	return "Not implemented: Kimi Response", nil
}

func (p *KimiProvider) AdaptInstructions(raw string) string {
	return "Kimi Style: " + raw // Kimi is optimized for long-context financial analysis
}

type DoubaoProvider struct{}

func (p *DoubaoProvider) GenerateResponse(ctx context.Context, prompt string, systemPrompt string, options map[string]interface{}) (string, error) {
	return "Not implemented: Doubao Response", nil
}

func (p *DoubaoProvider) AdaptInstructions(raw string) string {
	return "Doubao Style: " + raw // Doubao (ByteDance) has strong performance in creative and localized tasks
}
