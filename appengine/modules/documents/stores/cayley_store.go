package stores

import (
	"context"
	"fmt"
	"time"

	"github.com/cayleygraph/cayley"
	"github.com/cayleygraph/cayley/graph"
	_ "github.com/cayleygraph/cayley/graph/kv/bolt" // BoltDB backend
	"github.com/cayleygraph/quad"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// CayleyGraphStore implements GraphStoreInterface using Cayley with BoltDB backend
type CayleyGraphStore struct {
	store   *cayley.Handle
	backend string
	path    string
	logger  interfaces.Logger
	metrics interfaces.MetricsCollector
}

// CayleyConfig contains configuration for Cayley graph store
type CayleyConfig struct {
	Backend string                 `json:"backend"` // bolt, postgres, mongo, etc.
	Path    string                 `json:"path"`    // Database path/connection string
	Options map[string]interface{} `json:"options"` // Backend-specific options
}

// NewCayleyGraphStore creates a new Cayley graph store
func NewCayleyGraphStore(config CayleyConfig) (*CayleyGraphStore, error) {
	if config.Backend == "" {
		config.Backend = "bolt"
	}
	if config.Path == "" {
		config.Path = "./data/cayley.db"
	}

	return &CayleyGraphStore{
		backend: config.Backend,
		path:    config.Path,
	}, nil
}

// WithLogger adds logging support
func (cgs *CayleyGraphStore) WithLogger(logger interfaces.Logger) *CayleyGraphStore {
	cgs.logger = logger
	return cgs
}

// WithMetrics adds metrics collection support
func (cgs *CayleyGraphStore) WithMetrics(metrics interfaces.MetricsCollector) *CayleyGraphStore {
	cgs.metrics = metrics
	return cgs
}

// Initialize initializes the Cayley graph store
func (cgs *CayleyGraphStore) Initialize(ctx context.Context) error {
	// Initialize the database (creates if doesn't exist)
	err := graph.InitQuadStore(cgs.backend, cgs.path, nil)
	if err != nil && err != graph.ErrDatabaseExists {
		return fmt.Errorf("failed to initialize graph store: %w", err)
	}

	// Open the database
	store, err := cayley.NewGraph(cgs.backend, cgs.path, nil)
	if err != nil {
		return fmt.Errorf("failed to open graph store: %w", err)
	}

	cgs.store = store

	if cgs.logger != nil {
		cgs.logger.Info(ctx, "Cayley graph store initialized",
			"backend", cgs.backend,
			"path", cgs.path)
	}

	return nil
}

// AddQuad adds a single quad to the graph
func (cgs *CayleyGraphStore) AddQuad(ctx context.Context, q models.Quad) error {
	startTime := time.Now()
	defer func() {
		if cgs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			cgs.metrics.RecordTimer("graph.add_quad.duration", duration, nil)
		}
	}()

	if err := q.Validate(); err != nil {
		return fmt.Errorf("invalid quad: %w", err)
	}

	cayleyQuad := cgs.modelQuadToCayleyQuad(q)

	if err := cgs.store.AddQuad(cayleyQuad); err != nil {
		if cgs.metrics != nil {
			cgs.metrics.IncrementCounter("graph.add_quad.error", nil)
		}
		return fmt.Errorf("failed to add quad: %w", err)
	}

	if cgs.metrics != nil {
		cgs.metrics.IncrementCounter("graph.add_quad.success", nil)
	}

	return nil
}

