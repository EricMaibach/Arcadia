package stores

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// QdrantVectorStore implements VectorStoreInterface using QDrant vector database
type QdrantVectorStore struct {
	client     *qdrant.Client
	collection string
	config     QdrantConfig
	logger     interfaces.Logger
	metrics    interfaces.MetricsCollector
	mutex      sync.RWMutex
}

// QdrantConfig contains configuration for QDrant vector store
type QdrantConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	ApiKey     string `json:"api_key,omitempty"`
	Collection string `json:"collection"`
	Dimension  int    `json:"dimension"`
	UseHTTPS   bool   `json:"use_https"`
	Timeout    int    `json:"timeout"`    // Connection timeout in seconds
	MaxRetries int    `json:"max_retries"`
	RetryDelay int    `json:"retry_delay"` // Delay between retries in seconds
}

// QDRantPayload represents the payload structure for QDrant points
// Uses custom JSON marshaling to handle any type for chunk_id field
type QDRantPayload struct {
	DocumentID string            `json:"document_id"`
	ChunkID    *string           `json:"-"`                    // Handle manually
	Content    string            `json:"content"`
	Metadata   string            `json:"metadata,omitempty"`
	CreatedAt  int64             `json:"created_at"`
	logger     interfaces.Logger `json:"-"`                    // For logging warnings
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
				if p.logger != nil {
					p.logger.Warn(nil, "chunk_id has unexpected type, converting to string", "type", fmt.Sprintf("%T", v), "value", v)
				}
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

// Add a logger field to QDRantPayload for warnings
func (p *QDRantPayload) setLogger(logger interfaces.Logger) {
	p.logger = logger
}

// NewQdrantVectorStore creates a new QDrant vector store
func NewQdrantVectorStore(config QdrantConfig) (*QdrantVectorStore, error) {
	if config.Host == "" {
		return nil, models.NewDocumentError(models.ErrInvalidConfig, "QDrant host cannot be empty")
	}

	if config.Port <= 0 {
		config.Port = 6333 // Default QDrant port
	}

	if config.Collection == "" {
		return nil, models.NewDocumentError(models.ErrInvalidConfig, "QDrant collection cannot be empty")
	}

	if config.Dimension <= 0 {
		return nil, models.NewDocumentError(models.ErrInvalidConfig, "QDrant dimension must be positive")
	}

	store := &QdrantVectorStore{
		config:     config,
		collection: config.Collection,
	}

	// Initialize QDRant client connection
	if err := store.initializeClient(); err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrStorageFailed, "failed to initialize QDrant client", err)
	}

	return store, nil
}

// WithLogger adds logging to the QDrant vector store
func (qvs *QdrantVectorStore) WithLogger(logger interfaces.Logger) *QdrantVectorStore {
	qvs.logger = logger
	return qvs
}

// WithMetrics adds metrics collection to the QDrant vector store
func (qvs *QdrantVectorStore) WithMetrics(metrics interfaces.MetricsCollector) *QdrantVectorStore {
	qvs.metrics = metrics
	return qvs
}

// initializeClient initializes the QDRant client connection
func (qvs *QdrantVectorStore) initializeClient() error {
	// Build connection URL
	scheme := "http"
	if qvs.config.UseHTTPS {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s:%d", scheme, qvs.config.Host, qvs.config.Port)

	// Create client configuration
	config := &qdrant.Config{
		Host:   qvs.config.Host,
		Port:   qvs.config.Port,
		APIKey: qvs.config.ApiKey,
		UseTLS: qvs.config.UseHTTPS,
	}

	// Create client
	client, err := qdrant.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create QDRant client for %s: %v", url, err)
	}

	qvs.client = client
	if qvs.logger != nil {
		qvs.logger.Info(context.Background(), "Connected to QDRant", "url", url)
	}
	return nil
}

// Initialize initializes the QDrant vector store and creates collection if needed
func (qvs *QdrantVectorStore) Initialize(ctx context.Context) error {
	return qvs.ensureCollection(ctx)
}

