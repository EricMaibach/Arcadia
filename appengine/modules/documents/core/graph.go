package core

import (
	"context"
	"fmt"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
)

// GraphEngine provides high-level graph operations for entity and relationship storage
type GraphEngine struct {
	store   interfaces.GraphStoreInterface
	logger  interfaces.Logger
	metrics interfaces.MetricsCollector
}

// NewGraphEngine creates a new graph engine
func NewGraphEngine(store interfaces.GraphStoreInterface) *GraphEngine {
	return &GraphEngine{
		store: store,
	}
}

// WithLogger adds logging support to the graph engine
func (ge *GraphEngine) WithLogger(logger interfaces.Logger) *GraphEngine {
	ge.logger = logger
	return ge
}

// WithMetrics adds metrics collection support to the graph engine
func (ge *GraphEngine) WithMetrics(metrics interfaces.MetricsCollector) *GraphEngine {
	ge.metrics = metrics
	return ge
}

// StoreExtractionResult stores entities and relationships from an extraction result
func (ge *GraphEngine) StoreExtractionResult(ctx context.Context, documentID string, result *models.ExtractionResult) error {
	if result == nil {
		return fmt.Errorf("extraction result is nil")
	}

	if documentID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if ge.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			ge.metrics.RecordTimer("graph.store_extraction.duration", duration, nil)
		}
	}()

	// Validate the extraction result
	if err := result.Validate(); err != nil {
		return fmt.Errorf("invalid extraction result: %w", err)
	}

	quads := make([]models.Quad, 0)

	// Create quads for each entity
	for _, entity := range result.Entities {
		entityQuads := models.MakeEntityQuads(documentID, entity)
		quads = append(quads, entityQuads...)
	}

	// Create quads for each relationship
	for _, relationship := range result.Relationships {
		relationshipQuad := models.MakeRelationshipQuad(documentID, relationship)
		quads = append(quads, relationshipQuad)
	}

	// Store all quads in batch
	if err := ge.store.AddQuads(ctx, quads); err != nil {
		if ge.metrics != nil {
			ge.metrics.IncrementCounter("graph.store_extraction.error", nil)
		}
		return fmt.Errorf("failed to store extraction in graph: %w", err)
	}

	if ge.logger != nil {
		ge.logger.Info(ctx, "Stored extraction result in graph",
			"document_id", documentID,
			"entity_count", len(result.Entities),
			"relationship_count", len(result.Relationships),
			"quad_count", len(quads))
	}

	if ge.metrics != nil {
		ge.metrics.IncrementCounter("graph.store_extraction.success", nil)
		ge.metrics.SetGauge("graph.entities_stored", float64(len(result.Entities)), map[string]string{
			"document_id": documentID,
		})
		ge.metrics.SetGauge("graph.relationships_stored", float64(len(result.Relationships)), map[string]string{
			"document_id": documentID,
		})
	}

	return nil
}

// StoreEntity stores a single entity in the graph
func (ge *GraphEngine) StoreEntity(ctx context.Context, documentID string, entity models.Entity) error {
	if err := entity.Validate(); err != nil {
		return fmt.Errorf("invalid entity: %w", err)
	}

	quads := models.MakeEntityQuads(documentID, entity)

	if err := ge.store.AddQuads(ctx, quads); err != nil {
		return fmt.Errorf("failed to store entity: %w", err)
	}

	if ge.logger != nil {
		ge.logger.Debug(ctx, "Stored entity in graph",
			"document_id", documentID,
			"entity_id", entity.ID,
			"entity_type", entity.Type,
			"entity_text", entity.Text)
	}

	return nil
}

// StoreRelationship stores a single relationship in the graph
func (ge *GraphEngine) StoreRelationship(ctx context.Context, documentID string, relationship models.Relationship) error {
	if err := relationship.Validate(); err != nil {
		return fmt.Errorf("invalid relationship: %w", err)
	}

	quad := models.MakeRelationshipQuad(documentID, relationship)

	if err := ge.store.AddQuad(ctx, quad); err != nil {
		return fmt.Errorf("failed to store relationship: %w", err)
	}

	if ge.logger != nil {
		ge.logger.Debug(ctx, "Stored relationship in graph",
			"document_id", documentID,
			"subject", relationship.Subject,
			"predicate", relationship.Predicate,
			"object", relationship.Object)
	}

	return nil
}