// AddQuads adds multiple quads to the graph in a batch
func (cgs *CayleyGraphStore) AddQuads(ctx context.Context, quads []models.Quad) error {
	startTime := time.Now()
	defer func() {
		if cgs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			cgs.metrics.RecordTimer("graph.add_quads.duration", duration, map[string]string{
				"count": fmt.Sprintf("%d", len(quads)),
			})
		}
	}()

	// Validate all quads first
	for i, q := range quads {
		if err := q.Validate(); err != nil {
			return fmt.Errorf("invalid quad at index %d: %w", i, err)
		}
	}

	// Convert to Cayley quads
	cayleyQuads := make([]quad.Quad, len(quads))
	for i, q := range quads {
		cayleyQuads[i] = cgs.modelQuadToCayleyQuad(q)
	}

	// Add quads as a transaction
	writer, err := cgs.store.NewQuadWriter()
	if err != nil {
		if cgs.metrics != nil {
			cgs.metrics.IncrementCounter("graph.add_quads.error", nil)
		}
		return fmt.Errorf("failed to create quad writer: %w", err)
	}

	for _, q := range cayleyQuads {
		if err := writer.WriteQuad(q); err != nil {
			writer.Close()
			if cgs.metrics != nil {
				cgs.metrics.IncrementCounter("graph.add_quads.error", nil)
			}
			return fmt.Errorf("failed to write quad: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		if cgs.metrics != nil {
			cgs.metrics.IncrementCounter("graph.add_quads.error", nil)
		}
		return fmt.Errorf("failed to close quad writer: %w", err)
	}

	if cgs.metrics != nil {
		cgs.metrics.IncrementCounter("graph.add_quads.success", map[string]string{
			"count": fmt.Sprintf("%d", len(quads)),
		})
	}

	if cgs.logger != nil {
		cgs.logger.Debug(ctx, "Added quads to graph",
			"count", len(quads))
	}

	return nil
}

// DeleteQuad deletes a single quad from the graph
func (cgs *CayleyGraphStore) DeleteQuad(ctx context.Context, q models.Quad) error {
	cayleyQuad := cgs.modelQuadToCayleyQuad(q)

	if err := cgs.store.RemoveQuad(cayleyQuad); err != nil {
		return fmt.Errorf("failed to delete quad: %w", err)
	}

	return nil
}

// DeleteQuads deletes multiple quads from the graph
func (cgs *CayleyGraphStore) DeleteQuads(ctx context.Context, quads []models.Quad) error {
	cayleyQuads := make([]quad.Quad, len(quads))
	for i, q := range quads {
		cayleyQuads[i] = cgs.modelQuadToCayleyQuad(q)
	}

	// Use RemoveQuad for deletions
	for _, q := range cayleyQuads {
		if err := cgs.store.RemoveQuad(q); err != nil {
			return fmt.Errorf("failed to remove quad: %w", err)
		}
	}

	return nil
}

// Query executes a query against the graph
// Note: This is a simplified implementation. For production, you'd want to support
// proper Gizmo or GraphQL queries
func (cgs *CayleyGraphStore) Query(ctx context.Context, query string) ([]map[string]interface{}, error) {
	// This is a placeholder for query functionality
	// In a full implementation, you would:
	// 1. Parse the query (Gizmo/GraphQL/etc)
	// 2. Execute against Cayley
	// 3. Return results
	return nil, fmt.Errorf("query functionality not yet implemented")
}

// GetNode retrieves a node and its properties from the graph
func (cgs *CayleyGraphStore) GetNode(ctx context.Context, nodeID string) (*models.GraphNode, error) {
	// Get all outgoing predicates and objects
	properties := make(map[string]interface{})
	nodeType := ""

	// Build a path starting from the node to get all properties
	p := cayley.StartPath(cgs.store, quad.String(nodeID)).Out()

	// Iterate through outgoing edges to collect properties
	it := p.BuildIterator()
	defer it.Close()

	for it.Next(ctx) {
		// This is a simplified implementation
		// In production, you'd properly extract predicates and values
		token := it.Result()
		value := cgs.store.NameOf(token)
		if strValue, ok := value.(quad.String); ok {
			// Store as property (this is simplified)
			properties["value"] = string(strValue)
		}
	}

	return &models.GraphNode{
		ID:         nodeID,
		Type:       nodeType,
		Properties: properties,
	}, nil
}

// GetEdges retrieves all edges (relationships) for a given node
func (cgs *CayleyGraphStore) GetEdges(ctx context.Context, nodeID string) ([]models.GraphEdge, error) {
	// This is a placeholder
	// In a full implementation, you would query all quads where:
	// - Subject == nodeID (outgoing edges)
	// - Object == nodeID (incoming edges)
	return []models.GraphEdge{}, nil
}

