package prompt

// Convenience functions for common prompt operations

// GetDebatePrompt returns a debate agent's system prompt by role name
func GetDebatePrompt(role string) (string, error) {
	id := "debate." + role
	return Get().GetSystemPrompt(id)
}

// GetExtractionPrompt returns the prompt ID for a specific financial statement extraction task
// now mapping to V2 Table Mapper prompts to ensure legacy code using this function gets the new behavior
// or errors out if explicit legacy behavior was expected.
func GetExtractionPrompt(statementType string) (string, error) {
	// Map legacy short names to V2 prompts if possible, or error out
	switch statementType {
	case "balance_sheet":
		return "extraction.v2_table_mapper_balance_sheet", nil
	case "income_statement":
		return "extraction.v2_table_mapper_income_statement", nil
	case "cash_flow":
		return "extraction.v2_table_mapper_cash_flow", nil
	default:
		// Fallback to generic ID construction for other types
		id := "extraction." + statementType
		return Get().GetSystemPrompt(id)
	}
}

// GetQualitativePrompt returns a qualitative agent's system prompt
func GetQualitativePrompt(agentType string) (string, error) {
	id := "qualitative." + agentType
	return Get().GetSystemPrompt(id)
}

// GetAssistantPrompt returns an assistant prompt
func GetAssistantPrompt(name string) (string, error) {
	id := "assistant." + name
	return Get().GetSystemPrompt(id)
}

// MustGetDebatePrompt is like GetDebatePrompt but panics on error
func MustGetDebatePrompt(role string) string {
	p, err := GetDebatePrompt(role)
	if err != nil {
		panic(err)
	}
	return p
}

// MustGetExtractionPrompt is like GetExtractionPrompt but panics on error
func MustGetExtractionPrompt(statementType string) string {
	p, err := GetExtractionPrompt(statementType)
	if err != nil {
		panic(err)
	}
	return p
}

// PromptIDs contains all known prompt identifiers
var PromptIDs = struct {
	// Debate agents
	DebateMacro       string
	DebateSentiment   string
	DebateFundamental string
	DebateSkeptic     string
	DebateOptimist    string
	DebateSynthesizer string

	// Extraction
	// Deprecated: ExtractionBalanceSheet
	// Deprecated: ExtractionIncomeStatement
	// Deprecated: ExtractionCashFlow

	// V2 Extraction Prompts (Navigator + Table Mapper)
	ExtractionNavigatorTOC string
	ExtractionSupplemental string
	ExtractionTOC          string
	ExtractionNotes        string
	ExtractionBaseRules    string
	NoteIndex              string

	// Qualitative
	QualitativeStrategy          string
	QualitativeCapitalAllocation string
	QualitativeSegment           string
	QualitativeRisk              string

	// Assistant
	AssistantNavigation string
}{
	DebateMacro:       "debate.macro",
	DebateSentiment:   "debate.sentiment",
	DebateFundamental: "debate.fundamental",
	DebateSkeptic:     "debate.skeptic",
	DebateOptimist:    "debate.optimist",
	DebateSynthesizer: "debate.synthesizer",

	// ExtractionBalanceSheet: "extraction.balance_sheet",
	// ExtractionIncomeStatement: "extraction.income_statement",
	// ExtractionCashFlow: "extraction.cash_flow",

	ExtractionNavigatorTOC: "extraction.v2_navigator_toc",
	ExtractionSupplemental: "extraction.supplemental",
	ExtractionTOC:          "extraction.toc_analysis",
	ExtractionNotes:        "extraction.notes_analysis",
	ExtractionBaseRules:    "extraction.base_rules",
	NoteIndex:              "extraction.note_index",

	QualitativeStrategy:          "qualitative.strategy",
	QualitativeCapitalAllocation: "qualitative.capital_allocation",
	QualitativeSegment:           "qualitative.segment",
	QualitativeRisk:              "qualitative.risk",

	AssistantNavigation: "assistant.navigation",
}
