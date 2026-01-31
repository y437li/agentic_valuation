package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// NoteExtractor extracts all notes from SEC filings
type NoteExtractor struct {
	provider AIProvider
}

// NewNoteExtractor creates a new note extractor
func NewNoteExtractor(provider AIProvider) *NoteExtractor {
	return &NoteExtractor{provider: provider}
}

// ExtractAllNotes extracts all notes from a filing's markdown
func (e *NoteExtractor) ExtractAllNotes(ctx context.Context, markdown string, meta *FilingMetadata) ([]*ExtractedNote, error) {
	// Step 1: Locate Notes section
	notesSection := e.locateNotesSection(markdown)
	if notesSection == "" {
		return nil, fmt.Errorf("notes section not found in filing")
	}

	// Step 2: Split into individual notes
	noteTexts := e.splitIntoNotes(notesSection)
	if len(noteTexts) == 0 {
		return nil, fmt.Errorf("no individual notes found")
	}

	fmt.Printf("  Found %d notes in filing\n", len(noteTexts))

	// Step 3: Extract each note
	var notes []*ExtractedNote
	for _, noteText := range noteTexts {
		note, err := e.extractSingleNote(ctx, noteText, meta)
		if err != nil {
			fmt.Printf("  Warning: failed to extract note: %v\n", err)
			continue
		}
		notes = append(notes, note)
	}

	return notes, nil
}

// locateNotesSection finds the Notes to Financial Statements section
func (e *NoteExtractor) locateNotesSection(markdown string) string {
	lines := strings.Split(markdown, "\n")

	// Patterns to identify start of Notes section
	startPatterns := []string{
		"NOTES TO CONSOLIDATED FINANCIAL STATEMENTS",
		"NOTES TO FINANCIAL STATEMENTS",
		"NOTES TO THE CONSOLIDATED FINANCIAL STATEMENTS",
		"Note 1",
		"NOTE 1",
	}

	startIdx := -1
	for i, line := range lines {
		upperLine := strings.ToUpper(line)
		for _, pattern := range startPatterns {
			if strings.Contains(upperLine, strings.ToUpper(pattern)) {
				startIdx = i
				break
			}
		}
		if startIdx >= 0 {
			break
		}
	}

	if startIdx < 0 {
		return ""
	}

	// Notes section typically runs until next major section or end
	// Take up to 50000 chars to avoid context limits
	endIdx := startIdx + 2000 // ~2000 lines max
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	section := strings.Join(lines[startIdx:endIdx], "\n")
	if len(section) > 100000 {
		section = section[:100000]
	}

	return section
}

// splitIntoNotes splits the notes section into individual notes
func (e *NoteExtractor) splitIntoNotes(notesSection string) []string {
	// Pattern: "Note 1", "Note 2", "NOTE 1.", etc.
	notePattern := regexp.MustCompile(`(?mi)^(?:#{1,3}\s*)?(?:NOTE|Note)\s*(\d+[A-Z]?)[\.\:\s\—\-]+(.*)`)

	matches := notePattern.FindAllStringSubmatchIndex(notesSection, -1)
	if len(matches) == 0 {
		return nil
	}

	var notes []string
	for i, match := range matches {
		start := match[0]
		end := len(notesSection)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}

		noteText := strings.TrimSpace(notesSection[start:end])
		if len(noteText) > 100 { // Skip very short fragments
			notes = append(notes, noteText)
		}
	}

	return notes
}

// extractSingleNote extracts structured data from a single note
func (e *NoteExtractor) extractSingleNote(ctx context.Context, noteText string, meta *FilingMetadata) (*ExtractedNote, error) {
	// Parse note number and title from first line
	noteNum, noteTitle := e.parseNoteHeader(noteText)

	// Categorize the note
	category := e.categorizeNote(noteTitle)

	// Detect and extract tables within the note
	tables := e.extractTablesFromNote(ctx, noteText, meta.FiscalYear)

	// Use LLM to extract structured data
	structuredData, err := e.llmExtractStructure(ctx, noteText, category)
	if err != nil {
		// Return note with just raw text if LLM fails
		return &ExtractedNote{
			NoteNumber:     noteNum,
			NoteTitle:      noteTitle,
			NoteCategory:   category,
			RawText:        truncateText(noteText, 50000),
			StructuredData: nil,
			SourceDoc:      meta.AccessionNumber,
			Tables:         tables,
		}, nil
	}

	return &ExtractedNote{
		NoteNumber:     noteNum,
		NoteTitle:      noteTitle,
		NoteCategory:   category,
		RawText:        truncateText(noteText, 50000),
		StructuredData: structuredData,
		SourceDoc:      meta.AccessionNumber,
		Tables:         tables,
	}, nil
}

// parseNoteHeader extracts note number and title from the header line
func (e *NoteExtractor) parseNoteHeader(noteText string) (string, string) {
	lines := strings.Split(noteText, "\n")
	if len(lines) == 0 {
		return "", ""
	}

	firstLine := strings.TrimSpace(lines[0])
	// Remove markdown headers
	firstLine = strings.TrimLeft(firstLine, "# ")

	// Pattern: "Note 1. Summary of Significant Accounting Policies"
	pattern := regexp.MustCompile(`(?i)(?:NOTE|Note)\s*(\d+[A-Z]?)[\.\:\s\—\-]+(.*)`)
	if matches := pattern.FindStringSubmatch(firstLine); len(matches) >= 3 {
		return "Note " + matches[1], strings.TrimSpace(matches[2])
	}

	return "", firstLine
}