// ensureCollection ensures the collection exists, creating it if necessary
func (qvs *QdrantVectorStore) ensureCollection(ctx context.Context) error {
	// Check if collection exists
	collectionInfo, err := qvs.client.GetCollectionInfo(ctx, qvs.collection)

	if err != nil {
		// Collection doesn't exist, create it
		if qvs.logger != nil {
			qvs.logger.Info(ctx, "Creating QDrant collection", "collection", qvs.collection)
		}

		err = qvs.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: qvs.collection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     uint64(qvs.config.Dimension),
				Distance: qdrant.Distance_Cosine,
			}),
		})

		if err != nil {
			return fmt.Errorf("failed to create collection '%s': %v", qvs.collection, err)
		}

		if qvs.logger != nil {
			qvs.logger.Info(ctx, "Successfully created QDrant collection", "collection", qvs.collection)
		}
		return nil
	}

	// Collection exists
	if collectionInfo != nil && qvs.logger != nil {
		qvs.logger.Info(ctx, "QDrant collection already exists", "collection", qvs.collection)
	}

	return nil
}

// StoreVector stores a vector entry in QDrant
func (qvs *QdrantVectorStore) StoreVector(ctx context.Context, entry *models.VectorEntry) error {
	qvs.mutex.Lock()
	defer qvs.mutex.Unlock()

	if entry == nil {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector entry cannot be nil")
	}

	if entry.ID == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector entry ID cannot be empty")
	}

	if len(entry.Vector) == 0 {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if qvs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			qvs.metrics.RecordTimer("vector_store.qdrant.store.duration", duration, nil)
		}
	}()

	// Create payload
	payload := &QDRantPayload{
		DocumentID: entry.DocumentID,
		ChunkID:    &entry.ChunkID,
		Content:    entry.Content,
		Metadata:   entry.Metadata,
		CreatedAt:  entry.CreatedAt.Unix(),
	}
	payload.setLogger(qvs.logger)

	// Convert payload to map
	payloadMap, err := structToMap(payload)
	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to convert payload to map", err)
	}

	// Create point
	point := &qdrant.PointStruct{
		Id:      qdrant.NewIDUUID(entry.ID),
		Vectors: qdrant.NewVectors(entry.Vector...),
		Payload: qdrant.NewValueMap(payloadMap),
	}

	// Upsert the point
	_, err = qvs.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: qvs.collection,
		Points:         []*qdrant.PointStruct{point},
	})

	if err != nil {
		if qvs.metrics != nil {
			qvs.metrics.IncrementCounter("vector_store.qdrant.store.error", nil)
		}
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, fmt.Sprintf("failed to upsert point %s", entry.ID), err)
	}

	if qvs.metrics != nil {
		qvs.metrics.IncrementCounter("vector_store.qdrant.store.success", nil)
	}

	return nil
}

// SearchSimilar searches for similar vectors in QDrant
func (qvs *QdrantVectorStore) SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]*models.SearchResult, error) {
	return qvs.SearchWithFilter(ctx, queryVector, topK, nil)
}

// SearchWithFilter searches for similar vectors with optional filtering
func (qvs *QdrantVectorStore) SearchWithFilter(ctx context.Context, queryVector []float32, topK int, filter map[string]interface{}) ([]*models.SearchResult, error) {
	qvs.mutex.RLock()
	defer qvs.mutex.RUnlock()

	if len(queryVector) == 0 {
		return nil, models.NewDocumentError(models.ErrSearchFailed, "query vector cannot be empty")
	}

	if topK <= 0 {
		return []*models.SearchResult{}, nil
	}

	startTime := time.Now()
	defer func() {
		if qvs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			qvs.metrics.RecordTimer("vector_store.qdrant.search.duration", duration, nil)
		}
	}()

	// Build the query request
	queryRequest := &qdrant.QueryPoints{
		CollectionName: qvs.collection,
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
	response, err := qvs.client.Query(ctx, queryRequest)
	if err != nil {
		if qvs.metrics != nil {
			qvs.metrics.IncrementCounter("vector_store.qdrant.search.error", nil)
		}
		return nil, models.NewDocumentErrorWithCause(models.ErrSearchFailed, "failed to query vectors", err)
	}

	// Convert results to SearchResult format
	results := make([]*models.SearchResult, 0, len(response))
	for i, point := range response {
		// Convert ScoredPoint to RetrievedPoint for our conversion function
		retrievedPoint := &qdrant.RetrievedPoint{
			Id:      point.Id,
			Payload: point.Payload,
			Vectors: point.Vectors,
		}

		entry, err := qvs.convertPointToVectorEntry(retrievedPoint)
		if err != nil {
			if qvs.logger != nil {
				qvs.logger.Warn(ctx, "Failed to convert point", "index", i, "error", err)
			}
			continue
		}

		results = append(results, &models.SearchResult{
			Entry: entry,
			Score: point.Score,
		})
	}

	if qvs.metrics != nil {
		qvs.metrics.IncrementCounter("vector_store.qdrant.search.success", map[string]string{
			"results": fmt.Sprintf("%d", len(results)),
		})
	}

	return results, nil
}

