package store

import (
	"context"
	"encoding/json"
	"fmt"

	"agentic_valuation/pkg/core/edgar"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NotesRepo provides storage for extracted SEC filing notes
type NotesRepo struct {
	pool *pgxpool.Pool
}

// NewNotesRepo creates a new notes repository
func NewNotesRepo(pool *pgxpool.Pool) *NotesRepo {
	return &NotesRepo{pool: pool}
}

// SaveNote stores an extracted note to Supabase
func (r *NotesRepo) SaveNote(ctx context.Context, note *edgar.ExtractedNote, meta *edgar.FilingMetadata) error {
	if r.pool == nil {
		return fmt.Errorf("database pool not configured")
	}

	// Serialize structured data to JSONB
	var structuredJSON []byte
	if note.StructuredData != nil {
		var err error
		structuredJSON, err = json.Marshal(note.StructuredData)
		if err != nil {
			return fmt.Errorf("failed to marshal structured data: %w", err)
		}
	}

	// Get ticker
	ticker := ""
	if len(meta.Tickers) > 0 {
		ticker = meta.Tickers[0]
	}

	// Insert note and get ID for table rows
	query := `
		INSERT INTO sec_filing_notes (
			cik, ticker, accession_number, fiscal_year, fiscal_period,
			note_number, note_title, note_category,
			raw_text, structured_data, llm_provider
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (accession_number, note_number) 
		DO UPDATE SET 
			note_title = EXCLUDED.note_title,
			note_category = EXCLUDED.note_category,
			raw_text = EXCLUDED.raw_text,
			structured_data = EXCLUDED.structured_data,
			updated_at = NOW()
		RETURNING id
	`

	var noteID string
	err := r.pool.QueryRow(ctx, query,
		meta.CIK, ticker, meta.AccessionNumber, meta.FiscalYear, meta.FiscalPeriod,
		note.NoteNumber, note.NoteTitle, note.NoteCategory,
		note.RawText, structuredJSON, "gemini",
	).Scan(&noteID)
	if err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	// Save table rows if present
	if len(note.Tables) > 0 {
		if err := r.saveNoteTableRows(ctx, noteID, note.Tables); err != nil {
			fmt.Printf("  Warning: failed to save table rows for %s: %v\n", note.NoteNumber, err)
		}
	}

	return nil
}

// saveNoteTableRows saves normalized table rows to sec_note_tables
func (r *NotesRepo) saveNoteTableRows(ctx context.Context, noteID string, tables []edgar.NoteTable) error {
	// Delete existing rows for this note (upsert at note level)
	_, err := r.pool.Exec(ctx, "DELETE FROM sec_note_tables WHERE note_id = $1", noteID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO sec_note_tables (
			note_id, table_index, table_title, row_label, mapped_field,
			column_year, column_period, value, value_text
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	for _, table := range tables {
		for _, row := range table.Rows {
			_, err := r.pool.Exec(ctx, query,
				noteID, table.Index, table.Title, row.RowLabel, row.MappedField,
				row.ColumnYear, row.ColumnPeriod, row.Value, row.ValueText,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SaveAllNotes saves multiple notes in a transaction
func (r *NotesRepo) SaveAllNotes(ctx context.Context, notes []*edgar.ExtractedNote, meta *edgar.FilingMetadata) error {
	for _, note := range notes {
		if err := r.SaveNote(ctx, note, meta); err != nil {
			fmt.Printf("  Warning: failed to save %s: %v\n", note.NoteNumber, err)
			// Continue with other notes
		}
	}
	return nil
}

// GetNotesByFiling retrieves all notes for a specific filing
func (r *NotesRepo) GetNotesByFiling(ctx context.Context, accessionNumber string) ([]*edgar.ExtractedNote, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("database pool not configured")
	}

	query := `
		SELECT note_number, note_title, note_category, raw_text, structured_data
		FROM sec_filing_notes
		WHERE accession_number = $1
		ORDER BY note_number
	`

	rows, err := r.pool.Query(ctx, query, accessionNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	var notes []*edgar.ExtractedNote
	for rows.Next() {
		var note edgar.ExtractedNote
		var structuredJSON []byte

		if err := rows.Scan(&note.NoteNumber, &note.NoteTitle, &note.NoteCategory, &note.RawText, &structuredJSON); err != nil {
			return nil, fmt.Errorf("failed to scan note row: %w", err)
		}

		if len(structuredJSON) > 0 {
			json.Unmarshal(structuredJSON, &note.StructuredData)
		}
		note.SourceDoc = accessionNumber
		notes = append(notes, &note)
	}

	return notes, nil
}

// GetNotesByCategory retrieves notes of a specific category for a company
func (r *NotesRepo) GetNotesByCategory(ctx context.Context, cik string, category string) ([]*edgar.ExtractedNote, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("database pool not configured")
	}

	query := `
		SELECT accession_number, note_number, note_title, note_category, raw_text, structured_data, fiscal_year
		FROM sec_filing_notes
		WHERE cik = $1 AND note_category = $2
		ORDER BY fiscal_year DESC
	`

	rows, err := r.pool.Query(ctx, query, cik, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes by category: %w", err)
	}
	defer rows.Close()

	var notes []*edgar.ExtractedNote
	for rows.Next() {
		var note edgar.ExtractedNote
		var structuredJSON []byte
		var fiscalYear int

		if err := rows.Scan(&note.SourceDoc, &note.NoteNumber, &note.NoteTitle, &note.NoteCategory, &note.RawText, &structuredJSON, &fiscalYear); err != nil {
			return nil, fmt.Errorf("failed to scan note row: %w", err)
		}

		if len(structuredJSON) > 0 {
			json.Unmarshal(structuredJSON, &note.StructuredData)
		}
		notes = append(notes, &note)
	}

	return notes, nil
}

// NoteExists checks if a note already exists
func (r *NotesRepo) NoteExists(ctx context.Context, accessionNumber, noteNumber string) bool {
	if r.pool == nil {
		return false
	}

	query := `SELECT 1 FROM sec_filing_notes WHERE accession_number = $1 AND note_number = $2 LIMIT 1`
	var exists int
	err := r.pool.QueryRow(ctx, query, accessionNumber, noteNumber).Scan(&exists)
	return err == nil
}
