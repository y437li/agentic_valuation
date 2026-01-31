// Package prompt provides a centralized prompt library for LLM interactions.
// It allows prompts to be defined in JSON files and loaded at runtime,
// making it easy to update prompts without code changes.
package prompt

// PromptTemplate represents a reusable prompt with metadata
type PromptTemplate struct {
	ID               string           `json:"id"`                   // Unique identifier (e.g., "extraction.balance_sheet")
	Name             string           `json:"name"`                 // Human-readable name
	Category         string           `json:"category"`             // Category (debate, extraction, qualitative, etc.)
	Description      string           `json:"description"`          // Description of prompt purpose
	SystemPrompt     string           `json:"system_prompt"`        // The system prompt content
	UserPromptTmpl   string           `json:"user_prompt_template"` // Go template for user prompt
	ResponseSchemaID string           `json:"response_schema_ref"`  // Reference to response schema
	Variables        []PromptVariable `json:"variables"`            // Variables used in template
	Version          string           `json:"version"`              // Version for tracking changes
}

// PromptVariable defines a variable used in a prompt template
type PromptVariable struct {
	Name        string `json:"name"`        // Variable name (e.g., "CompanyName")
	Type        string `json:"type"`        // Type: string, int, float, array, object
	Description string `json:"description"` // What this variable represents
	Required    bool   `json:"required"`    // Whether this variable is required
	Default     string `json:"default"`     // Default value if not provided
}

// ResponseSchema represents the expected JSON response structure
type ResponseSchema struct {
	ID          string `json:"id"`          // Schema identifier
	Name        string `json:"name"`        // Human-readable name
	Description string `json:"description"` // Description of the schema
	JSONSchema  string `json:"json_schema"` // JSON Schema definition as string
}

// PromptExecutionContext holds runtime values for prompt execution
type PromptExecutionContext struct {
	Variables map[string]interface{} // Key-value pairs for template substitution
}

// NewContext creates a new execution context
func NewContext() *PromptExecutionContext {
	return &PromptExecutionContext{
		Variables: make(map[string]interface{}),
	}
}

// Set adds a variable to the context
func (c *PromptExecutionContext) Set(key string, value interface{}) *PromptExecutionContext {
	c.Variables[key] = value
	return c
}