// DeleteDocumentData deletes all graph data for a specific document
func (cgs *CayleyGraphStore) DeleteDocumentData(ctx context.Context, documentID string) error {
	startTime := time.Now()
	defer func() {
		if cgs.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			cgs.metrics.RecordTimer("graph.delete_document.duration", duration, nil)
		}
	}()

	// Find all quads with this document as the label
	// We need to iterate through all quads and check their labels
	it := cgs.store.QuadsAllIterator()
	defer it.Close()

	quadsToDelete := []quad.Quad{}

	for it.Next(ctx) {
		token := it.Result()
		q := cgs.store.Quad(token)

		// Check if the label matches the document ID
		if q.Label != nil {
			if labelStr, ok := q.Label.(quad.String); ok {
				if string(labelStr) == documentID {
					quadsToDelete = append(quadsToDelete, q)
				}
			}
		}
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("error iterating quads: %w", err)
	}

	// Delete the quads
	if len(quadsToDelete) > 0 {
		for _, q := range quadsToDelete {
			if err := cgs.store.RemoveQuad(q); err != nil {
				return fmt.Errorf("failed to remove quad: %w", err)
			}
		}

		if cgs.logger != nil {
			cgs.logger.Info(ctx, "Deleted document data from graph",
				"document_id", documentID,
				"quads_deleted", len(quadsToDelete))
		}
	}

	return nil
}

// GetStats retrieves statistics about the graph
func (cgs *CayleyGraphStore) GetStats(ctx context.Context) (*models.GraphStats, error) {
	stats := models.NewGraphStats()

	// Count quads by iterating through all of them
	it := cgs.store.QuadsAllIterator()
	defer it.Close()

	quadCount := int64(0)
	entities := make(map[string]bool)
	relationships := 0
	documents := make(map[string]bool)
	typeDistribution := make(map[string]int)

	for it.Next(ctx) {
		quadCount++
		token := it.Result()
		q := cgs.store.Quad(token)

		// Track documents via labels
		if q.Label != nil {
			if labelStr, ok := q.Label.(quad.String); ok {
				documents[string(labelStr)] = true
			}
		}

		// Check if this is a type predicate (indicates an entity)
		if predStr, ok := q.Predicate.(quad.String); ok {
			if string(predStr) == "type" {
				if subjStr, ok := q.Subject.(quad.String); ok {
					entities[string(subjStr)] = true
				}
				if objStr, ok := q.Object.(quad.String); ok {
					typeDistribution[string(objStr)]++
				}
			} else if string(predStr) != "text" && string(predStr) != "extracted_from" && string(predStr) != "entity_id" {
				// Count as relationship if it's not a property predicate
				relationships++
			}
		}
	}

	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("error iterating quads: %w", err)
	}

	stats.QuadCount = quadCount
	stats.EntityCount = len(entities)
	stats.RelationshipCount = relationships
	stats.DocumentCount = len(documents)
	stats.TypeDistribution = typeDistribution

	return stats, nil
}

// HealthCheck performs a health check on the graph store
func (cgs *CayleyGraphStore) HealthCheck(ctx context.Context) error {
	if cgs.store == nil {
		return fmt.Errorf("graph store not initialized")
	}

	// Try a simple operation to verify the store is working
	it := cgs.store.QuadsAllIterator()
	if it == nil {
		return fmt.Errorf("failed to create iterator")
	}
	it.Close()

	return nil
}

// Close closes the graph store
func (cgs *CayleyGraphStore) Close() error {
	if cgs.store != nil {
		if err := cgs.store.Close(); err != nil {
			return fmt.Errorf("failed to close graph store: %w", err)
		}
		if cgs.logger != nil {
			cgs.logger.Info(context.Background(), "Cayley graph store closed")
		}
	}
	return nil
}

// modelQuadToCayleyQuad converts a models.Quad to a quad.Quad
func (cgs *CayleyGraphStore) modelQuadToCayleyQuad(q models.Quad) quad.Quad {
	var object quad.Value

	// Convert object to appropriate type
	switch v := q.Object.(type) {
	case string:
		object = quad.String(v)
	case int:
		object = quad.Int(int64(v))
	case int64:
		object = quad.Int(v)
	case float64:
		object = quad.Float(v)
	case bool:
		object = quad.Bool(v)
	default:
		// Fallback to string representation
		object = quad.String(fmt.Sprintf("%v", v))
	}

	return quad.Make(
		quad.String(q.Subject),
		quad.String(q.Predicate),
		object,
		quad.String(q.Label),
	)
}
