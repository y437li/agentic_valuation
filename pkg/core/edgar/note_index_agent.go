// Package edgar - NoteIndexAgent for indexing Notes to Financial Statements
package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"agentic_valuation/pkg/core/prompt"
)

// =============================================================================
// NOTE INDEX AGENT - Builds an index of Notes to Financial Statements
// =============================================================================

// NoteEntry represents a single note in the financial statements
type NoteEntry struct {
	NoteNumber int    `json:"note_number"`          // e.g., 13
	Title      string `json:"title"`                // e.g., "Segment Information and Geographic Data"
	StartLine  int    `json:"start_line,omitempty"` // Approximate line number in the document
	Category   string `json:"category,omitempty"`   // Standardized category (see NoteCategoryXxx)
}

// Standardized Note Categories (SEC-mandated disclosures)
const (
	NoteCategorySegment          = "segment_info"
	NoteCategoryRevenue          = "revenue_recognition"
	NoteCategoryIncomeTax        = "income_taxes"
	NoteCategoryDebt             = "debt"
	NoteCategoryLeases           = "leases"
	NoteCategoryFairValue        = "fair_value"
	NoteCategoryStockComp        = "stock_compensation"
	NoteCategoryPension          = "pension"
	NoteCategoryContingencies    = "contingencies"
	NoteCategoryRelatedParty     = "related_party"
	NoteCategorySubsequentEvents = "subsequent_events"
	NoteCategoryAccountingPolicy = "accounting_policies"
	NoteCategoryEPS              = "earnings_per_share"
	NoteCategoryIntangibles      = "intangibles"
	NoteCategoryGoodwill         = "goodwill"
	NoteCategoryInventory        = "inventory"
	NoteCategoryPPE              = "property_plant_equipment"
	NoteCategoryOther            = "other"
)

// NoteIndex provides quick lookup of notes by category
type NoteIndex struct {
	Notes      []NoteEntry          `json:"notes"`       // All notes in order
	ByCategory map[string]NoteEntry `json:"by_category"` // Lookup by standardized category
}

// GetNote retrieves a note by its standardized category
func (idx *NoteIndex) GetNote(category string) *NoteEntry {
	if idx == nil || idx.ByCategory == nil {
		return nil
	}
	if note, ok := idx.ByCategory[category]; ok {
		return &note
	}
	return nil
}

// GetSegmentNote is a convenience method
func (idx *NoteIndex) GetSegmentNote() *NoteEntry {
	return idx.GetNote(NoteCategorySegment)
}

// NoteIndexAgent parses the Notes section to build a structured index
type NoteIndexAgent struct {
	provider AIProvider
}

// NewNoteIndexAgent creates a new note index agent
func NewNoteIndexAgent(provider AIProvider) *NoteIndexAgent {
	return &NoteIndexAgent{provider: provider}
}

// BuildIndex analyzes the Notes section and returns a structured index
func (a *NoteIndexAgent) BuildIndex(ctx context.Context, notesText string) (*NoteIndex, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Step 1: Try regex-based extraction first (faster, no LLM cost)
	index := a.regexExtract(notesText)
	if len(index.Notes) >= 5 {
		// Got a reasonable number of notes via regex, categorize them
		return a.categorizeNotes(ctx, index)
	}

	// Step 2: Fall back to LLM-based extraction
	return a.llmExtract(ctx, notesText)
}

// regexExtract attempts to find notes using common patterns
func (a *NoteIndexAgent) regexExtract(notesText string) *NoteIndex {
	index := &NoteIndex{
		Notes:      make([]NoteEntry, 0),
		ByCategory: make(map[string]NoteEntry),
	}

	// Common patterns for note headers:
	// "Note 1 – Summary of Significant Accounting Policies"
	// "NOTE 13 — SEGMENT INFORMATION AND GEOGRAPHIC DATA"
	// "1. Summary of Significant Accounting Policies"
	// "Note 1: Basis of Presentation"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)Note\s+(\d+)\s*[-–—:\.]\s*(.+?)(?:\n|$)`),
		regexp.MustCompile(`(?i)^(\d+)\.\s+([A-Z][^\.]+?)(?:\n|$)`),
	}

	lines := strings.Split(notesText, "\n")
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		for _, pattern := range patterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) >= 3 {
				var noteNum int
				fmt.Sscanf(matches[1], "%d", &noteNum)
				title := strings.TrimSpace(matches[2])
				// Skip if title is too short or looks like garbage
				if len(title) > 5 && noteNum > 0 && noteNum < 50 {
					index.Notes = append(index.Notes, NoteEntry{
						NoteNumber: noteNum,
						Title:      title,
						StartLine:  lineNum + 1,
					})
				}
				break
			}
		}
	}

	return index
}

