package models

import (
	"encoding/json"
	"fmt"
)

// EntityType represents the type of an extracted entity
type EntityType string

const (
	EntityTypePerson       EntityType = "PERSON"
	EntityTypeOrganization EntityType = "ORGANIZATION"
	EntityTypeLocation     EntityType = "LOCATION"
	EntityTypeDate         EntityType = "DATE"
	EntityTypeEvent        EntityType = "EVENT"
	EntityTypeAnimal       EntityType = "ANIMAL"
	EntityTypeObject       EntityType = "OBJECT"
	EntityTypeOther        EntityType = "OTHER"
)

// ValidEntityTypes returns all valid entity types
func ValidEntityTypes() []EntityType {
	return []EntityType{
		EntityTypePerson,
		EntityTypeOrganization,
		EntityTypeLocation,
		EntityTypeDate,
		EntityTypeEvent,
		EntityTypeAnimal,
		EntityTypeObject,
		EntityTypeOther,
	}
}

// IsValid checks if the entity type is valid
func (et EntityType) IsValid() bool {
	for _, validType := range ValidEntityTypes() {
		if et == validType {
			return true
		}
	}
	return false
}

// Entity represents an extracted entity from text
type Entity struct {
	ID   int        `json:"id"`
	Type EntityType `json:"type"`
	Text string     `json:"text"`
}

// Validate checks if the entity is valid
func (e *Entity) Validate() error {
	if e.ID < 0 {
		return fmt.Errorf("entity ID must be non-negative, got %d", e.ID)
	}
	if e.Text == "" {
		return fmt.Errorf("entity text cannot be empty")
	}
	if !e.Type.IsValid() {
		return fmt.Errorf("invalid entity type: %s", e.Type)
	}
	return nil
}

// Relationship represents a relationship between two entities
type Relationship struct {
	Subject   int    `json:"subject"`   // Entity ID (subject)
	Predicate string `json:"predicate"` // Relationship type/verb
	Object    int    `json:"object"`    // Entity ID (object)
}

// Validate checks if the relationship is valid
func (r *Relationship) Validate() error {
	if r.Subject < 0 {
		return fmt.Errorf("subject ID must be non-negative, got %d", r.Subject)
	}
	if r.Object < 0 {
		return fmt.Errorf("object ID must be non-negative, got %d", r.Object)
	}
	if r.Predicate == "" {
		return fmt.Errorf("predicate cannot be empty")
	}
	return nil
}

// ExtractionResult contains extracted entities and relationships from text
type ExtractionResult struct {
	Entities      []Entity       `json:"entities"`
	Relationships []Relationship `json:"relationships"`
}

// Validate checks if the extraction result is valid
func (er *ExtractionResult) Validate() error {
	if er == nil {
		return fmt.Errorf("extraction result is nil")
	}

	// Validate entities and build ID map
	entityIDs := make(map[int]bool)
	for i, entity := range er.Entities {
		if err := entity.Validate(); err != nil {
			return fmt.Errorf("entity %d is invalid: %w", i, err)
		}
		if entityIDs[entity.ID] {
			return fmt.Errorf("duplicate entity ID: %d", entity.ID)
		}
		entityIDs[entity.ID] = true
	}

	// Validate relationships
	for i, rel := range er.Relationships {
		if err := rel.Validate(); err != nil {
			return fmt.Errorf("relationship %d is invalid: %w", i, err)
		}
		// Check that subject and object reference existing entities
		if !entityIDs[rel.Subject] {
			return fmt.Errorf("relationship %d references non-existent subject entity: %d", i, rel.Subject)
		}
		if !entityIDs[rel.Object] {
			return fmt.Errorf("relationship %d references non-existent object entity: %d", i, rel.Object)
		}
	}

	return nil
}

// EntityCount returns the number of entities
func (er *ExtractionResult) EntityCount() int {
	if er == nil {
		return 0
	}
	return len(er.Entities)
}

// RelationshipCount returns the number of relationships
func (er *ExtractionResult) RelationshipCount() int {
	if er == nil {
		return 0
	}
	return len(er.Relationships)
}

// GetEntity retrieves an entity by ID
func (er *ExtractionResult) GetEntity(id int) (*Entity, error) {
	if er == nil {
		return nil, fmt.Errorf("extraction result is nil")
	}
	for i := range er.Entities {
		if er.Entities[i].ID == id {
			return &er.Entities[i], nil
		}
	}
	return nil, fmt.Errorf("entity with ID %d not found", id)
}

// GetEntitiesByType returns all entities of a specific type
func (er *ExtractionResult) GetEntitiesByType(entityType EntityType) []Entity {
	if er == nil {
		return []Entity{}
	}

	var result []Entity
	for _, entity := range er.Entities {
		if entity.Type == entityType {
			result = append(result, entity)
		}
	}
	return result
}

// GetRelationshipsForEntity returns all relationships involving the given entity ID
func (er *ExtractionResult) GetRelationshipsForEntity(entityID int) []Relationship {
	if er == nil {
		return []Relationship{}
	}

	var result []Relationship
	for _, rel := range er.Relationships {
		if rel.Subject == entityID || rel.Object == entityID {
			result = append(result, rel)
		}
	}
	return result
}

// ToJSON converts the extraction result to JSON string
func (er *ExtractionResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(er, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal extraction result: %w", err)
	}
	return string(data), nil
}

// FromJSON creates an ExtractionResult from JSON string
func FromJSON(jsonStr string) (*ExtractionResult, error) {
	var result ExtractionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extraction result: %w", err)
	}
	return &result, nil
}

// NewExtractionResult creates an empty extraction result
func NewExtractionResult() *ExtractionResult {
	return &ExtractionResult{
		Entities:      []Entity{},
		Relationships: []Relationship{},
	}
}