// GetVector retrieves a vector by ID from QDrant
func (qvs *QdrantVectorStore) GetVector(ctx context.Context, id string) (*models.VectorEntry, error) {
	qvs.mutex.RLock()
	defer qvs.mutex.RUnlock()

	if id == "" {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "vector ID cannot be empty")
	}

	// Get the point by ID
	response, err := qvs.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: qvs.collection,
		Ids:            []*qdrant.PointId{qdrant.NewIDUUID(id)},
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(true),
	})

	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrDocumentNotFound, fmt.Sprintf("failed to get vector %s", id), err)
	}

	if len(response) == 0 {
		return nil, models.NewDocumentError(models.ErrDocumentNotFound, fmt.Sprintf("vector not found: %s", id))
	}

	// Convert the point to VectorEntry
	entry, err := qvs.convertPointToVectorEntry(response[0])
	if err != nil {
		return nil, models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, "failed to convert point to vector entry", err)
	}

	return entry, nil
}

// DeleteVector deletes a vector by ID from QDrant
func (qvs *QdrantVectorStore) DeleteVector(ctx context.Context, id string) error {
	qvs.mutex.Lock()
	defer qvs.mutex.Unlock()

	if id == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "vector ID cannot be empty")
	}

	// Delete the point by ID
	_, err := qvs.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: qvs.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: []*qdrant.PointId{qdrant.NewIDUUID(id)},
				},
			},
		},
	})

	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, fmt.Sprintf("failed to delete vector %s", id), err)
	}

	if qvs.metrics != nil {
		qvs.metrics.IncrementCounter("vector_store.qdrant.delete.success", nil)
	}

	return nil
}

// DeleteDocumentVectors deletes all vectors for a document from QDrant
func (qvs *QdrantVectorStore) DeleteDocumentVectors(ctx context.Context, documentID string) error {
	qvs.mutex.Lock()
	defer qvs.mutex.Unlock()

	if documentID == "" {
		return models.NewDocumentError(models.ErrVectorStoreFailed, "document ID cannot be empty")
	}

	// Delete all points with matching document_id in payload
	_, err := qvs.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: qvs.collection,
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
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, fmt.Sprintf("failed to delete document vectors for %s", documentID), err)
	}

	if qvs.metrics != nil {
		qvs.metrics.IncrementCounter("vector_store.qdrant.delete_document.success", nil)
	}

	return nil
}

// GetDocumentVectors retrieves all vectors for a document from QDrant
func (qvs *QdrantVectorStore) GetDocumentVectors(ctx context.Context, documentID string) ([]*models.VectorEntry, error) {
	qvs.mutex.RLock()
	defer qvs.mutex.RUnlock()

	if documentID == "" {
		return nil, models.NewDocumentError(models.ErrVectorStoreFailed, "document ID cannot be empty")
	}

	// Use scroll to get all points matching the document_id filter
	response, err := qvs.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: qvs.collection,
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
		return nil, models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, fmt.Sprintf("failed to get document vectors for %s", documentID), err)
	}

	// Convert points to VectorEntry format
	entries := make([]*models.VectorEntry, 0, len(response))
	for i, point := range response {
		entry, err := qvs.convertPointToVectorEntry(point)
		if err != nil {
			if qvs.logger != nil {
				qvs.logger.Warn(ctx, "Failed to convert point for document", "index", i, "document_id", documentID, "error", err)
			}
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// Clear removes all vectors from the collection
func (qvs *QdrantVectorStore) Clear(ctx context.Context) error {
	qvs.mutex.Lock()
	defer qvs.mutex.Unlock()

	// Delete all points from the collection
	_, err := qvs.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: qvs.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{}, // Empty filter matches all points
			},
		},
	})

	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, fmt.Sprintf("failed to clear collection %s", qvs.collection), err)
	}

	if qvs.logger != nil {
		qvs.logger.Info(ctx, "Successfully cleared QDrant collection", "collection", qvs.collection)
	}

	return nil
}

