package edgar

import (
	"testing"
)

func TestGoExtractor_UnitScaling(t *testing.T) {
	extractor := NewGoExtractor()

	markdown := `
(In millions)
| Item | 2024 |
|---|---|
| Revenue | 100 |
`
	// Manually construct ParsedTable (simulating parsing)
	// IMPORTANT: RawContent MUST be set for scaling to work
	table := extractor.ParseMarkdownTable(markdown, "income_statement")
	if table.RawContent == "" {
		t.Fatal("ParseMarkdownTable failed to set RawContent")
	}

	mapping := &LineItemMapping{
		RowMappings: []RowMapping{
			{RowIndex: 0, RowLabel: "Revenue", FSAPVariable: "revenues", Confidence: 1.0},
		},
		YearColumns: []YearColumn{
			{Year: 2024, ColumnIndex: 1}, // 1-based index
		},
	}

	values := extractor.ExtractValues(table, mapping)

	if len(values) != 1 {
		t.Fatalf("Expected 1 value, got %d", len(values))
	}

	val := values[0].Years["2024"]
	expected := 100.0 // Value should NOT be scaled per user request

	if val != expected {
		t.Errorf("Expected raw value %.0f, got %.0f", expected, val)
	}

	// Verify metadata
	if values[0].Provenance.Scale != "millions" {
		t.Errorf("Expected Scale 'millions', got '%s'", values[0].Provenance.Scale)
	}
}
