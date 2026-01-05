// Package entity provides LLM-based entity extraction for memories.
package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// EntityType represents the category of an extracted entity.
type EntityType string

const (
	TypePerson       EntityType = "person"
	TypeOrganization EntityType = "organization"
	TypeLocation     EntityType = "location"
	TypeConcept      EntityType = "concept"
	TypeTechnology   EntityType = "technology"
	TypeProject      EntityType = "project"
	TypeEvent        EntityType = "event"
	TypeOther        EntityType = "other"
)

// ExtractedEntity represents an entity extracted from text.
type ExtractedEntity struct {
	Name        string            `json:"name"`
	Type        EntityType        `json:"type"`
	Aliases     []string          `json:"aliases,omitempty"`
	Description string            `json:"description,omitempty"`
	Role        string            `json:"role,omitempty"`      // Role in the source text
	Confidence  float32           `json:"confidence,omitempty"` // 0-1 confidence score
	Meta        map[string]any    `json:"meta,omitempty"`
}

// ExtractedRelation represents a relationship between two entities.
type ExtractedRelation struct {
	SourceName   string `json:"source_name"`
	TargetName   string `json:"target_name"`
	RelationType string `json:"relation_type"`
}

// ExtractionResult holds the complete extraction output.
type ExtractionResult struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations,omitempty"`
}

// ChatProvider is the interface for LLM text completion.
type ChatProvider interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// Extractor uses an LLM to extract entities from text.
type Extractor struct {
	llm ChatProvider
}

// NewExtractor creates a new entity extractor.
func NewExtractor(llm ChatProvider) *Extractor {
	return &Extractor{llm: llm}
}

// Extract extracts entities and relations from the given text.
func (e *Extractor) Extract(ctx context.Context, text string) (*ExtractionResult, error) {
	prompt := buildExtractionPrompt(text)

	response, err := e.llm.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	result, err := parseExtractionResponse(response)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result, nil
}

// buildExtractionPrompt creates the prompt for entity extraction.
func buildExtractionPrompt(text string) string {
	return fmt.Sprintf(`Extract entities and relationships from the following text. Return a JSON object with:
- "entities": array of objects with "name", "type", "description" (optional), "role" (optional)
- "relations": array of objects with "source_name", "target_name", "relation_type"

Entity types: person, organization, location, concept, technology, project, event, other

Guidelines:
- Extract named entities (people, companies, places, technologies, projects)
- Include concepts if they are significant topics
- Identify relationships between entities when clear
- Use lowercase for relation_type (e.g., "works_at", "uses", "part_of", "created_by")
- Keep descriptions brief (under 50 words)

Text:
"""
%s
"""

Respond with ONLY valid JSON, no markdown or explanation:`, text)
}

// parseExtractionResponse parses the LLM response into structured entities.
func parseExtractionResponse(response string) (*ExtractionResult, error) {
	// Clean up response - remove markdown code blocks if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Handle empty or null responses
	if response == "" || response == "null" || response == "{}" {
		return &ExtractionResult{
			Entities:  []ExtractedEntity{},
			Relations: []ExtractedRelation{},
		}, nil
	}

	var result ExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Try to extract just entities if full parse fails
		var entitiesOnly struct {
			Entities []ExtractedEntity `json:"entities"`
		}
		if err2 := json.Unmarshal([]byte(response), &entitiesOnly); err2 == nil {
			return &ExtractionResult{
				Entities:  entitiesOnly.Entities,
				Relations: []ExtractedRelation{},
			}, nil
		}
		return nil, fmt.Errorf("unmarshal response: %w (response: %s)", err, truncate(response, 200))
	}

	// Validate and normalize entity types
	for i := range result.Entities {
		result.Entities[i].Type = normalizeEntityType(result.Entities[i].Type)
		if result.Entities[i].Confidence == 0 {
			result.Entities[i].Confidence = 1.0 // Default confidence
		}
	}

	return &result, nil
}

// normalizeEntityType ensures the entity type is valid.
func normalizeEntityType(t EntityType) EntityType {
	switch t {
	case TypePerson, TypeOrganization, TypeLocation, TypeConcept,
		TypeTechnology, TypeProject, TypeEvent, TypeOther:
		return t
	default:
		// Map common variations
		lower := EntityType(strings.ToLower(string(t)))
		switch lower {
		case "person", "people", "user", "name":
			return TypePerson
		case "organization", "org", "company", "team", "group":
			return TypeOrganization
		case "location", "place", "address", "city", "country":
			return TypeLocation
		case "concept", "idea", "topic", "theme":
			return TypeConcept
		case "technology", "tech", "tool", "framework", "language", "library":
			return TypeTechnology
		case "project", "repo", "repository", "product":
			return TypeProject
		case "event", "meeting", "deadline", "milestone", "date":
			return TypeEvent
		default:
			return TypeOther
		}
	}
}

// truncate shortens a string to the given length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