// StoreBatch stores multiple vectors in a batch
func (qvs *QdrantVectorStore) StoreBatch(ctx context.Context, entries []*models.VectorEntry) error {
	qvs.mutex.Lock()
	defer qvs.mutex.Unlock()

	if len(entries) == 0 {
		return nil // Nothing to store
	}

	startTime := time.Now()
	defer func() {
		if qvs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			qvs.metrics.RecordTimer("vector_store.qdrant.store_batch.duration", duration, nil)
		}
	}()

	// Convert all entries to QDRant points
	points := make([]*qdrant.PointStruct, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.ID == "" || len(entry.Vector) == 0 {
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
		payload.setLogger(qvs.logger)

		// Convert payload to map
		payloadMap, err := structToMap(payload)
		if err != nil {
			return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, fmt.Sprintf("failed to convert payload to map for entry %s", entry.ID), err)
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
	_, err := qvs.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: qvs.collection,
		Points:         points,
	})

	if err != nil {
		if qvs.metrics != nil {
			qvs.metrics.IncrementCounter("vector_store.qdrant.store_batch.error", nil)
		}
		return models.NewDocumentErrorWithCause(models.ErrVectorStoreFailed, fmt.Sprintf("failed to batch upsert %d points", len(points)), err)
	}

	if qvs.metrics != nil {
		qvs.metrics.IncrementCounter("vector_store.qdrant.store_batch.success", map[string]string{
			"count": fmt.Sprintf("%d", len(points)),
		})
	}

	return nil
}

// HealthCheck performs a health check on the QDrant vector store
func (qvs *QdrantVectorStore) HealthCheck(ctx context.Context) error {
	qvs.mutex.RLock()
	defer qvs.mutex.RUnlock()

	if qvs.client == nil {
		return models.NewDocumentError(models.ErrStorageFailed, "QDRant client not initialized")
	}

	// Check if we can get collection info
	collectionInfo, err := qvs.client.GetCollectionInfo(ctx, qvs.collection)

	if err != nil {
		return models.NewDocumentErrorWithCause(models.ErrStorageFailed, fmt.Sprintf("failed to get collection info for '%s'", qvs.collection), err)
	}

	if collectionInfo == nil {
		return models.NewDocumentError(models.ErrStorageFailed, fmt.Sprintf("collection '%s' not found", qvs.collection))
	}

	return nil
}

// Helper functions

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
func (qvs *QdrantVectorStore) convertPointToVectorEntry(point *qdrant.RetrievedPoint) (*models.VectorEntry, error) {
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
	payload.setLogger(qvs.logger)
	if point.Payload != nil {
		// Convert payload map to JSON and back to our struct
		payloadData, err := json.Marshal(point.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %v", err)
		}

		err = json.Unmarshal(payloadData, payload)
		if err != nil {
			// Log the specific unmarshal error with more context for debugging
			if qvs.logger != nil {
				qvs.logger.Error(context.Background(), "Failed to unmarshal payload", "json", string(payloadData), "error", err)

				// Try to log what the problematic chunk_id looks like
				var rawPayload map[string]interface{}
				if jsonErr := json.Unmarshal(payloadData, &rawPayload); jsonErr == nil {
					if chunkIDValue, exists := rawPayload["chunk_id"]; exists {
						qvs.logger.Debug(context.Background(), "Problematic chunk_id", "type", fmt.Sprintf("%T", chunkIDValue), "value", chunkIDValue)
					}
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

	return &models.VectorEntry{
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

// DefaultQdrantConfig returns a default QDrant configuration
func DefaultQdrantConfig() QdrantConfig {
	return QdrantConfig{
		Host:       "localhost",
		Port:       6333,
		Collection: "documents",
		Dimension:  768, // Default for EmbeddingGemma
		UseHTTPS:   false,
		Timeout:    30,
		MaxRetries: 3,
		RetryDelay: 5,
	}
}

