package services

import (
	"fmt"
	"sort"
	"sync"
)

// MockVectorStore implements VectorStoreInterface for testing
type MockVectorStore struct {
	vectors map[string]*VectorEntry
	mutex   sync.RWMutex
}

// NewMockVectorStore creates a new mock vector store for testing
func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{
		vectors: make(map[string]*VectorEntry),
	}
}

// StoreVector stores a vector entry
func (m *MockVectorStore) StoreVector(entry *VectorEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Make a copy to avoid reference issues
	copyEntry := &VectorEntry{
		ID:          entry.ID,
		DocumentID:  entry.DocumentID,
		ChunkID:     entry.ChunkID,
		Vector:      make([]float32, len(entry.Vector)),
		Dimension:   entry.Dimension,
		Content:     entry.Content,
		Metadata:    entry.Metadata,
		CreatedAt:   entry.CreatedAt,
	}
	copy(copyEntry.Vector, entry.Vector)

	m.vectors[entry.ID] = copyEntry
	return nil
}

// SearchSimilar searches for similar vectors using cosine similarity
func (m *MockVectorStore) SearchSimilar(queryVector []float32, topK int) ([]*SearchResult, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var results []*SearchResult

	for _, entry := range m.vectors {
		// Calculate cosine similarity
		score := CosineSimilarity(queryVector, entry.Vector)

		result := &SearchResult{
			Entry: entry,
			Score: score,
		}

		results = append(results, result)
	}

	// Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// GetVector retrieves a vector by ID
func (m *MockVectorStore) GetVector(id string) (*VectorEntry, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	entry, exists := m.vectors[id]
	if !exists {
		return nil, nil
	}

	return entry, nil
}

// DeleteVector deletes a vector by ID
func (m *MockVectorStore) DeleteVector(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.vectors, id)
	return nil
}

// DeleteDocumentVectors deletes all vectors for a document
func (m *MockVectorStore) DeleteDocumentVectors(documentID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for id, entry := range m.vectors {
		if entry.DocumentID == documentID {
			delete(m.vectors, id)
		}
	}

	return nil
}

// GetDocumentVectors retrieves all vectors for a document
func (m *MockVectorStore) GetDocumentVectors(documentID string) ([]*VectorEntry, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var vectors []*VectorEntry
	for _, entry := range m.vectors {
		if entry.DocumentID == documentID {
			vectors = append(vectors, entry)
		}
	}

	// Sort by created time
	sort.Slice(vectors, func(i, j int) bool {
		return vectors[i].CreatedAt.Before(vectors[j].CreatedAt)
	})

	return vectors, nil
}

// Clear removes all vectors from the store
func (m *MockVectorStore) Clear() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.vectors = make(map[string]*VectorEntry)
	return nil
}

// StoreBatch stores multiple vector entries in a batch
func (m *MockVectorStore) StoreBatch(entries []*VectorEntry) error {
	for _, entry := range entries {
		if err := m.StoreVector(entry); err != nil {
			return fmt.Errorf("failed to store vector %s in batch: %v", entry.ID, err)
		}
	}
	return nil
}

// SearchWithFilter searches for similar vectors with optional filtering
func (m *MockVectorStore) SearchWithFilter(queryVector []float32, topK int, filter map[string]interface{}) ([]*SearchResult, error) {
	// For mock, ignore filters and delegate to regular search
	return m.SearchSimilar(queryVector, topK)
}

// HealthCheck checks if the mock store is working
func (m *MockVectorStore) HealthCheck() error {
	return nil // Mock store is always healthy
}

