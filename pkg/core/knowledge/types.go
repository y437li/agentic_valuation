// Package knowledge implements the Unified Knowledge Layer for multi-source data.
// Supports SEC filings, local PDFs, and web content as knowledge assets.
// Enables RAG (Retrieval Augmented Generation) for AI agents.
package knowledge

import (
	"fmt"
	"time"
)

// =============================================================================
// ASSET TYPES
// =============================================================================

// AssetType defines the category of a knowledge asset
type AssetType string

const (
	AssetSEC      AssetType = "SEC"      // 10-K, 10-Q, 8-K filings
	AssetPDF      AssetType = "PDF"      // Local uploaded PDFs (research reports, expert calls)
	AssetWeb      AssetType = "WEB"      // Ingested web pages
	AssetExcel    AssetType = "EXCEL"    // Excel models
	AssetEarnings AssetType = "EARNINGS" // Earnings call transcripts
)

// AssetStatus tracks the processing state of an asset
type AssetStatus string

const (
	StatusPending    AssetStatus = "PENDING"    // Awaiting processing
	StatusProcessing AssetStatus = "PROCESSING" // Currently being parsed
	StatusIndexed    AssetStatus = "INDEXED"    // Successfully indexed for RAG
	StatusError      AssetStatus = "ERROR"      // Processing failed
)

// =============================================================================
// KNOWLEDGE ASSET
// =============================================================================

// KnowledgeAsset represents a unified document in the knowledge base
// This is the core entity for the "Supporting Material" concept
type KnowledgeAsset struct {
	ID     string      `json:"id"`
	Type   AssetType   `json:"type"`
	Name   string      `json:"name"`   // e.g., "Q3 2024 Expert Call.pdf"
	Source string      `json:"source"` // URL or local path
	Status AssetStatus `json:"status"`

	// Ownership
	CaseID    *string `json:"case_id,omitempty"`    // Optional link to case
	CompanyID *int64  `json:"company_id,omitempty"` // Optional link to company
	UserID    *string `json:"user_id,omitempty"`    // Who uploaded it

	// SEC-specific metadata
	SECMetadata *SECAssetMetadata `json:"sec_metadata,omitempty"`

	// Parsed content (for RAG)
	Chunks []Chunk `json:"chunks,omitempty"`

	// Index metadata
	IsIndexed   bool   `json:"is_indexed"`
	EmbeddingID string `json:"embedding_id,omitempty"` // Vector store reference

	// Timestamps
	UploadedAt  time.Time  `json:"uploaded_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Flexible metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SECAssetMetadata contains SEC-specific filing information
type SECAssetMetadata struct {
	CIK             string `json:"cik"`
	AccessionNumber string `json:"accession_number"`
	FormType        string `json:"form_type"` // "10-K", "10-Q", "8-K"
	FilingDate      string `json:"filing_date"`
	FiscalYear      int    `json:"fiscal_year"`
	FiscalPeriod    string `json:"fiscal_period"` // "FY", "Q1", "Q2", "Q3"
	IsAmended       bool   `json:"is_amended"`
}

// =============================================================================
// CHUNK (Semantic Unit for RAG)
// =============================================================================

// ChunkType identifies the content type of a chunk
type ChunkType string

const (
	ChunkParagraph ChunkType = "PARAGRAPH" // Regular text
	ChunkTable     ChunkType = "TABLE"     // Financial table
	ChunkHeader    ChunkType = "HEADER"    // Section header
	ChunkFootnote  ChunkType = "FOOTNOTE"  // Footnote or note reference
)

// Chunk represents a semantic unit within an asset
// Used for RAG retrieval and citation
type Chunk struct {
	ID      string    `json:"id"`
	AssetID string    `json:"asset_id"` // Parent asset reference
	Type    ChunkType `json:"type"`

	// Content
	Content string `json:"content"`            // Markdown text
	RawHTML string `json:"raw_html,omitempty"` // Original HTML if applicable

	// Position in source document
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	PageRef   string `json:"page_ref,omitempty"` // e.g., "F-22"

	// Section context
	Section    string `json:"section"` // e.g., "MD&A", "Risk Factors", "Balance Sheet"
	Subsection string `json:"subsection,omitempty"`

	// Table-specific metadata
	TableID       string   `json:"table_id,omitempty"` // Fingerprint of table
	ColumnHeaders []string `json:"column_headers,omitempty"`

	// Embedding (not serialized to JSON for API responses)
	Embedding []float64 `json:"-"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
}

// =============================================================================
// KNOWLEDGE STORE INTERFACE
// =============================================================================

// KnowledgeStore defines the interface for asset storage and retrieval
type KnowledgeStore interface {
	// Asset CRUD
	CreateAsset(asset *KnowledgeAsset) error
	GetAsset(id string) (*KnowledgeAsset, error)
	UpdateAsset(asset *KnowledgeAsset) error
	DeleteAsset(id string) error

	// Query assets
	ListAssetsByCase(caseID string) ([]*KnowledgeAsset, error)
	ListAssetsByCompany(companyID int64) ([]*KnowledgeAsset, error)
	ListAssetsByType(assetType AssetType) ([]*KnowledgeAsset, error)

	// Chunk operations
	AddChunks(assetID string, chunks []Chunk) error
	GetChunks(assetID string) ([]Chunk, error)
	GetChunkByID(chunkID string) (*Chunk, error)

	// Search (for RAG)
	SearchChunks(query string, limit int) ([]Chunk, error)
	SearchChunksByEmbedding(embedding []float64, limit int) ([]Chunk, error)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// NewKnowledgeAsset creates a new asset with default values
func NewKnowledgeAsset(name string, assetType AssetType, source string) *KnowledgeAsset {
	now := time.Now()
	return &KnowledgeAsset{
		ID:         generateID(), // Implement with UUID
		Type:       assetType,
		Name:       name,
		Source:     source,
		Status:     StatusPending,
		UploadedAt: now,
		UpdatedAt:  now,
		Metadata:   make(map[string]interface{}),
	}
}

// generateID creates a unique identifier
// Uses atomic counter for test reliability, production should use UUID
var idCounter int64 = 0

func generateID() string {
	idCounter++
	return fmt.Sprintf("%s-%d", time.Now().Format("20060102150405"), idCounter)
}

// MarkAsProcessed updates asset status after successful processing
func (a *KnowledgeAsset) MarkAsProcessed() {
	now := time.Now()
	a.Status = StatusIndexed
	a.ProcessedAt = &now
	a.UpdatedAt = now
	a.IsIndexed = true
}

// MarkAsError updates asset status after processing failure
func (a *KnowledgeAsset) MarkAsError(errMsg string) {
	a.Status = StatusError
	a.UpdatedAt = time.Now()
	if a.Metadata == nil {
		a.Metadata = make(map[string]interface{})
	}
	a.Metadata["error"] = errMsg
}

// AddChunk appends a chunk to the asset
func (a *KnowledgeAsset) AddChunk(chunk Chunk) {
	chunk.AssetID = a.ID
	chunk.CreatedAt = time.Now()
	a.Chunks = append(a.Chunks, chunk)
}
