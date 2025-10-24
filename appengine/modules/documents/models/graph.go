package models

import (
	"encoding/json"
	"fmt"
)

// Quad represents a graph quad (subject, predicate, object, label)
// This is the fundamental unit of storage in a quad store
type Quad struct {
	Subject   string      `json:"subject"`   // The subject node (e.g., "entity:doc123:1")
	Predicate string      `json:"predicate"` // The relationship type (e.g., "type", "works_at")
	Object    interface{} `json:"object"`    // Can be string value or another node ID
	Label     string      `json:"label"`     // Context/provenance (usually document ID)
}

// Validate checks if the quad is valid
func (q *Quad) Validate() error {
	if q.Subject == "" {
		return fmt.Errorf("quad subject cannot be empty")
	}
	if q.Predicate == "" {
		return fmt.Errorf("quad predicate cannot be empty")
	}
	if q.Object == nil {
		return fmt.Errorf("quad object cannot be nil")
	}
	return nil
}

// GraphNode represents a node in the graph with its properties
type GraphNode struct {
	ID         string                 `json:"id"`         // Unique node identifier
	Type       string                 `json:"type"`       // Node type (e.g., "PERSON", "ORGANIZATION")
	Properties map[string]interface{} `json:"properties"` // Additional properties
}

// GraphEdge represents an edge (relationship) in the graph
type GraphEdge struct {
	From      string `json:"from"`      // Source node ID
	Predicate string `json:"predicate"` // Relationship type
	To        string `json:"to"`        // Target node ID
	Label     string `json:"label"`     // Context (usually document ID)
}

// GraphQueryResult represents the result of a graph query
type GraphQueryResult struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphStats contains statistics about the graph database
type GraphStats struct {
	EntityCount       int            `json:"entity_count"`       // Total number of entities
	RelationshipCount int            `json:"relationship_count"` // Total number of relationships
	DocumentCount     int            `json:"document_count"`     // Number of documents with graph data
	TypeDistribution  map[string]int `json:"type_distribution"`  // Count by entity type
	QuadCount         int64          `json:"quad_count"`         // Total quads in database
}

// EntityGraphData represents entity data ready for graph storage
type EntityGraphData struct {
	DocumentID string   `json:"document_id"`
	EntityID   int      `json:"entity_id"`
	Type       string   `json:"type"`
	Text       string   `json:"text"`
	Quads      []Quad   `json:"quads"`
}

// RelationshipGraphData represents relationship data ready for graph storage
type RelationshipGraphData struct {
	DocumentID string `json:"document_id"`
	SubjectID  int    `json:"subject_id"`
	Predicate  string `json:"predicate"`
	ObjectID   int    `json:"object_id"`
	Quad       Quad   `json:"quad"`
}

// ToJSON converts a quad to JSON string
func (q *Quad) ToJSON() (string, error) {
	data, err := json.Marshal(q)
	if err != nil {
		return "", fmt.Errorf("failed to marshal quad: %w", err)
	}
	return string(data), nil
}

// QuadFromJSON creates a Quad from JSON string
func QuadFromJSON(jsonStr string) (*Quad, error) {
	var quad Quad
	if err := json.Unmarshal([]byte(jsonStr), &quad); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quad: %w", err)
	}
	return &quad, nil
}

// NewGraphStats creates an empty GraphStats
func NewGraphStats() *GraphStats {
	return &GraphStats{
		EntityCount:       0,
		RelationshipCount: 0,
		DocumentCount:     0,
		TypeDistribution:  make(map[string]int),
		QuadCount:         0,
	}
}

// MakeEntityQuads creates quads for an entity
func MakeEntityQuads(documentID string, entity Entity) []Quad {
	entityID := fmt.Sprintf("entity:%s:%d", documentID, entity.ID)

	quads := []Quad{
		{
			Subject:   entityID,
			Predicate: "type",
			Object:    string(entity.Type),
			Label:     documentID,
		},
		{
			Subject:   entityID,
			Predicate: "text",
			Object:    entity.Text,
			Label:     documentID,
		},
		{
			Subject:   entityID,
			Predicate: "extracted_from",
			Object:    documentID,
			Label:     documentID,
		},
		{
			Subject:   entityID,
			Predicate: "entity_id",
			Object:    fmt.Sprintf("%d", entity.ID),
			Label:     documentID,
		},
	}

	return quads
}

// MakeRelationshipQuad creates a quad for a relationship
func MakeRelationshipQuad(documentID string, relationship Relationship) Quad {
	subjectID := fmt.Sprintf("entity:%s:%d", documentID, relationship.Subject)
	objectID := fmt.Sprintf("entity:%s:%d", documentID, relationship.Object)

	return Quad{
		Subject:   subjectID,
		Predicate: relationship.Predicate,
		Object:    objectID,
		Label:     documentID,
	}
}
