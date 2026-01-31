package knowledge

import (
	"fmt"
	"strings"
	"sync"
)

// =============================================================================
// IN-MEMORY STORE (For development/testing)
// Production should use Supabase with pgvector
// =============================================================================

// MemoryStore implements KnowledgeStore with in-memory storage
type MemoryStore struct {
	mu     sync.RWMutex
	assets map[string]*KnowledgeAsset
	chunks map[string]*Chunk // chunkID -> Chunk
}

// NewMemoryStore creates a new in-memory knowledge store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		assets: make(map[string]*KnowledgeAsset),
		chunks: make(map[string]*Chunk),
	}
}

// CreateAsset stores a new knowledge asset
func (s *MemoryStore) CreateAsset(asset *KnowledgeAsset) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.assets[asset.ID]; exists {
		return fmt.Errorf("asset '%s' already exists", asset.ID)
	}
	s.assets[asset.ID] = asset
	return nil
}

// GetAsset retrieves an asset by ID
func (s *MemoryStore) GetAsset(id string) (*KnowledgeAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	asset, ok := s.assets[id]
	if !ok {
		return nil, fmt.Errorf("asset '%s' not found", id)
	}
	return asset, nil
}

// UpdateAsset updates an existing asset
func (s *MemoryStore) UpdateAsset(asset *KnowledgeAsset) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.assets[asset.ID]; !exists {
		return fmt.Errorf("asset '%s' not found", asset.ID)
	}
	s.assets[asset.ID] = asset
	return nil
}

// DeleteAsset removes an asset and its chunks
func (s *MemoryStore) DeleteAsset(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	asset, ok := s.assets[id]
	if !ok {
		return fmt.Errorf("asset '%s' not found", id)
	}

	// Delete associated chunks
	for _, chunk := range asset.Chunks {
		delete(s.chunks, chunk.ID)
	}

	delete(s.assets, id)
	return nil
}

// ListAssetsByCase returns all assets for a case
func (s *MemoryStore) ListAssetsByCase(caseID string) ([]*KnowledgeAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*KnowledgeAsset
	for _, asset := range s.assets {
		if asset.CaseID != nil && *asset.CaseID == caseID {
			results = append(results, asset)
		}
	}
	return results, nil
}

// ListAssetsByCompany returns all assets for a company
func (s *MemoryStore) ListAssetsByCompany(companyID int64) ([]*KnowledgeAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*KnowledgeAsset
	for _, asset := range s.assets {
		if asset.CompanyID != nil && *asset.CompanyID == companyID {
			results = append(results, asset)
		}
	}
	return results, nil
}

// ListAssetsByType returns all assets of a specific type
func (s *MemoryStore) ListAssetsByType(assetType AssetType) ([]*KnowledgeAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*KnowledgeAsset
	for _, asset := range s.assets {
		if asset.Type == assetType {
			results = append(results, asset)
		}
	}
	return results, nil
}

// AddChunks adds chunks to an asset
func (s *MemoryStore) AddChunks(assetID string, chunks []Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	asset, ok := s.assets[assetID]
	if !ok {
		return fmt.Errorf("asset '%s' not found", assetID)
	}

	for i := range chunks {
		chunks[i].AssetID = assetID
		if chunks[i].ID == "" {
			chunks[i].ID = generateID()
		}
		s.chunks[chunks[i].ID] = &chunks[i]
		asset.Chunks = append(asset.Chunks, chunks[i])
	}
	return nil
}

// GetChunks returns all chunks for an asset
func (s *MemoryStore) GetChunks(assetID string) ([]Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	asset, ok := s.assets[assetID]
	if !ok {
		return nil, fmt.Errorf("asset '%s' not found", assetID)
	}
	return asset.Chunks, nil
}

// GetChunkByID returns a specific chunk
func (s *MemoryStore) GetChunkByID(chunkID string) (*Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chunk, ok := s.chunks[chunkID]
	if !ok {
		return nil, fmt.Errorf("chunk '%s' not found", chunkID)
	}
	return chunk, nil
}

// SearchChunks performs simple text search across chunks
func (s *MemoryStore) SearchChunks(query string, limit int) ([]Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []Chunk

	for _, chunk := range s.chunks {
		if strings.Contains(strings.ToLower(chunk.Content), queryLower) {
			results = append(results, *chunk)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// SearchChunksByEmbedding performs semantic search (placeholder for vector search)
// In production, this should use pgvector or similar
func (s *MemoryStore) SearchChunksByEmbedding(embedding []float64, limit int) ([]Chunk, error) {
	// Placeholder: In production, use cosine similarity search with pgvector
	// For now, return first N chunks
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Chunk
	for _, chunk := range s.chunks {
		results = append(results, *chunk)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}
