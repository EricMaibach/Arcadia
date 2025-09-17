package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// QDRantVectorStore implements VectorStoreInterface using QDRant
type QDRantVectorStore struct {
	client     *qdrant.Client
	collection string
	config     QDRantConfig
	mutex      sync.RWMutex
}

// QDRantPayload represents the payload structure for QDRant points
// Uses custom JSON marshaling to handle any type for chunk_id field
type QDRantPayload struct {
	DocumentID string  `json:"document_id"`
	ChunkID    *string `json:"-"`                    // Handle manually
	Content    string  `json:"content"`
	Metadata   string  `json:"metadata,omitempty"`
	CreatedAt  int64   `json:"created_at"`
}

// UnmarshalJSON implements custom JSON unmarshaling for QDRantPayload
func (p *QDRantPayload) UnmarshalJSON(data []byte) error {
	// First unmarshal into a map to handle chunk_id safely
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal payload to map: %v", err)
	}

	// Extract document_id - handle both string and nested map structure
	if docIDRaw, exists := raw["document_id"]; exists {
		switch v := docIDRaw.(type) {
		case string:
			p.DocumentID = v
		default:
			// Try to extract from nested map structure
			if extracted := extractStringFromNestedMap(v); extracted != "" {
				p.DocumentID = extracted
			} else {
				// Fallback to string representation
				p.DocumentID = fmt.Sprintf("%v", v)
			}
		}
	}

	// Extract content - handle both string and nested map structure
	if contentRaw, exists := raw["content"]; exists {
		switch v := contentRaw.(type) {
		case string:
			p.Content = v
		default:
			// Try to extract from nested map structure
			if extracted := extractStringFromNestedMap(v); extracted != "" {
				p.Content = extracted
			} else {
				// Fallback to string representation
				p.Content = fmt.Sprintf("%v", v)
			}
		}
	}

	// Extract metadata - handle both string and nested map structure
	if metadataRaw, exists := raw["metadata"]; exists {
		switch v := metadataRaw.(type) {
		case string:
			p.Metadata = v
		default:
			// Try to extract from nested map structure
			if extracted := extractStringFromNestedMap(v); extracted != "" {
				p.Metadata = extracted
			} else {
				// Fallback to string representation
				p.Metadata = fmt.Sprintf("%v", v)
			}
		}
	}

	// Extract created_at
	if createdAt, ok := raw["created_at"]; ok {
		switch v := createdAt.(type) {
		case float64:
			p.CreatedAt = int64(v)
		case int64:
			p.CreatedAt = v
		case int:
			p.CreatedAt = int64(v)
		}
	}

	// Handle chunk_id flexibly - it can be null, string, or any other type
	if chunkIDRaw, exists := raw["chunk_id"]; exists {
		switch v := chunkIDRaw.(type) {
		case nil:
			// chunk_id is null, leave p.ChunkID as nil
			p.ChunkID = nil
		case string:
			// chunk_id is a string, use it
			p.ChunkID = &v
		default:
			// chunk_id is some other type (object, number, etc.)
			// Try to extract from nested map structure: map[Kind:map[StringValue:actual_value]]
			if extracted := extractStringFromNestedMap(v); extracted != "" {
				p.ChunkID = &extracted
			} else {
				// Fallback to string representation
				log.Printf("[QDRant] Warning: chunk_id has unexpected type %T, converting to string: %v", v, v)
				strValue := fmt.Sprintf("%v", v)
				p.ChunkID = &strValue
			}
		}
	} else {
		// chunk_id field doesn't exist, leave as nil
		p.ChunkID = nil
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling for QDRantPayload
func (p *QDRantPayload) MarshalJSON() ([]byte, error) {
	// Create a map for marshaling
	result := map[string]interface{}{
		"document_id": p.DocumentID,
		"content":     p.Content,
		"created_at":  p.CreatedAt,
	}

	// Add metadata if not empty
	if p.Metadata != "" {
		result["metadata"] = p.Metadata
	}

	// Add chunk_id if not nil
	if p.ChunkID != nil {
		result["chunk_id"] = *p.ChunkID
	}

	return json.Marshal(result)
}

// NewQDRantVectorStore creates a new QDRant vector store
func NewQDRantVectorStore(config QDRantConfig) (VectorStoreInterface, error) {
	store := &QDRantVectorStore{
		config:     config,
		collection: config.Collection,
	}

	// Initialize QDRant client connection
	if err := store.initializeClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize QDRant client: %v", err)
	}

	// Initialize collection if it doesn't exist
	if err := store.initializeCollection(); err != nil {
		return nil, fmt.Errorf("failed to initialize QDRant collection: %v", err)
	}

	return store, nil
}

