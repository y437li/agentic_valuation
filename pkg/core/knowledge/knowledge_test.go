package knowledge

import (
	"testing"
)

func TestNewKnowledgeAsset(t *testing.T) {
	asset := NewKnowledgeAsset("Q3 Expert Call.pdf", AssetPDF, "/uploads/q3_call.pdf")

	if asset.Name != "Q3 Expert Call.pdf" {
		t.Errorf("expected name 'Q3 Expert Call.pdf', got '%s'", asset.Name)
	}
	if asset.Type != AssetPDF {
		t.Errorf("expected type PDF, got %s", asset.Type)
	}
	if asset.Status != StatusPending {
		t.Errorf("expected status PENDING, got %s", asset.Status)
	}
	if asset.IsIndexed {
		t.Error("new asset should not be indexed")
	}
}

func TestAssetMarkAsProcessed(t *testing.T) {
	asset := NewKnowledgeAsset("10-K.html", AssetSEC, "https://sec.gov/...")

	asset.MarkAsProcessed()

	if asset.Status != StatusIndexed {
		t.Errorf("expected status INDEXED, got %s", asset.Status)
	}
	if !asset.IsIndexed {
		t.Error("asset should be indexed after MarkAsProcessed")
	}
	if asset.ProcessedAt == nil {
		t.Error("ProcessedAt should be set")
	}
}

func TestAssetMarkAsError(t *testing.T) {
	asset := NewKnowledgeAsset("bad.pdf", AssetPDF, "/uploads/bad.pdf")

	asset.MarkAsError("parsing failed")

	if asset.Status != StatusError {
		t.Errorf("expected status ERROR, got %s", asset.Status)
	}
	if asset.Metadata["error"] != "parsing failed" {
		t.Errorf("expected error message in metadata")
	}
}

func TestAssetAddChunk(t *testing.T) {
	asset := NewKnowledgeAsset("10-K.html", AssetSEC, "https://sec.gov/...")

	chunk := Chunk{
		ID:        "chunk-1",
		Type:      ChunkTable,
		Content:   "| Revenue | 100M |",
		Section:   "Balance Sheet",
		StartLine: 100,
		EndLine:   110,
	}

	asset.AddChunk(chunk)

	if len(asset.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(asset.Chunks))
	}
	if asset.Chunks[0].AssetID != asset.ID {
		t.Error("chunk AssetID should be set to parent asset ID")
	}
}

func TestMemoryStore_CreateAndGet(t *testing.T) {
	store := NewMemoryStore()

	asset := NewKnowledgeAsset("test.pdf", AssetPDF, "/test.pdf")
	err := store.CreateAsset(asset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := store.GetAsset(asset.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.Name != asset.Name {
		t.Errorf("expected name '%s', got '%s'", asset.Name, retrieved.Name)
	}
}

func TestMemoryStore_CreateDuplicate(t *testing.T) {
	store := NewMemoryStore()

	asset := NewKnowledgeAsset("test.pdf", AssetPDF, "/test.pdf")
	_ = store.CreateAsset(asset)
	err := store.CreateAsset(asset)

	if err == nil {
		t.Fatal("expected error for duplicate asset, got nil")
	}
}

func TestMemoryStore_ListByType(t *testing.T) {
	store := NewMemoryStore()

	_ = store.CreateAsset(NewKnowledgeAsset("10-K.html", AssetSEC, "/sec/10k"))
	_ = store.CreateAsset(NewKnowledgeAsset("report.pdf", AssetPDF, "/pdf/report"))
	_ = store.CreateAsset(NewKnowledgeAsset("10-Q.html", AssetSEC, "/sec/10q"))

	secAssets, err := store.ListAssetsByType(AssetSEC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(secAssets) != 2 {
		t.Errorf("expected 2 SEC assets, got %d", len(secAssets))
	}
}

func TestMemoryStore_AddAndGetChunks(t *testing.T) {
	store := NewMemoryStore()

	asset := NewKnowledgeAsset("10-K.html", AssetSEC, "/sec/10k")
	_ = store.CreateAsset(asset)

	chunks := []Chunk{
		{ID: "c1", Content: "Revenue was $100M", Section: "MD&A"},
		{ID: "c2", Content: "Total Assets: $500M", Section: "Balance Sheet"},
	}

	err := store.AddChunks(asset.ID, chunks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, err := store.GetChunks(asset.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(retrieved))
	}
}

func TestMemoryStore_SearchChunks(t *testing.T) {
	store := NewMemoryStore()

	asset := NewKnowledgeAsset("10-K.html", AssetSEC, "/sec/10k")
	_ = store.CreateAsset(asset)

	chunks := []Chunk{
		{ID: "c1", Content: "Revenue increased by 15%", Section: "MD&A"},
		{ID: "c2", Content: "Operating expenses decreased", Section: "MD&A"},
		{ID: "c3", Content: "Revenue guidance for 2025", Section: "Outlook"},
	}
	_ = store.AddChunks(asset.ID, chunks)

	results, err := store.SearchChunks("revenue", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results containing 'revenue', got %d", len(results))
	}
}

func TestMemoryStore_DeleteAsset(t *testing.T) {
	store := NewMemoryStore()

	asset := NewKnowledgeAsset("test.pdf", AssetPDF, "/test.pdf")
	_ = store.CreateAsset(asset)

	chunks := []Chunk{{ID: "c1", Content: "test content"}}
	_ = store.AddChunks(asset.ID, chunks)

	err := store.DeleteAsset(asset.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify asset is deleted
	_, err = store.GetAsset(asset.ID)
	if err == nil {
		t.Error("expected error for deleted asset, got nil")
	}

	// Verify chunk is also deleted
	_, err = store.GetChunkByID("c1")
	if err == nil {
		t.Error("expected error for deleted chunk, got nil")
	}
}
