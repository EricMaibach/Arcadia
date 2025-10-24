package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/models"
	"arcadia/modules/documents/providers"
)

// Default prompt template for entity and relationship extraction
const defaultExtractionPrompt = `You are an information extraction system. Extract entities and relationships from the given text and return JSON matching this exact schema:

{
  "entities": [
    {"id": "integer", "type": "PERSON | ORGANIZATION | LOCATION | DATE | EVENT | ANIMAL | OBJECT | OTHER", "text": "string"}
  ],
  "relationships": [
    {"subject": "entity_id", "predicate": "string", "object": "entity_id"}
  ]
}

Return only valid JSON. Do not add explanations or markdown.

Now extract from this text:

`

// ExtractionEngine handles entity and relationship extraction from text using LLMs
type ExtractionEngine struct {
	ollama         *providers.OllamaProvider
	modelName      string
	promptTemplate string
	logger         interfaces.Logger
	metrics        interfaces.MetricsCollector
	options        map[string]interface{} // Ollama generation options
}

// ExtractionConfig contains configuration for the extraction engine
type ExtractionConfig struct {
	ModelName      string                 `json:"model_name"`
	PromptTemplate string                 `json:"prompt_template"`
	Temperature    float64                `json:"temperature"`
	MaxTokens      int                    `json:"max_tokens"`
	Options        map[string]interface{} `json:"options"`
}

// NewExtractionEngine creates a new extraction engine
func NewExtractionEngine(ollama *providers.OllamaProvider, modelName string, config ExtractionConfig) *ExtractionEngine {
	if modelName == "" {
		modelName = "llama3:8b"
	}

	promptTemplate := config.PromptTemplate
	if promptTemplate == "" {
		promptTemplate = defaultExtractionPrompt
	}

	// Default Ollama options for extraction
	// Low temperature for structured output consistency
	options := map[string]interface{}{
		"temperature": 0.1,
		"num_predict": 2048,
	}

	// Override with temperature from config if provided
	if config.Temperature > 0 {
		options["temperature"] = config.Temperature
	}

	// Override with max tokens if provided
	if config.MaxTokens > 0 {
		options["num_predict"] = config.MaxTokens
	}

	// Merge any custom options from config
	for k, v := range config.Options {
		options[k] = v
	}

	return &ExtractionEngine{
		ollama:         ollama,
		modelName:      modelName,
		promptTemplate: promptTemplate,
		options:        options,
	}
}

// WithLogger adds logging support to the extraction engine
func (ee *ExtractionEngine) WithLogger(logger interfaces.Logger) *ExtractionEngine {
	ee.logger = logger
	return ee
}

// WithMetrics adds metrics collection support to the extraction engine
func (ee *ExtractionEngine) WithMetrics(metrics interfaces.MetricsCollector) *ExtractionEngine {
	ee.metrics = metrics
	return ee
}

// ExtractEntities extracts entities and relationships from the given text
func (ee *ExtractionEngine) ExtractEntities(ctx context.Context, text string) (*models.ExtractionResult, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if ee.metrics != nil {
			duration := time.Since(startTime).Seconds() * 1000
			ee.metrics.RecordTimer("extraction.engine.duration", duration, map[string]string{
				"model": ee.modelName,
			})
		}
	}()

	if ee.logger != nil {
		ee.logger.Debug(ctx, "Starting entity extraction",
			"model", ee.modelName,
			"text_length", len(text))
	}

	// Build the full prompt with the text
	prompt := ee.buildPrompt(text)

	// Call Ollama generate endpoint with our model and options
	response, err := ee.ollama.CallGenerate(ctx, ee.modelName, prompt, ee.options)
	if err != nil {
		if ee.metrics != nil {
			ee.metrics.IncrementCounter("extraction.error", map[string]string{
				"model": ee.modelName,
				"type":  "ollama_call",
			})
		}
		return nil, fmt.Errorf("failed to call Ollama for extraction: %w", err)
	}

	if ee.logger != nil {
		ee.logger.Debug(ctx, "Received extraction response",
			"response_length", len(response))
	}

	// Parse the JSON response
	result, err := ee.parseResponse(response)
	if err != nil {
		if ee.metrics != nil {
			ee.metrics.IncrementCounter("extraction.error", map[string]string{
				"model": ee.modelName,
				"type":  "parse",
			})
		}
		if ee.logger != nil {
			ee.logger.Warn(ctx, "Failed to parse extraction response",
				"error", err,
				"response", response)
		}
		return nil, fmt.Errorf("failed to parse extraction response: %w", err)
	}

	// Validate the result
	if err := result.Validate(); err != nil {
		if ee.metrics != nil {
			ee.metrics.IncrementCounter("extraction.error", map[string]string{
				"model": ee.modelName,
				"type":  "validation",
			})
		}
		return nil, fmt.Errorf("invalid extraction result: %w", err)
	}

	// Record success metrics
	if ee.metrics != nil {
		ee.metrics.IncrementCounter("extraction.success", map[string]string{
			"model": ee.modelName,
		})
		ee.metrics.SetGauge("extraction.entities.count", float64(len(result.Entities)), map[string]string{
			"model": ee.modelName,
		})
		ee.metrics.SetGauge("extraction.relationships.count", float64(len(result.Relationships)), map[string]string{
			"model": ee.modelName,
		})
	}

	if ee.logger != nil {
		ee.logger.Info(ctx, "Successfully extracted entities and relationships",
			"model", ee.modelName,
			"entity_count", len(result.Entities),
			"relationship_count", len(result.Relationships))
	}

	return result, nil
}

// buildPrompt constructs the full prompt by appending the text to the template
func (ee *ExtractionEngine) buildPrompt(text string) string {
	return ee.promptTemplate + "\n\n" + text
}

// parseResponse parses the LLM response into an ExtractionResult
func (ee *ExtractionEngine) parseResponse(response string) (*models.ExtractionResult, error) {
	// Clean up the response
	// Sometimes LLMs add markdown code blocks or explanations
	cleaned := strings.TrimSpace(response)

	// Remove markdown code blocks if present
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Try to find JSON if there's extra text
	// Look for the first { and last }
	startIdx := strings.Index(cleaned, "{")
	endIdx := strings.LastIndex(cleaned, "}")

	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return nil, fmt.Errorf("no valid JSON object found in response")
	}

	jsonStr := cleaned[startIdx : endIdx+1]

	// Parse the JSON
	var result models.ExtractionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w (json: %s)", err, jsonStr)
	}

	return &result, nil
}

// GetModelName returns the model name being used for extraction
func (ee *ExtractionEngine) GetModelName() string {
	return ee.modelName
}

// GetPromptTemplate returns the prompt template being used
func (ee *ExtractionEngine) GetPromptTemplate() string {
	return ee.promptTemplate
}

// HealthCheck performs a health check on the extraction engine
func (ee *ExtractionEngine) HealthCheck(ctx context.Context) error {
	// Test with a simple text
	testText := "John works at Google in California."
	_, err := ee.ExtractEntities(ctx, testText)
	return err
}

// DefaultExtractionConfig returns a default extraction configuration
func DefaultExtractionConfig() ExtractionConfig {
	return ExtractionConfig{
		ModelName:      "llama3:8b",
		PromptTemplate: defaultExtractionPrompt,
		Temperature:    0.1, // Low temperature for structured output
		MaxTokens:      2048,
		Options:        map[string]interface{}{},
	}
}
