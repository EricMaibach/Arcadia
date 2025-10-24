package stores

import (
	"context"
	"fmt"
	"sync"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// GraphStoreConfig contains configuration for creating a graph store
type GraphStoreConfig struct {
	Type    string                 `json:"type"`    // cayley, neo4j, memory
	Backend string                 `json:"backend"` // For Cayley: bolt, postgres, etc.
	Path    string                 `json:"path"`    // Database path or connection string
	Options map[string]interface{} `json:"options"` // Backend-specific options
}

// GraphStoreFactory creates graph store instances
type GraphStoreFactory struct {
	logger  interfaces.Logger
	metrics interfaces.MetricsCollector
}

// NewGraphStoreFactory creates a new graph store factory
func NewGraphStoreFactory() *GraphStoreFactory {
	return &GraphStoreFactory{}
}

// WithLogger adds logging support to the factory
func (gsf *GraphStoreFactory) WithLogger(logger interfaces.Logger) *GraphStoreFactory {
	gsf.logger = logger
	return gsf
}

// WithMetrics adds metrics collection support to the factory
func (gsf *GraphStoreFactory) WithMetrics(metrics interfaces.MetricsCollector) *GraphStoreFactory {
	gsf.metrics = metrics
	return gsf
}

// CreateGraphStore creates a graph store based on configuration
func (gsf *GraphStoreFactory) CreateGraphStore(config GraphStoreConfig) (interfaces.GraphStoreInterface, error) {
	switch config.Type {
	case "cayley":
		cayleyConfig := CayleyConfig{
			Backend: config.Backend,
			Path:    config.Path,
			Options: config.Options,
		}
		store, err := NewCayleyGraphStore(cayleyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Cayley store: %w", err)
		}
		if gsf.logger != nil {
			store.WithLogger(gsf.logger)
		}
		if gsf.metrics != nil {
			store.WithMetrics(gsf.metrics)
		}
		return store, nil

	case "memory":
		// Memory store for testing
		store := NewMemoryGraphStore()
		if gsf.logger != nil {
			store.WithLogger(gsf.logger)
		}
		return store, nil

	default:
		return nil, fmt.Errorf("unsupported graph store type: %s", config.Type)
	}
}

// MemoryGraphStore is a simple in-memory graph store for testing
type MemoryGraphStore struct {
	quads  []models.Quad
	mutex  sync.RWMutex
	logger interfaces.Logger
}

// NewMemoryGraphStore creates a new memory-based graph store
func NewMemoryGraphStore() *MemoryGraphStore {
	return &MemoryGraphStore{
		quads: make([]models.Quad, 0),
	}
}

// WithLogger adds logging support
func (mgs *MemoryGraphStore) WithLogger(logger interfaces.Logger) *MemoryGraphStore {
	mgs.logger = logger
	return mgs
}

// Initialize initializes the memory store (no-op)
func (mgs *MemoryGraphStore) Initialize(ctx context.Context) error {
	if mgs.logger != nil {
		mgs.logger.Info(ctx, "Memory graph store initialized")
	}
	return nil
}

// Close closes the memory store (no-op)
func (mgs *MemoryGraphStore) Close() error {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()
	mgs.quads = nil
	return nil
}

// HealthCheck performs a health check (always healthy)
func (mgs *MemoryGraphStore) HealthCheck(ctx context.Context) error {
	return nil
}

// AddQuad adds a single quad to memory
func (mgs *MemoryGraphStore) AddQuad(ctx context.Context, quad models.Quad) error {
	if err := quad.Validate(); err != nil {
		return err
	}

	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()

	mgs.quads = append(mgs.quads, quad)
	return nil
}

// AddQuads adds multiple quads to memory
func (mgs *MemoryGraphStore) AddQuads(ctx context.Context, quads []models.Quad) error {
	for i, q := range quads {
		if err := q.Validate(); err != nil {
			return fmt.Errorf("invalid quad at index %d: %w", i, err)
		}
	}

	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()

	mgs.quads = append(mgs.quads, quads...)
	return nil
}

// DeleteQuad deletes a single quad from memory
func (mgs *MemoryGraphStore) DeleteQuad(ctx context.Context, quad models.Quad) error {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()

	for i, q := range mgs.quads {
		if mgs.quadsEqual(q, quad) {
			// Remove quad by swapping with last and truncating
			mgs.quads[i] = mgs.quads[len(mgs.quads)-1]
			mgs.quads = mgs.quads[:len(mgs.quads)-1]
			return nil
		}
	}

	return nil // Quad not found, but not an error
}

// DeleteQuads deletes multiple quads from memory
func (mgs *MemoryGraphStore) DeleteQuads(ctx context.Context, quads []models.Quad) error {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()

	for _, quadToDelete := range quads {
		for i := len(mgs.quads) - 1; i >= 0; i-- {
			if mgs.quadsEqual(mgs.quads[i], quadToDelete) {
				mgs.quads = append(mgs.quads[:i], mgs.quads[i+1:]...)
			}
		}
	}

	return nil
}

// Query executes a query (not implemented for memory store)
func (mgs *MemoryGraphStore) Query(ctx context.Context, query string) ([]map[string]interface{}, error) {
	return nil, fmt.Errorf("query not implemented for memory store")
}

// GetNode retrieves a node (simplified implementation)
func (mgs *MemoryGraphStore) GetNode(ctx context.Context, nodeID string) (*models.GraphNode, error) {
	mgs.mutex.RLock()
	defer mgs.mutex.RUnlock()

	properties := make(map[string]interface{})
	nodeType := ""

	for _, q := range mgs.quads {
		if q.Subject == nodeID {
			if q.Predicate == "type" {
				if t, ok := q.Object.(string); ok {
					nodeType = t
				}
			} else {
				properties[q.Predicate] = q.Object
			}
		}
	}

	return &models.GraphNode{
		ID:         nodeID,
		Type:       nodeType,
		Properties: properties,
	}, nil
}

// GetEdges retrieves all edges for a node
func (mgs *MemoryGraphStore) GetEdges(ctx context.Context, nodeID string) ([]models.GraphEdge, error) {
	mgs.mutex.RLock()
	defer mgs.mutex.RUnlock()

	edges := []models.GraphEdge{}

	for _, q := range mgs.quads {
		// Skip property quads (type, text, etc.)
		if q.Predicate == "type" || q.Predicate == "text" || q.Predicate == "extracted_from" || q.Predicate == "entity_id" {
			continue
		}

		if q.Subject == nodeID {
			if objectNode, ok := q.Object.(string); ok {
				edges = append(edges, models.GraphEdge{
					From:      q.Subject,
					Predicate: q.Predicate,
					To:        objectNode,
					Label:     q.Label,
				})
			}
		}
	}

	return edges, nil
}

// DeleteDocumentData deletes all quads with a specific document label
func (mgs *MemoryGraphStore) DeleteDocumentData(ctx context.Context, documentID string) error {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()

	newQuads := make([]models.Quad, 0)
	deleted := 0

	for _, q := range mgs.quads {
		if q.Label != documentID {
			newQuads = append(newQuads, q)
		} else {
			deleted++
		}
	}

	mgs.quads = newQuads

	if mgs.logger != nil {
		mgs.logger.Info(ctx, "Deleted document data from memory graph",
			"document_id", documentID,
			"quads_deleted", deleted)
	}

	return nil
}

// GetStats retrieves statistics about the graph
func (mgs *MemoryGraphStore) GetStats(ctx context.Context) (*models.GraphStats, error) {
	mgs.mutex.RLock()
	defer mgs.mutex.RUnlock()

	stats := models.NewGraphStats()
	stats.QuadCount = int64(len(mgs.quads))

	// Count unique entities (subjects with type predicate)
	entities := make(map[string]bool)
	relationships := 0
	documents := make(map[string]bool)
	typeDistribution := make(map[string]int)

	for _, q := range mgs.quads {
		// Track documents
		if q.Label != "" {
			documents[q.Label] = true
		}

		// Track entities
		if q.Predicate == "type" {
			entities[q.Subject] = true
			if entityType, ok := q.Object.(string); ok {
				typeDistribution[entityType]++
			}
		}

		// Count relationships (non-property predicates)
		if q.Predicate != "type" && q.Predicate != "text" && q.Predicate != "extracted_from" && q.Predicate != "entity_id" {
			relationships++
		}
	}

	stats.EntityCount = len(entities)
	stats.RelationshipCount = relationships
	stats.DocumentCount = len(documents)
	stats.TypeDistribution = typeDistribution

	return stats, nil
}

// quadsEqual checks if two quads are equal
func (mgs *MemoryGraphStore) quadsEqual(a, b models.Quad) bool {
	return a.Subject == b.Subject &&
		a.Predicate == b.Predicate &&
		fmt.Sprintf("%v", a.Object) == fmt.Sprintf("%v", b.Object) &&
		a.Label == b.Label
}