// initializeClient initializes the QDRant client connection
func (q *QDRantVectorStore) initializeClient() error {
	// Build connection URL
	scheme := "http"
	if q.config.UseHTTPS {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s:%d", scheme, q.config.Host, q.config.Port)

	// Create client configuration - simplified for basic connection
	config := &qdrant.Config{
		Host:   q.config.Host,
		Port:   q.config.Port,
		APIKey: q.config.APIKey,
		UseTLS: q.config.UseHTTPS,
	}

	// Create client
	client, err := qdrant.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create QDRant client for %s: %v", url, err)
	}

	q.client = client
	log.Printf("[QDRant] Connected to QDRant at %s", url)
	return nil
}

// initializeCollection creates the collection if it doesn't exist
func (q *QDRantVectorStore) initializeCollection() error {
	return q.ensureCollection()
}

// ensureCollection ensures the collection exists, creating it if necessary
func (q *QDRantVectorStore) ensureCollection() error {
	ctx := context.Background()

	// Check if collection exists
	collectionInfo, err := q.client.GetCollectionInfo(ctx, q.collection)

	if err != nil {
		// Collection doesn't exist, create it
		log.Printf("[QDRant] Creating collection '%s'", q.collection)

		err = q.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: q.collection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     768, // EmbeddingGemma dimension
				Distance: qdrant.Distance_Cosine,
			}),
		})

		if err != nil {
			return fmt.Errorf("failed to create collection '%s': %v", q.collection, err)
		}

		log.Printf("[QDRant] Successfully created collection '%s'", q.collection)
		return nil
	}

	// Collection exists, verify it has the correct configuration
	if collectionInfo != nil {
		log.Printf("[QDRant] Collection '%s' already exists", q.collection)
	}

	return nil
}

// HealthCheck checks if QDRant is accessible and responsive
func (q *QDRantVectorStore) HealthCheck() error {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if q.client == nil {
		return fmt.Errorf("QDRant client not initialized")
	}

	ctx := context.Background()

	// Check if we can get collection info
	collectionInfo, err := q.client.GetCollectionInfo(ctx, q.collection)

	if err != nil {
		return fmt.Errorf("failed to get collection info for '%s': %v", q.collection, err)
	}

	if collectionInfo == nil {
		return fmt.Errorf("collection '%s' not found", q.collection)
	}

	log.Printf("[QDRant] Health check passed for collection '%s'", q.collection)
	return nil
}

// StoreVector stores a single vector entry in QDRant
func (q *QDRantVectorStore) StoreVector(entry *VectorEntry) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if entry == nil {
		return fmt.Errorf("entry is nil")
	}

	ctx := context.Background()

	// Create payload
	payload := &QDRantPayload{
		DocumentID: entry.DocumentID,
		ChunkID:    &entry.ChunkID,
		Content:    entry.Content,
		Metadata:   entry.Metadata,
		CreatedAt:  entry.CreatedAt.Unix(),
	}

	// Convert payload to map
	payloadMap, err := structToMap(payload)
	if err != nil {
		return fmt.Errorf("failed to convert payload to map: %v", err)
	}

	// Create point
	point := &qdrant.PointStruct{
		Id:      qdrant.NewIDUUID(entry.ID),
		Vectors: qdrant.NewVectors(entry.Vector...),
		Payload: qdrant.NewValueMap(payloadMap),
	}

	// Upsert the point
	_, err = q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: q.collection,
		Points:         []*qdrant.PointStruct{point},
	})

	if err != nil {
		return fmt.Errorf("failed to upsert point %s: %v", entry.ID, err)
	}

	log.Printf("[QDRant] Successfully stored vector %s", entry.ID)
	return nil
}

// StoreBatch stores multiple vector entries in a single batch operation
func (q *QDRantVectorStore) StoreBatch(entries []*VectorEntry) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(entries) == 0 {
		return nil // Nothing to store
	}

	ctx := context.Background()

	// Convert all entries to QDRant points
	points := make([]*qdrant.PointStruct, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}

		// Create payload
		payload := &QDRantPayload{
			DocumentID: entry.DocumentID,
			ChunkID:    &entry.ChunkID,
			Content:    entry.Content,
			Metadata:   entry.Metadata,
			CreatedAt:  entry.CreatedAt.Unix(),
		}

		// Convert payload to map
		payloadMap, err := structToMap(payload)
		if err != nil {
			return fmt.Errorf("failed to convert payload to map for entry %s: %v", entry.ID, err)
		}

		// Create point
		point := &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(entry.ID),
			Vectors: qdrant.NewVectors(entry.Vector...),
			Payload: qdrant.NewValueMap(payloadMap),
		}

		points = append(points, point)
	}

	if len(points) == 0 {
		return nil // No valid entries to store
	}

	// Batch upsert all points
	_, err := q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: q.collection,
		Points:         points,
	})

	if err != nil {
		return fmt.Errorf("failed to batch upsert %d points: %v", len(points), err)
	}

	log.Printf("[QDRant] Successfully stored batch of %d vectors", len(points))
	return nil
}

