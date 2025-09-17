package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

// App structure for dynamic tools (from registry.go and claude.go)
type App struct {
	AppID          string    `json:"appId"`
	Version        string    `json:"version"`
	Runtime        string    `json:"runtime"`
	Tools          []AppTool `json:"tools"`
	ArtifactURI    string    `json:"artifactUri"`
	SourceLanguage string    `json:"sourceLanguage,omitempty"`
	Files          []File    `json:"files,omitempty"`
}

type AppTool struct {
	Name        string `json:"name"`
	InputFormat string `json:"inputFormat"`
}

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// Document search types (referenced in claude.go)
type DocumentSearchResult struct {
	Document      *Document      `json:"document"`
	Chunks        []*ChunkResult `json:"chunks"`
	BestScore     float32        `json:"best_score"`
	TotalChunks   int            `json:"total_chunks"`
	RelevanceRank int            `json:"relevance_rank"`
}

type EnhancedDocumentSearchResult struct {
	Document          *Document `json:"document"`
	ContextHighlights []string  `json:"context_highlights"`
	ContentPreview    string    `json:"content_preview"`
	IsTruncated       bool      `json:"is_truncated"`
	BestScore         float32   `json:"best_score"`
	RelevanceRank     int       `json:"relevance_rank"`
}

type Document struct {
	ID         string         `json:"id"`
	FilePath   string         `json:"file_path"`
	FileHash   string         `json:"file_hash"`
	Content    string         `json:"content"`
	ChunkCount int            `json:"chunk_count"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type ChunkResult struct {
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
	ChunkIndex int     `json:"chunk_index"`
}

// Schedule related types (from claude.go and scheduler.go)
type ScheduleRequest struct {
	AppID         string          `json:"appId"`
	ToolName      string          `json:"toolName"`
	Input         json.RawMessage `json:"input"`
	ScheduleType  string          `json:"scheduleType"`
	ScheduledTime FlexTime        `json:"scheduledTime"`
	Recurrence    *RecurrenceRule `json:"recurrence,omitempty"`
}

type FlexTime struct {
	Time time.Time
}

// UnmarshalJSON implements json.Unmarshaler for FlexTime
func (ft *FlexTime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	// Remove quotes from JSON string
	timeStr := string(b)
	if len(timeStr) > 1 && timeStr[0] == '"' && timeStr[len(timeStr)-1] == '"' {
		timeStr = timeStr[1 : len(timeStr)-1]
	}

	// Try multiple common time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			ft.Time = t
			return nil
		}
	}

	return fmt.Errorf("unable to parse time: %s", timeStr)
}

type RecurrenceRule struct {
	Interval   int        `json:"interval"`
	Unit       string     `json:"unit"` // "minutes", "hours", "days", "weeks", "months"
	DaysOfWeek []int      `json:"daysOfWeek,omitempty"`
	EndDate    *time.Time `json:"endDate,omitempty"`
}

type Schedule struct {
	ID      string    `json:"id"`
	NextRun time.Time `json:"next_run"`
}

// Define these constants that are referenced in claude.go
const (
	ScheduleTypeOneTime   = "one-time"
	ScheduleTypeRecurring = "recurring"
)

// SearchConfig interface for enhanced search
type SearchConfig interface{}

// DefaultSearchConfig provides a default search configuration
func DefaultSearchConfig() SearchConfig {
	return struct{}{}
}

// Placeholder functions referenced in claude.go - these will be implemented elsewhere
func CreateSchedule(req ScheduleRequest) (*Schedule, error) {
	// This would be implemented elsewhere, just define the interface
	return nil, fmt.Errorf("not implemented")
}

func GetAllSchedules(appID string) []Schedule {
	// This would be implemented elsewhere, just define the interface
	return []Schedule{}
}

// Note: EmbeddingService will be defined elsewhere and injected via interfaces