// categorizeNote assigns a category based on note title (uses constants from note_index_agent.go)
func (e *NoteExtractor) categorizeNote(title string) string {
	titleLower := strings.ToLower(title)

	switch {
	case strings.Contains(titleLower, "segment"):
		return NoteCategorySegment
	case strings.Contains(titleLower, "revenue"):
		return NoteCategoryRevenue
	case strings.Contains(titleLower, "debt") || strings.Contains(titleLower, "borrowing"):
		return NoteCategoryDebt
	case strings.Contains(titleLower, "fair value"):
		return NoteCategoryFairValue
	case strings.Contains(titleLower, "commitment") || strings.Contains(titleLower, "contingenc"):
		return NoteCategoryContingencies
	case strings.Contains(titleLower, "accounting polic") || strings.Contains(titleLower, "significant"):
		return NoteCategoryAccountingPolicy
	case strings.Contains(titleLower, "tax") || strings.Contains(titleLower, "income tax"):
		return NoteCategoryIncomeTax
	case strings.Contains(titleLower, "equity") || strings.Contains(titleLower, "stock"):
		return NoteCategoryStockComp
	case strings.Contains(titleLower, "lease"):
		return NoteCategoryLeases
	default:
		return NoteCategoryOther
	}
}

// llmExtractStructure uses LLM to extract structured data from note
func (e *NoteExtractor) llmExtractStructure(ctx context.Context, noteText string, category string) (map[string]interface{}, error) {
	systemPrompt := `You are a Financial Data Extractor. Extract structured information from SEC filing notes.
Return a JSON object with relevant fields based on the note content.
Be concise. Extract numerical values, dates, and key terms.`

	userPrompt := fmt.Sprintf(`Extract structured data from this SEC filing note (Category: %s).

Return JSON with relevant fields. Examples:
- For SEGMENT: {"segments": [{"name": "...", "revenue": ..., "operating_income": ...}]}
- For DEBT: {"debt_instruments": [{"type": "...", "principal": ..., "maturity": "..."}]}
- For LEASES: {"operating_leases": ..., "finance_leases": ..., "total_lease_liability": ...}
- For others: Extract key numerical values and dates.

NOTE TEXT:
%s`, category, truncateText(noteText, 15000))

	resp, err := e.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var result map[string]interface{}
	cleaned := cleanJSONResponse(resp)
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return result, nil
}

// truncateText limits text to maxLen characters
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen]
}

// cleanJSONResponse extracts JSON from LLM response
func cleanJSONResponse(resp string) string {
	// Try to find JSON block
	start := strings.Index(resp, "{")
	end := strings.LastIndex(resp, "}")
	if start >= 0 && end > start {
		return resp[start : end+1]
	}
	return resp
}

// extractTablesFromNote detects and extracts tables from note text
func (e *NoteExtractor) extractTablesFromNote(ctx context.Context, noteText string, fiscalYear int) []NoteTable {
	// Detect markdown tables (lines with | separators)
	tableBlocks := e.detectMarkdownTables(noteText)
	if len(tableBlocks) == 0 {
		return nil
	}

	var tables []NoteTable
	for i, block := range tableBlocks {
		table := e.extractTableRows(ctx, block, i, fiscalYear)
		if len(table.Rows) > 0 {
			tables = append(tables, table)
		}
	}

	return tables
}

// detectMarkdownTables finds markdown table blocks in text
func (e *NoteExtractor) detectMarkdownTables(text string) []string {
	var tables []string
	lines := strings.Split(text, "\n")

	inTable := false
	var tableLines []string

	for _, line := range lines {
		// Check if line contains table structure (| character)
		if strings.Contains(line, "|") && strings.Count(line, "|") >= 2 {
			inTable = true
			tableLines = append(tableLines, line)
		} else if inTable {
			// End of table
			if len(tableLines) >= 3 { // At least header + separator + 1 data row
				tables = append(tables, strings.Join(tableLines, "\n"))
			}
			tableLines = nil
			inTable = false
		}
	}

	// Handle table at end of text
	if len(tableLines) >= 3 {
		tables = append(tables, strings.Join(tableLines, "\n"))
	}

	return tables
}

// extractTableRows uses LLM to extract structured rows from a table
func (e *NoteExtractor) extractTableRows(ctx context.Context, tableText string, tableIndex int, fiscalYear int) NoteTable {
	systemPrompt := `You are a Financial Table Parser. Extract rows from markdown tables as structured JSON.
Return JSON: {"title": "...", "rows": [{"label": "...", "values": [{"year": 2024, "value": 1234.5}, ...]}]}`

	userPrompt := fmt.Sprintf(`Parse this markdown table from a SEC filing (fiscal year %d):

%s

Return JSON with extracted rows. For each row, extract the label and all numeric values with their year columns.`, fiscalYear, tableText)

	resp, err := e.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return NoteTable{Index: tableIndex}
	}

	// Parse response
	var parsed struct {
		Title string `json:"title"`
		Rows  []struct {
			Label  string `json:"label"`
			Values []struct {
				Year  int      `json:"year"`
				Value *float64 `json:"value"`
			} `json:"values"`
		} `json:"rows"`
	}

	cleaned := cleanJSONResponse(resp)
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return NoteTable{Index: tableIndex}
	}

	// Convert to NoteTableRow format
	var rows []NoteTableRow
	for _, r := range parsed.Rows {
		for _, v := range r.Values {
			rows = append(rows, NoteTableRow{
				RowLabel:   r.Label,
				ColumnYear: v.Year,
				Value:      v.Value,
			})
		}
	}

	return NoteTable{
		Index: tableIndex,
		Title: parsed.Title,
		Rows:  rows,
	}
}