// SearchSimilar searches for similar vectors using cosine similarity
func (q *QDRantVectorStore) SearchSimilar(queryVector []float32, topK int) ([]*SearchResult, error) {
	return q.SearchWithFilter(queryVector, topK, nil)
}

// SearchWithFilter searches for similar vectors with optional filtering
func (q *QDRantVectorStore) SearchWithFilter(queryVector []float32, topK int, filter map[string]interface{}) ([]*SearchResult, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}

	if topK <= 0 {
		topK = 10 // Default to 10 results
	}

	ctx := context.Background()

	// Build the query request
	queryRequest := &qdrant.QueryPoints{
		CollectionName: q.collection,
		Query:          qdrant.NewQuery(queryVector...),
		Limit:          qdrant.PtrOf(uint64(topK)),
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(true),
	}

	// Add filter if provided
	if filter != nil && len(filter) > 0 {
		conditions := make([]*qdrant.Condition, 0, len(filter))
		for key, value := range filter {
			switch v := value.(type) {
			case string:
				conditions = append(conditions, qdrant.NewMatch(key, v))
			default:
				conditions = append(conditions, qdrant.NewMatch(key, fmt.Sprintf("%v", v)))
			}
		}
		queryRequest.Filter = &qdrant.Filter{
			Must: conditions,
		}
	}

	// Execute the query
	response, err := q.client.Query(ctx, queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %v", err)
	}

	// Convert results to SearchResult format
	results := make([]*SearchResult, 0, len(response))
	for i, point := range response {
		// Convert ScoredPoint to RetrievedPoint for our conversion function
		retrievedPoint := &qdrant.RetrievedPoint{
			Id:      point.Id,
			Payload: point.Payload,
			Vectors: point.Vectors,
		}

		entry, err := q.convertPointToVectorEntry(retrievedPoint)
		if err != nil {
			log.Printf("[QDRant] Warning: failed to convert point %d: %v", i, err)
			continue
		}

		results = append(results, &SearchResult{
			Entry:      entry,
			Score:      point.Score,
			ChunkIndex: i, // Use position in results as chunk index
		})
	}

	log.Printf("[QDRant] Found %d similar vectors", len(results))
	return results, nil
}

// GetVector retrieves a vector by ID
func (q *QDRantVectorStore) GetVector(id string) (*VectorEntry, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if id == "" {
		return nil, fmt.Errorf("id is empty")
	}

	ctx := context.Background()

	// Get the point by ID
	response, err := q.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: q.collection,
		Ids:            []*qdrant.PointId{qdrant.NewIDUUID(id)},
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(true),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get vector %s: %v", id, err)
	}

	if len(response) == 0 {
		return nil, fmt.Errorf("vector with ID %s not found", id)
	}

	// Convert the point to VectorEntry
	entry, err := q.convertPointToVectorEntry(response[0])
	if err != nil {
		return nil, fmt.Errorf("failed to convert point to vector entry: %v", err)
	}

	return entry, nil
}

// DeleteVector deletes a vector by ID
func (q *QDRantVectorStore) DeleteVector(id string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if id == "" {
		return fmt.Errorf("id is empty")
	}

	ctx := context.Background()

	// Delete the point by ID
	_, err := q.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: q.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: []*qdrant.PointId{qdrant.NewIDUUID(id)},
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete vector %s: %v", id, err)
	}

	log.Printf("[QDRant] Successfully deleted vector %s", id)
	return nil
}

// DeleteDocumentVectors deletes all vectors for a document
func (q *QDRantVectorStore) DeleteDocumentVectors(documentID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if documentID == "" {
		return fmt.Errorf("documentID is empty")
	}

	ctx := context.Background()

	// Delete all points with matching document_id in payload
	_, err := q.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: q.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Must: []*qdrant.Condition{
						qdrant.NewMatch("document_id", documentID),
					},
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete document vectors for %s: %v", documentID, err)
	}

	log.Printf("[QDRant] Successfully deleted all vectors for document %s", documentID)
	return nil
}

