package prompt

import (
	"fmt"
	"sync"
)

// Registry holds all loaded prompts and schemas
type Registry struct {
	prompts map[string]*PromptTemplate
	schemas map[string]*ResponseSchema
	mu      sync.RWMutex
}

var globalRegistry *Registry
var once sync.Once

// Get returns the global registry singleton
func Get() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			prompts: make(map[string]*PromptTemplate),
			schemas: make(map[string]*ResponseSchema),
		}
	})
	return globalRegistry
}

// Register adds a prompt template to the registry
func (r *Registry) Register(pt *PromptTemplate) error {
	if pt.ID == "" {
		return fmt.Errorf("prompt ID cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.prompts[pt.ID] = pt
	return nil
}

// RegisterSchema adds a response schema to the registry
func (r *Registry) RegisterSchema(schema *ResponseSchema) error {
	if schema.ID == "" {
		return fmt.Errorf("schema ID cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.schemas[schema.ID] = schema
	return nil
}

// GetPrompt retrieves a prompt by ID
func (r *Registry) GetPrompt(id string) (*PromptTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if p, ok := r.prompts[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("prompt not found: %s", id)
}

// GetSchema retrieves a response schema by ID
func (r *Registry) GetSchema(id string) (*ResponseSchema, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if s, ok := r.schemas[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("schema not found: %s", id)
}

// GetSystemPrompt is a convenience method to get only the system prompt string
func (r *Registry) GetSystemPrompt(id string) (string, error) {
	pt, err := r.GetPrompt(id)
	if err != nil {
		return "", err
	}
	return pt.SystemPrompt, nil
}

// ListPrompts returns all registered prompt IDs
func (r *Registry) ListPrompts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.prompts))
	for id := range r.prompts {
		ids = append(ids, id)
	}
	return ids
}

// ListByCategory returns all prompts in a specific category
func (r *Registry) ListByCategory(category string) []*PromptTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*PromptTemplate
	for _, pt := range r.prompts {
		if pt.Category == category {
			result = append(result, pt)
		}
	}
	return result
}

// Count returns the number of registered prompts
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.prompts)
}

// Clear removes all prompts (useful for testing)
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prompts = make(map[string]*PromptTemplate)
	r.schemas = make(map[string]*ResponseSchema)
}
