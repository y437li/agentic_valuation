package edgar

import (
	"context"
	"fmt"
)

// QuantitativeSegmentAgent extracts structured financial data from Segment Notes
type QuantitativeSegmentAgent struct {
	provider AIProvider
}

func NewQuantitativeSegmentAgent(provider AIProvider) *QuantitativeSegmentAgent {
	return &QuantitativeSegmentAgent{provider: provider}
}

// AnalyzeSegments extracts revenue and income by segment from the Notes section
func (a *QuantitativeSegmentAgent) AnalyzeSegments(ctx context.Context, notesText string) (*SegmentAnalysis, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}

	// Truncate to avoid context limit, but usually Notes are huge.
	// Ideally we just pass the metadata and let a 'Seeker' find it, but for now we search chunks?
	// Or relies on 'Notes' extracted by Navigator being reasonable size?
	// Navigator extracted all notes.
	// Strategy: Use a "Locator" step first.

	// Step 1: Locate "Segment Information" Note
	segmentNoteText, err := a.locateSegmentNote(ctx, notesText)
	if err != nil {
		return nil, err
	}
	if len(segmentNoteText) < 100 {
		return nil, fmt.Errorf("segment note not found or empty")
	}

	// Step 2: Extract Data
	return a.extractSegmentData(ctx, segmentNoteText)
}

func (a *QuantitativeSegmentAgent) locateSegmentNote(ctx context.Context, fullNotes string) (string, error) {
	// Simple heuristic: If text is small enough, just use it.
	if len(fullNotes) < 30000 {
		return fullNotes, nil
	}

	// Otherwise, ask LLM to identify the start/end or just extract the relevant chunk.
	systemPrompt := "You are a Financial Data Locator. Find the 'Segment Information' or 'Business Segments' note."
	userPrompt := fmt.Sprintf(`Analyze the following Notes text and EXTRACT ONLY the specific Note related to Segment Information.
It usually contains tables for "Net Sales by Segment" and "Operating Income by Segment".
Return the full text of that Note only.

Text (first 50k chars):
%s`, truncate(fullNotes, 50000))

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Cleanup response? LLM might chat.
	// Assuming LLM follows instruction to return just text.
	return resp, nil
}

func (a *QuantitativeSegmentAgent) extractSegmentData(ctx context.Context, noteText string) (*SegmentAnalysis, error) {
	systemPrompt := `You are an expert at extracting Segment Information tables.
Extract quantitative data for each Reporting Segment.

TARGET DATA:
1. Revenues (Net Sales)
2. Operating Income (EBIT)
3. Total Assets (if available)

OUTPUT FORMAT (JSON):
{
  "segments": [
    {
      "name": "Exact Segment Name",
      "standardized_type": "Product" | "Service" | "Geo",
      "revenues": { "value": 12345, "years": {"2024": 12345, "2023": 11000} },
      "operating_income": { "value": 4567, "years": {"2024": 4567, "2023": 4000} },
      "assets": { "value": 99999 } 
    }
  ],
  "geographic_breakdown": [
     { "region": "Americas", "revenues": { "value": 50000, "years": {...} } }
  ]
}

RULES:
- Extract values in MILLIONS (check table units).
- Include ALL years shown.
- Do NOT halllucinate data. If Operating Income is not shown by segment, leave null.
`
	userPrompt := fmt.Sprintf("Extract segment data from this text:\n\n%s", truncate(noteText, 15000))

	resp, err := a.provider.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var result SegmentAnalysis
	if err := parseJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse segment JSON: %w", err)
	}
	return &result, nil
}