// categorizeNotes uses LLM to map note titles to standardized categories
func (a *NoteIndexAgent) categorizeNotes(ctx context.Context, index *NoteIndex) (*NoteIndex, error) {
	if len(index.Notes) == 0 {
		return index, nil
	}

	// Build a list of notes for the LLM
	var noteList strings.Builder
	for _, note := range index.Notes {
		noteList.WriteString(fmt.Sprintf("Note %d: %s\n", note.NoteNumber, note.Title))
	}

	systemPrompt := `You are a Financial Reporting Expert. Categorize each note into its standardized SEC disclosure category.`

	userPrompt := fmt.Sprintf(`Match each note to its standardized category.

NOTES:
%s

CATEGORIES (use exact keys):
- segment_info: Segment Information, Geographic Data, Reportable Segments
- revenue_recognition: Revenue Recognition, Revenue from Contracts, ASC 606
- income_taxes: Income Taxes, Tax Provision
- debt: Debt, Borrowings, Credit Facilities
- leases: Leases, Operating Leases, Finance Leases
- fair_value: Fair Value Measurements, Level 1/2/3
- stock_compensation: Stock-Based Compensation, Share-Based Awards
- pension: Pension, Retirement Benefits, Postretirement
- contingencies: Contingencies, Commitments, Legal Proceedings
- related_party: Related Party Transactions
- subsequent_events: Subsequent Events
- accounting_policies: Significant Accounting Policies, Basis of Presentation
- earnings_per_share: Earnings Per Share, EPS
- intangibles: Intangible Assets
- goodwill: Goodwill, Acquisitions
- inventory: Inventory, Inventories
- property_plant_equipment: Property Plant and Equipment, PP&E
- other: Any note not matching above

Return JSON ONLY:
{
  "mappings": [
    {"note_number": 1, "category": "accounting_policies"},
    {"note_number": 13, "category": "segment_info"}
  ]
}`, noteList.String())

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		// On error, return index without categories
		return index, nil
	}

	// Parse response
	type CategoryMapping struct {
		NoteNumber int    `json:"note_number"`
		Category   string `json:"category"`
	}
	type MappingResponse struct {
		Mappings []CategoryMapping `json:"mappings"`
	}

	var mapping MappingResponse
	if err := parseJSONSafe(resp, &mapping); err != nil {
		return index, nil
	}

	// Apply categories
	categoryMap := make(map[int]string)
	for _, m := range mapping.Mappings {
		categoryMap[m.NoteNumber] = m.Category
	}

	for i := range index.Notes {
		if cat, ok := categoryMap[index.Notes[i].NoteNumber]; ok {
			index.Notes[i].Category = cat
			index.ByCategory[cat] = index.Notes[i]
		}
	}

	return index, nil
}

// llmExtract uses LLM to extract notes when regex fails
func (a *NoteIndexAgent) llmExtract(ctx context.Context, notesText string) (*NoteIndex, error) {
	// Truncate to first 30k chars (usually enough to get the note headers/TOC)
	if len(notesText) > 30000 {
		notesText = notesText[:30000] + "\n... [truncated]"
	}

	// Try to load from prompt library
	systemPrompt := getNoteIndexPrompt()

	userPrompt := fmt.Sprintf(`Analyze the Notes to Financial Statements section and extract all note numbers and titles.

TEXT:
%s

Return JSON ONLY:
{
  "notes": [
    {"note_number": 1, "title": "Summary of Significant Accounting Policies", "category": "accounting_policies"},
    {"note_number": 2, "title": "Revenue Recognition", "category": "revenue_recognition"},
    {"note_number": 13, "title": "Segment Information and Geographic Data", "category": "segment_info"}
  ]
}

IMPORTANT:
- Extract ALL notes found (typically 15-30 notes)
- Use exact note numbers as shown in document
- Map to standardized categories: segment_info, revenue_recognition, income_taxes, debt, leases, fair_value, stock_compensation, pension, contingencies, related_party, subsequent_events, accounting_policies, earnings_per_share, intangibles, goodwill, inventory, property_plant_equipment, other`, notesText)

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	var result struct {
		Notes []NoteEntry `json:"notes"`
	}
	if err := parseJSONSafe(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse note index JSON: %w", err)
	}

	// Build the index
	index := &NoteIndex{
		Notes:      result.Notes,
		ByCategory: make(map[string]NoteEntry),
	}

	for _, note := range index.Notes {
		if note.Category != "" {
			index.ByCategory[note.Category] = note
		}
	}

	return index, nil
}