// GetDocumentVectors retrieves all vectors for a document
func (q *QDRantVectorStore) GetDocumentVectors(documentID string) ([]*VectorEntry, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if documentID == "" {
		return nil, fmt.Errorf("documentID is empty")
	}

	ctx := context.Background()

	// Use scroll to get all points matching the document_id filter
	// We use a large limit and scroll through if needed
	response, err := q.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: q.collection,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("document_id", documentID),
			},
		},
		Limit:       qdrant.PtrOf(uint32(1000)), // Large limit to get all vectors for a document
		WithPayload: qdrant.NewWithPayload(true),
		WithVectors: qdrant.NewWithVectors(true),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get document vectors for %s: %v", documentID, err)
	}

	// Convert points to VectorEntry format
	entries := make([]*VectorEntry, 0, len(response))
	for i, point := range response {
		entry, err := q.convertPointToVectorEntry(point)
		if err != nil {
			log.Printf("[QDRant] Warning: failed to convert point %d for document %s: %v", i, documentID, err)
			continue
		}
		entries = append(entries, entry)
	}

	log.Printf("[QDRant] Retrieved %d vectors for document %s", len(entries), documentID)
	return entries, nil
}

// Clear removes all vectors from the store
func (q *QDRantVectorStore) Clear() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	ctx := context.Background()

	// Delete all points from the collection
	_, err := q.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: q.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{}, // Empty filter matches all points
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to clear collection %s: %v", q.collection, err)
	}

	log.Printf("[QDRant] Successfully cleared all vectors from collection %s", q.collection)
	return nil
}


// Helper functions for Phase 1 - mostly stubs

// extractStringFromNestedMap extracts the actual string value from QDRant's nested map structure
// Handles patterns like: map[Kind:map[StringValue:chunk_6cde57d36be59bbe]]
func extractStringFromNestedMap(v interface{}) string {
	// Try to cast to map[string]interface{}
	if topMap, ok := v.(map[string]interface{}); ok {
		// Look for "Kind" key which should contain another map
		if kindValue, exists := topMap["Kind"]; exists {
			// Cast the Kind value to another map
			if kindMap, ok := kindValue.(map[string]interface{}); ok {
				// Look for "StringValue" key which should contain the actual string
				if stringValue, exists := kindMap["StringValue"]; exists {
					// Cast to string and return
					if str, ok := stringValue.(string); ok {
						return str
					}
				}
			}
		}
	}

	// If the structure doesn't match expected pattern, return empty string
	return ""
}

// structToMap converts a struct to a map[string]interface{} for QDRant payload
func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %v", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %v", err)
	}

	return result, nil
}

// convertPointToVectorEntry converts a QDRant point to a VectorEntry
func (q *QDRantVectorStore) convertPointToVectorEntry(point *qdrant.RetrievedPoint) (*VectorEntry, error) {
	if point == nil {
		return nil, fmt.Errorf("point is nil")
	}

	// Extract ID
	var id string
	switch idValue := point.Id.PointIdOptions.(type) {
	case *qdrant.PointId_Uuid:
		id = idValue.Uuid
	case *qdrant.PointId_Num:
		id = fmt.Sprintf("%d", idValue.Num)
	default:
		return nil, fmt.Errorf("unsupported point ID type")
	}

	// Extract vectors
	var vectors []float32
	if point.Vectors != nil {
		switch vectorData := point.Vectors.VectorsOptions.(type) {
		case *qdrant.Vectors_Vector:
			vectors = vectorData.Vector.Data
		default:
			return nil, fmt.Errorf("unsupported vector format")
		}
	}

	// Extract payload
	payload := &QDRantPayload{}
	if point.Payload != nil {
		// Convert payload map to JSON and back to our struct
		payloadData, err := json.Marshal(point.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %v", err)
		}

		err = json.Unmarshal(payloadData, payload)
		if err != nil {
			// Log the specific unmarshal error with more context for debugging
			log.Printf("[QDRant] Error: Failed to unmarshal payload. JSON: %s, Error: %v", string(payloadData), err)

			// Try to log what the problematic chunk_id looks like
			var rawPayload map[string]interface{}
			if jsonErr := json.Unmarshal(payloadData, &rawPayload); jsonErr == nil {
				if chunkIDValue, exists := rawPayload["chunk_id"]; exists {
					log.Printf("[QDRant] Debug: Problematic chunk_id type: %T, value: %v", chunkIDValue, chunkIDValue)
				}
			}

			return nil, fmt.Errorf("failed to unmarshal payload: %v", err)
		}
	}

	// Handle nullable ChunkID
	var chunkID string
	if payload.ChunkID != nil {
		chunkID = *payload.ChunkID
	}

	return &VectorEntry{
		ID:         id,
		DocumentID: payload.DocumentID,
		ChunkID:    chunkID,
		Vector:     vectors,
		Dimension:  len(vectors),
		Content:    payload.Content,
		Metadata:   payload.Metadata,
		CreatedAt:  time.Unix(payload.CreatedAt, 0),
	}, nil
}