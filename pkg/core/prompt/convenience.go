package prompt

// Convenience functions for common prompt operations

// GetDebatePrompt returns a debate agent's system prompt by role name
func GetDebatePrompt(role string) (string, error) {
	id := "debate." + role
	return Get().GetSystemPrompt(id)
}

// GetExtractionPrompt returns an extraction prompt by statement type
func GetExtractionPrompt(statementType string) (string, error) {
	id := "extraction." + statementType
	return Get().GetSystemPrompt(id)
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
	ExtractionBalanceSheet    string
	ExtractionIncomeStatement string
	ExtractionCashFlow        string
	ExtractionSupplemental    string
	ExtractionTOC             string
	ExtractionNotes           string
	ExtractionBaseRules       string
	NoteIndex                 string

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

	ExtractionBalanceSheet:    "extraction.balance_sheet",
	ExtractionIncomeStatement: "extraction.income_statement",
	ExtractionCashFlow:        "extraction.cash_flow",
	ExtractionSupplemental:    "extraction.supplemental",
	ExtractionTOC:             "extraction.toc_analysis",
	ExtractionNotes:           "extraction.notes_analysis",
	ExtractionBaseRules:       "extraction.base_rules",
	NoteIndex:                 "extraction.note_index",

	QualitativeStrategy:          "qualitative.strategy",
	QualitativeCapitalAllocation: "qualitative.capital_allocation",
	QualitativeSegment:           "qualitative.segment",
	QualitativeRisk:              "qualitative.risk",

	AssistantNavigation: "assistant.navigation",
}