// ExtractNoteContent extracts the full text content of a specific note
func (a *NoteIndexAgent) ExtractNoteContent(ctx context.Context, notesText string, noteEntry *NoteEntry) (string, error) {
	if noteEntry == nil {
		return "", fmt.Errorf("note entry is nil")
	}

	// Try to extract via regex first (find boundaries)
	content := a.regexExtractContent(notesText, noteEntry)
	if len(content) > 500 {
		return content, nil
	}

	// Fall back to LLM extraction
	return a.llmExtractContent(ctx, notesText, noteEntry)
}

// regexExtractContent tries to find note content via regex
func (a *NoteIndexAgent) regexExtractContent(notesText string, noteEntry *NoteEntry) string {
	// Build pattern for this note and the next note
	thisNote := fmt.Sprintf(`(?i)Note\s+%d\s*[-–—:\.]\s*`, noteEntry.NoteNumber)
	nextNote := fmt.Sprintf(`(?i)Note\s+%d\s*[-–—:\.]\s*`, noteEntry.NoteNumber+1)

	thisPattern := regexp.MustCompile(thisNote)
	nextPattern := regexp.MustCompile(nextNote)

	thisLoc := thisPattern.FindStringIndex(notesText)
	if thisLoc == nil {
		return ""
	}

	nextLoc := nextPattern.FindStringIndex(notesText)
	if nextLoc == nil {
		// Last note - take up to 20k chars or end
		end := thisLoc[0] + 20000
		if end > len(notesText) {
			end = len(notesText)
		}
		return notesText[thisLoc[0]:end]
	}

	return notesText[thisLoc[0]:nextLoc[0]]
}

// llmExtractContent uses LLM to extract note content
func (a *NoteIndexAgent) llmExtractContent(ctx context.Context, notesText string, noteEntry *NoteEntry) (string, error) {
	if a.provider == nil {
		return "", fmt.Errorf("no AI provider configured")
	}

	// Truncate to reasonable size
	if len(notesText) > 50000 {
		notesText = notesText[:50000] + "\n... [truncated]"
	}

	systemPrompt := "You are a Financial Data Locator. Extract the complete text of a specific note."

	userPrompt := fmt.Sprintf(`Find and extract the COMPLETE text of Note %d: "%s".

Extract from the start of Note %d to the start of Note %d (or end of notes section).
Return ONLY the note content, no JSON wrapper.

TEXT:
%s`, noteEntry.NoteNumber, noteEntry.Title, noteEntry.NoteNumber, noteEntry.NoteNumber+1, notesText)

	return a.provider.Generate(ctx, systemPrompt, userPrompt)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// parseJSONSafe extracts and parses JSON from potentially wrapped response
func parseJSONSafe(resp string, v interface{}) error {
	// Remove markdown code blocks
	cleanJson := strings.ReplaceAll(resp, "```json", "")
	cleanJson = strings.ReplaceAll(cleanJson, "```", "")
	cleanJson = strings.TrimSpace(cleanJson)

	// Find JSON boundaries
	start := strings.Index(cleanJson, "{")
	end := strings.LastIndex(cleanJson, "}")
	if start >= 0 && end > start {
		cleanJson = cleanJson[start : end+1]
	}

	return json.Unmarshal([]byte(cleanJson), v)
}

// getNoteIndexPrompt returns the system prompt for note indexing
func getNoteIndexPrompt() string {
	if p, err := prompt.Get().GetSystemPrompt(prompt.PromptIDs.NoteIndex); err == nil && p != "" {
		return p
	}
	// Fallback
	return `You are a Financial Reporting Expert specializing in SEC 10-K filings.
Analyze the Notes to Financial Statements section and build a structured index of all notes.
Each note should be mapped to its standardized SEC disclosure category.
Be precise with note numbers - they vary by company (e.g., Segment Info could be Note 11, 13, 20, or 25).`
}