// GetEntity retrieves an entity from the graph
func (ge *GraphEngine) GetEntity(ctx context.Context, documentID string, entityID int) (*models.Entity, error) {
	nodeID := fmt.Sprintf("entity:%s:%d", documentID, entityID)

	node, err := ge.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	// Convert GraphNode to Entity
	entity := &models.Entity{
		ID: entityID,
	}

	if node.Type != "" {
		entity.Type = models.EntityType(node.Type)
	}

	if text, ok := node.Properties["text"].(string); ok {
		entity.Text = text
	}

	return entity, nil
}

// GetEntitiesByType retrieves all entities of a specific type
func (ge *GraphEngine) GetEntitiesByType(ctx context.Context, entityType models.EntityType) ([]models.Entity, error) {
	// This would require a proper query implementation
	// For now, return empty list
	return []models.Entity{}, fmt.Errorf("GetEntitiesByType not yet implemented")
}

// GetRelationships retrieves all relationships for an entity
func (ge *GraphEngine) GetRelationships(ctx context.Context, documentID string, entityID int) ([]models.Relationship, error) {
	nodeID := fmt.Sprintf("entity:%s:%d", documentID, entityID)

	edges, err := ge.store.GetEdges(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get relationships: %w", err)
	}

	// Convert GraphEdges to Relationships
	relationships := make([]models.Relationship, 0, len(edges))
	for _, edge := range edges {
		// Parse entity IDs from node IDs
		// This is simplified - in production you'd parse properly
		relationships = append(relationships, models.Relationship{
			Subject:   entityID,
			Predicate: edge.Predicate,
			Object:    0, // Would need to parse from edge.To
		})
	}

	return relationships, nil
}

// DeleteDocument removes all graph data for a document
func (ge *GraphEngine) DeleteDocument(ctx context.Context, documentID string) error {
	if err := ge.store.DeleteDocumentData(ctx, documentID); err != nil {
		return fmt.Errorf("failed to delete document from graph: %w", err)
	}

	if ge.logger != nil {
		ge.logger.Info(ctx, "Deleted document from graph", "document_id", documentID)
	}

	if ge.metrics != nil {
		ge.metrics.IncrementCounter("graph.document_deleted", map[string]string{
			"document_id": documentID,
		})
	}

	return nil
}

// GetStats retrieves graph statistics
func (ge *GraphEngine) GetStats(ctx context.Context) (*models.GraphStats, error) {
	stats, err := ge.store.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph stats: %w", err)
	}

	return stats, nil
}

// HealthCheck performs a health check on the graph engine
func (ge *GraphEngine) HealthCheck(ctx context.Context) error {
	if err := ge.store.HealthCheck(ctx); err != nil {
		return fmt.Errorf("graph store health check failed: %w", err)
	}

	return nil
}

// QueryGraph executes a graph query
// This is a placeholder for future query functionality
func (ge *GraphEngine) QueryGraph(ctx context.Context, query string) ([]map[string]interface{}, error) {
	result, err := ge.store.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("graph query failed: %w", err)
	}

	return result, nil
}

// TraversePath traverses the graph starting from an entity
// This is a placeholder for future traversal functionality
func (ge *GraphEngine) TraversePath(ctx context.Context, startEntity string, predicate string, depth int) ([]models.Entity, error) {
	// This would implement graph traversal logic
	// For now, return empty list
	return []models.Entity{}, fmt.Errorf("TraversePath not yet implemented")
}

// FindConnectedEntities finds all entities connected to a given entity within a certain depth
// This is a placeholder for future functionality
func (ge *GraphEngine) FindConnectedEntities(ctx context.Context, documentID string, entityID int, maxDepth int) ([]models.Entity, error) {
	// This would implement breadth-first or depth-first search
	// For now, return empty list
	return []models.Entity{}, fmt.Errorf("FindConnectedEntities not yet implemented")
}
