package agent

import (
	"agentic_valuation/pkg/core/llm"
	"context"
	"fmt"
)

type Config struct {
	ActiveProvider string                 `yaml:"active_provider"`
	Agents         map[string]AgentConfig `yaml:"agents"`
}

type AgentConfig struct {
	Provider    string `yaml:"provider"` // Optional override
	Description string `yaml:"description"`
}

type Manager struct {
	config    Config
	providers map[string]llm.Provider
}

func NewManager(config Config) *Manager {
	return &Manager{
		config: config,
		providers: map[string]llm.Provider{
			"openai":   &llm.OpenAIProvider{},
			"gemini":   &llm.GeminiProvider{},
			"deepseek": &llm.DeepSeekProvider{},
			"qwen":     &llm.QwenProvider{},
			"kimi":     &llm.KimiProvider{},
			"doubao":   &llm.DoubaoProvider{},
		},
	}
}

func (m *Manager) GetProvider(agentType string) llm.Provider {
	// 1. Check for agent-specific override
	if agentConfig, ok := m.config.Agents[agentType]; ok && agentConfig.Provider != "" {
		if p, ok := m.providers[agentConfig.Provider]; ok {
			return p
		}
	}

	// 2. Use global active provider
	if p, ok := m.providers[m.config.ActiveProvider]; ok {
		return p
	}

	// 3. Fallback
	return m.providers["openai"]
}

// GetProviderByName retrieves a provider instance by its specific name (e.g. "deepseek", "gemini")
func (m *Manager) GetProviderByName(name string) llm.Provider {
	fmt.Printf("[DEBUG] Manager looking for provider: '%s'\n", name)
	if p, ok := m.providers[name]; ok {
		fmt.Printf("[DEBUG] Found provider: %T\n", p)
		return p
	}
	fmt.Printf("[DEBUG] Provider '%s' NOT FOUND in map keys: ", name)
	for k := range m.providers {
		fmt.Printf("%s, ", k)
	}
	fmt.Println()
	return nil
}

// ExecutePrompt handles instruction adaptation before sending to the model
func (m *Manager) ExecutePrompt(agentType string, rawPrompt string, rawSystemPrompt string, options map[string]interface{}) (string, error) {
	provider := m.GetProvider(agentType)

	// Debug: Log which provider we're using
	fmt.Printf("[DEBUG] ExecutePrompt: agentType=%s, activeProvider=%s, providerType=%T\n", agentType, m.config.ActiveProvider, provider)

	// Adapt instructions based on the model's specialized "teaching" style
	adaptedSystemPrompt := provider.AdaptInstructions(rawSystemPrompt)

	return provider.GenerateResponse(context.Background(), rawPrompt, adaptedSystemPrompt, options)
}

func (m *Manager) SetGlobalProvider(newProvider string) error {
	if _, ok := m.providers[newProvider]; !ok {
		return fmt.Errorf("provider %s not found", newProvider)
	}
	m.config.ActiveProvider = newProvider
	fmt.Printf("Global provider set to: %s\n", newProvider)
	return nil
}

func (m *Manager) GetActiveProvider() string {
	return m.config.ActiveProvider
}
