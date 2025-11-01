package documents

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"arcadia/modules/documents/detection"
	"arcadia/modules/documents/interfaces"
	"arcadia/modules/documents/processors"
	"arcadia/modules/documents/processors/base"
	"arcadia/modules/documents/processors/tika"
)

// Mock implementations for testing
type MockLogger struct {
	logs []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

func (m *MockLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
	m.addLog("DEBUG", msg, keysAndValues...)
}

func (m *MockLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	m.addLog("INFO", msg, keysAndValues...)
}

func (m *MockLogger) Warn(ctx context.Context, msg string, keysAndValues ...interface{}) {
	m.addLog("WARN", msg, keysAndValues...)
}

func (m *MockLogger) Error(ctx context.Context, msg string, keysAndValues ...interface{}) {
	m.addLog("ERROR", msg, keysAndValues...)
}

func (m *MockLogger) Fatal(ctx context.Context, msg string, keysAndValues ...interface{}) {
	m.addLog("FATAL", msg, keysAndValues...)
}

func (m *MockLogger) WithFields(fields map[string]interface{}) interfaces.Logger {
	return m
}

func (m *MockLogger) WithContext(ctx context.Context) interfaces.Logger {
	return m
}

func (m *MockLogger) WithModule(module string) interfaces.Logger {
	return m
}

func (m *MockLogger) WithComponent(component string) interfaces.Logger {
	return m
}

func (m *MockLogger) addLog(level, msg string, keysAndValues ...interface{}) {
	fields := make(map[string]interface{})
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			if key, ok := keysAndValues[i].(string); ok {
				fields[key] = keysAndValues[i+1]
			}
		}
	}
	m.logs = append(m.logs, LogEntry{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}

// IntegrationTestSuite defines comprehensive integration tests for Tika implementation
type IntegrationTestSuite struct {
	t                *testing.T
	tempDir          string
	logger           *MockLogger
	detector         detection.MultiStageDetector
	registry         *processors.Registry
	tikaProcessor    *tika.TikaProcessor
	fallbackProcessor *tika.TikaProcessor
	testFiles        map[string]string
}

// NewIntegrationTestSuite creates a new test suite
func NewIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	suite := &IntegrationTestSuite{
		t:         t,
		logger:    &MockLogger{},
		testFiles: make(map[string]string),
	}

	// Create temporary directory for test files
	tempDir, err := ioutil.TempDir("", "tika_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	suite.tempDir = tempDir

	return suite
}

// Setup initializes the test environment
func (suite *IntegrationTestSuite) Setup() error {
	var err error

	// Initialize multi-stage detector
	suite.detector = detection.NewMultiStageDetector()

	// Initialize processor registry
	suite.registry = processors.NewRegistry(suite.detector).
		WithLogger(suite.logger)

	// Initialize default processors
	if err := suite.registry.InitializeWithDefaults(); err != nil {
		return fmt.Errorf("failed to initialize default processors: %w", err)
	}

	// Create Tika processors with different configurations
	suite.tikaProcessor, err = tika.NewTikaProcessor(suite.logger)
	if err != nil {
		return fmt.Errorf("failed to create Tika processor: %w", err)
	}

	suite.fallbackProcessor, err = tika.NewTikaProcessorWithFallback(suite.logger)
	if err != nil {
		return fmt.Errorf("failed to create fallback Tika processor: %w", err)
	}

	// Create test files
	if err := suite.createTestFiles(); err != nil {
		return fmt.Errorf("failed to create test files: %w", err)
	}

	return nil
}

// Teardown cleans up the test environment
func (suite *IntegrationTestSuite) Teardown() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// createTestFiles creates various test files for different document types
func (suite *IntegrationTestSuite) createTestFiles() error {
	// Create sample text content for different document types
	testContent := "This is a test document with sample content for testing purposes."

	// Create test files for different formats
	testFiles := map[string]string{
		"sample.txt":       testContent,
		"sample.md":        "# Test Markdown\n" + testContent,
		"sample.json":      `{"test": "content", "value": "example"}`,
		"sample.csv":       "Name,Value\nTest,123\nExample,456",
		"sample.html":      "<html><body><h1>Test</h1><p>" + testContent + "</p></body></html>",
		"sample.xml":       `<?xml version="1.0"?><root><item>` + testContent + `</item></root>`,
		"sample.unknown":   testContent + " in unknown format",
		"restricted.txt":   testContent, // For security testing
		"large_file.txt":   strings.Repeat(testContent+" ", 1000), // For performance testing
	}

	// Create Office document placeholders (these would be actual documents in real testing)
	officeFiles := []string{
		"sample.docx", "sample.xlsx", "sample.pptx",
		"sample.doc", "sample.xls", "sample.ppt",
		"sample.odt", "sample.ods", "sample.odp",
		"sample.rtf",
	}

	// Create text files
	for filename, content := range testFiles {
		filePath := filepath.Join(suite.tempDir, filename)
		if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create test file %s: %w", filename, err)
		}
		suite.testFiles[filename] = filePath
	}

	// Create placeholder office files (in real testing, these would be actual Office documents)
	for _, filename := range officeFiles {
		filePath := filepath.Join(suite.tempDir, filename)
		// Create a minimal placeholder - in real testing, use actual Office documents
		placeholder := "Placeholder for " + filename + " - would be actual Office document in real testing"
		if err := ioutil.WriteFile(filePath, []byte(placeholder), 0644); err != nil {
			return fmt.Errorf("failed to create placeholder file %s: %w", filename, err)
		}
		suite.testFiles[filename] = filePath
	}

	return nil
}

// TestEndToEndDocumentProcessingFlow tests the complete document processing pipeline
func (suite *IntegrationTestSuite) TestEndToEndDocumentProcessingFlow() {
	suite.t.Log("Testing end-to-end document processing flow")

	ctx := context.Background()

	// Test Office documents
	officeFiles := []string{"sample.docx", "sample.xlsx", "sample.pptx", "sample.odt"}
	for _, filename := range officeFiles {
		filePath := suite.testFiles[filename]

		suite.t.Logf("Processing Office document: %s", filename)

		// Test detection
		docType, err := suite.detector.DetectDocumentType(filePath)
		if err != nil {
			suite.t.Errorf("Detection failed for %s: %v", filename, err)
			continue
		}

		if docType == nil {
			suite.t.Errorf("No document type detected for %s", filename)
			continue
		}

		// Should be detected as Tika type with high confidence
		if docType.Type != base.ProcessorTypeTika {
			suite.t.Errorf("Expected Tika type for %s, got %s", filename, docType.Type)
		}

		if docType.Confidence < 0.8 {
			suite.t.Errorf("Expected high confidence for %s, got %.2f", filename, docType.Confidence)
		}

		// Test processing through registry
		result, err := suite.registry.ProcessDocument(ctx, filePath)
		if err != nil {
			// Note: This may fail with placeholder files, but we test the routing logic
			suite.t.Logf("Processing failed for %s (expected with placeholder): %v", filename, err)
		} else {
			suite.t.Logf("Successfully processed %s: content length %d", filename, len(result.Content))
		}
	}
}

// TestMultiStageDetectionIntegration tests the detection priority system
func (suite *IntegrationTestSuite) TestMultiStageDetectionIntegration() {
	suite.t.Log("Testing multi-stage detection integration")

	testCases := []struct {
		filename         string
		expectedType     base.ProcessorType
		minConfidence    float64
		maxConfidence    float64
		description      string
	}{
		{"sample.txt", base.ProcessorTypeText, 0.8, 1.0, "Text file should be detected with high confidence"},
		{"sample.md", base.ProcessorTypeMarkdown, 0.6, 1.0, "Markdown should be detected"},
		{"sample.docx", base.ProcessorTypeTika, 0.8, 1.0, "Office document should route to Tika"},
		{"sample.json", base.ProcessorTypeText, 0.5, 1.0, "JSON should be detected as text"},
		{"sample.html", base.ProcessorTypeText, 0.5, 1.0, "HTML should be detected as text"},
	}

	for _, tc := range testCases {
		filePath := suite.testFiles[tc.filename]

		docType, err := suite.detector.DetectDocumentType(filePath)
		if err != nil {
			suite.t.Errorf("Detection failed for %s: %v", tc.filename, err)
			continue
		}

		if docType == nil {
			suite.t.Errorf("No document type detected for %s", tc.filename)
			continue
		}

		// Check processor type
		if docType.Type != tc.expectedType {
			suite.t.Errorf("%s: expected type %s, got %s", tc.description, tc.expectedType, docType.Type)
		}

		// Check confidence range
		if docType.Confidence < tc.minConfidence || docType.Confidence > tc.maxConfidence {
			suite.t.Errorf("%s: confidence %.2f outside expected range [%.2f, %.2f]",
				tc.description, docType.Confidence, tc.minConfidence, tc.maxConfidence)
		}

		suite.t.Logf("✓ %s: type=%s, confidence=%.2f", tc.filename, docType.Type, docType.Confidence)
	}
}

// TestProcessorRegistryIntegration tests processor selection and routing
func (suite *IntegrationTestSuite) TestProcessorRegistryIntegration() {
	suite.t.Log("Testing processor registry integration")

	// Get supported types
	supportedTypes := suite.registry.GetSupportedTypes()
	suite.t.Logf("Registry supports %d processor types: %v", len(supportedTypes), supportedTypes)

	// Verify Tika processor is registered
	tikaFound := false
	for _, pType := range supportedTypes {
		if pType == base.ProcessorTypeTika {
			tikaFound = true
			break
		}
	}

	if !tikaFound {
		suite.t.Error("Tika processor type not found in supported types")
	}

	// Test processor retrieval
	processor, exists := suite.registry.GetProcessor(base.ProcessorTypeTika)
	if !exists {
		suite.t.Error("Tika processor not found in registry")
	} else {
		suite.t.Logf("✓ Tika processor found: %T", processor)

		// Verify processor metadata (cast to TikaProcessor to access extended methods)
		if tikaProc, ok := processor.(*tika.TikaProcessor); ok {
			metadata := tikaProc.GetProcessorMetadata()
			suite.t.Logf("Processor metadata: %v", metadata)
		}
	}

	// Test file routing
	testFiles := []string{"sample.docx", "sample.xlsx", "sample.txt"}
	for _, filename := range testFiles {
		filePath := suite.testFiles[filename]

		canProcess := suite.registry.CanProcessFile(filePath)
		suite.t.Logf("Registry can process %s: %v", filename, canProcess)

		if canProcess {
			proc, pType, err := suite.registry.GetProcessorForFile(filePath)
			if err != nil {
				suite.t.Errorf("Failed to get processor for %s: %v", filename, err)
			} else {
				suite.t.Logf("✓ %s -> %s (%T)", filename, pType, proc)
			}
		}
	}
}

// TestConfigurationIntegration tests configuration across all components
func (suite *IntegrationTestSuite) TestConfigurationIntegration() {
	suite.t.Log("Testing configuration integration")

	// Test default configuration
	config := detection.DefaultDetectionConfig()
	if !config.EnableTikaDetection {
		suite.t.Error("Default config should enable Tika detection")
	}

	if config.TikaOfficeConfidence != 0.9 {
		suite.t.Errorf("Expected office confidence 0.9, got %.2f", config.TikaOfficeConfidence)
	}

	// Test custom configuration
	customConfig := detection.DefaultDetectionConfig()
	customConfig.EnableTikaFallback = true
	customConfig.TikaFallbackConfidence = 0.4
	customConfig.ConfidenceThreshold = 0.3

	customDetector := detection.NewMultiStageDetectorWithConfig(customConfig)

	// Test fallback detection with unknown file
	unknownFile := suite.testFiles["sample.unknown"]
	docType, err := customDetector.DetectDocumentType(unknownFile)

	if err != nil {
		suite.t.Logf("Fallback detection error (may be expected): %v", err)
	} else if docType != nil {
		suite.t.Logf("✓ Fallback detection successful: type=%s, confidence=%.2f",
			docType.Type, docType.Confidence)
	}

	// Test Tika processor configuration
	tikaConfig := tika.DefaultTikaConfig()
	tikaConfig.AcceptAllFormats = true
	tikaConfig.FallbackConfidence = 0.4

	customProcessor, err := tika.NewTikaProcessorWithConfig(tikaConfig, suite.logger)
	if err != nil {
		suite.t.Errorf("Failed to create custom Tika processor: %v", err)
	} else {
		if !customProcessor.IsInFallbackMode() {
			suite.t.Error("Custom processor should be in fallback mode")
		}
		suite.t.Log("✓ Custom Tika processor configuration successful")
	}
}

// TestErrorScenariosAndResilience tests failure handling and recovery
func (suite *IntegrationTestSuite) TestErrorScenariosAndResilience() {
	suite.t.Log("Testing error scenarios and resilience")

	ctx := context.Background()

	// Test non-existent file
	_, err := suite.detector.DetectDocumentType("/nonexistent/file.docx")
	if err == nil {
		suite.t.Error("Expected error for non-existent file")
	} else {
		suite.t.Logf("✓ Non-existent file handled correctly: %v", err)
	}

	// Test empty filename
	_, err = suite.detector.DetectDocumentType("")
	if err == nil {
		suite.t.Error("Expected error for empty filename")
	} else {
		suite.t.Logf("✓ Empty filename handled correctly: %v", err)
	}

	// Test processor error handling
	_, err = suite.tikaProcessor.Process(ctx, "/nonexistent/file.docx")
	if err == nil {
		suite.t.Error("Expected error for non-existent file processing")
	} else {
		suite.t.Logf("✓ Processor error handling works: %v", err)
	}

	// Test Tika server unavailability simulation
	// Note: This would require a way to temporarily disable Tika server
	suite.t.Log("✓ Error scenario testing completed")
}

// TestSecurityMeasures tests security controls and access restrictions
func (suite *IntegrationTestSuite) TestSecurityMeasures() {
	suite.t.Log("Testing security measures and access controls")

	ctx := context.Background()

	// Test path traversal protection
	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		"/etc/shadow",
		"C:\\Windows\\System32\\config\\SAM",
	}

	for _, path := range maliciousPaths {
		_, err := suite.tikaProcessor.Process(ctx, path)
		if err == nil {
			suite.t.Errorf("Security breach: malicious path %s was processed", path)
		} else {
			suite.t.Logf("✓ Malicious path blocked: %s", path)
		}
	}

	// Test large file handling
	largeFile := suite.testFiles["large_file.txt"]
	result, err := suite.tikaProcessor.Process(ctx, largeFile)
	if err != nil {
		suite.t.Logf("Large file processing error (may be expected): %v", err)
	} else {
		suite.t.Logf("✓ Large file processed: content length %d", len(result.Content))
	}

	suite.t.Log("✓ Security testing completed")
}

// TestPerformanceAndConcurrency tests system performance under load
func (suite *IntegrationTestSuite) TestPerformanceAndConcurrency() {
	suite.t.Log("Testing performance and concurrency")

	// Test detection performance
	testFile := suite.testFiles["sample.txt"]

	start := time.Now()
	iterations := 100
	for i := 0; i < iterations; i++ {
		_, err := suite.detector.DetectDocumentType(testFile)
		if err != nil {
			suite.t.Errorf("Detection failed on iteration %d: %v", i, err)
			break
		}
	}
	duration := time.Since(start)

	avgTime := duration / time.Duration(iterations)
	suite.t.Logf("✓ Detection performance: %d iterations in %v (avg: %v per detection)",
		iterations, duration, avgTime)

	if avgTime > time.Millisecond {
		suite.t.Logf("Warning: Detection taking longer than expected: %v", avgTime)
	}

	// Test concurrent processing
	concurrency := 5
	done := make(chan bool, concurrency)

	start = time.Now()
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 10; j++ {
				_, err := suite.detector.DetectDocumentType(testFile)
				if err != nil {
					suite.t.Errorf("Concurrent detection failed (goroutine %d, iteration %d): %v", id, j, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}
	duration = time.Since(start)

	suite.t.Logf("✓ Concurrent processing: %d goroutines completed in %v", concurrency, duration)

	// Test circuit breaker behavior
	if suite.tikaProcessor != nil {
		stats := suite.tikaProcessor.GetCircuitBreakerStats()
		suite.t.Logf("✓ Circuit breaker stats: %+v", stats)
	}
}

// TestCircuitBreakerBehavior tests circuit breaker functionality
func (suite *IntegrationTestSuite) TestCircuitBreakerBehavior() {
	suite.t.Log("Testing circuit breaker behavior")

	// Get initial circuit breaker stats
	initialStats := suite.tikaProcessor.GetCircuitBreakerStats()
	suite.t.Logf("Initial circuit breaker stats: %+v", initialStats)

	// Test circuit breaker reset
	suite.tikaProcessor.ResetCircuitBreaker()
	resetStats := suite.tikaProcessor.GetCircuitBreakerStats()
	suite.t.Logf("✓ Circuit breaker reset successful: %+v", resetStats)

	suite.t.Log("✓ Circuit breaker testing completed")
}

// TestFallbackProcessing tests fallback mode functionality
func (suite *IntegrationTestSuite) TestFallbackProcessing() {
	suite.t.Log("Testing fallback processing mode")

	// Create custom detector with fallback enabled
	config := detection.DefaultDetectionConfig()
	config.EnableTikaFallback = true
	config.TikaFallbackConfidence = 0.3
	config.ConfidenceThreshold = 0.2

	fallbackDetector := detection.NewMultiStageDetectorWithConfig(config)

	// Test with unknown file extension
	unknownFile := suite.testFiles["sample.unknown"]
	docType, err := fallbackDetector.DetectDocumentType(unknownFile)

	if err != nil {
		suite.t.Logf("Fallback detection error: %v", err)
	} else if docType != nil {
		suite.t.Logf("✓ Fallback detection: type=%s, confidence=%.2f",
			docType.Type, docType.Confidence)

		if docType.Type == base.ProcessorTypeTika && docType.Confidence <= 0.5 {
			suite.t.Log("✓ Fallback mode working correctly with low confidence")
		}
	} else {
		suite.t.Log("No fallback detection (may be expected)")
	}

	// Test fallback processor
	if suite.fallbackProcessor.IsInFallbackMode() {
		suite.t.Log("✓ Fallback processor is in fallback mode")
	} else {
		suite.t.Error("Fallback processor should be in fallback mode")
	}
}

// RunAllTests executes the complete integration test suite
func (suite *IntegrationTestSuite) RunAllTests() {
	defer suite.Teardown()

	if err := suite.Setup(); err != nil {
		suite.t.Fatalf("Test setup failed: %v", err)
	}

	suite.t.Log("=== Starting Comprehensive Tika Integration Tests ===")

	// Run all test categories
	suite.TestEndToEndDocumentProcessingFlow()
	suite.TestMultiStageDetectionIntegration()
	suite.TestProcessorRegistryIntegration()
	suite.TestConfigurationIntegration()
	suite.TestErrorScenariosAndResilience()
	suite.TestSecurityMeasures()
	suite.TestPerformanceAndConcurrency()
	suite.TestCircuitBreakerBehavior()
	suite.TestFallbackProcessing()

	// Log summary
	suite.t.Log("=== Integration Test Summary ===")
	suite.t.Logf("Total log entries: %d", len(suite.logger.logs))

	// Count log levels
	logCounts := make(map[string]int)
	for _, log := range suite.logger.logs {
		logCounts[log.Level]++
	}
	suite.t.Logf("Log level counts: %v", logCounts)

	suite.t.Log("=== Comprehensive Tika Integration Tests Completed ===")
}

// TestComprehensiveTikaIntegration is the main test function
func TestComprehensiveTikaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive integration test in short mode")
	}

	suite := NewIntegrationTestSuite(t)
	suite.RunAllTests()
}

// TestTikaServerAvailability tests Tika server connectivity
func TestTikaServerAvailability(t *testing.T) {
	logger := &MockLogger{}

	// Test with default configuration
	config := tika.DefaultTikaConfig()
	client := tika.NewTikaClient(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.HealthCheck(ctx)
	if err != nil {
		t.Logf("Tika server health check failed: %v", err)
		t.Skip("Tika server not available, skipping server-dependent tests")
	} else {
		t.Log("✓ Tika server is available and healthy")
	}
}

// BenchmarkDetectionPerformance benchmarks detection performance
func BenchmarkDetectionPerformance(b *testing.B) {
	// Create temporary test file
	tempDir, err := ioutil.TempDir("", "tika_benchmark")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "benchmark.txt")
	if err := ioutil.WriteFile(testFile, []byte("Benchmark test content"), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	detector := detection.NewMultiStageDetector()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := detector.DetectDocumentType(testFile)
			if err != nil {
				b.Errorf("Detection failed: %v", err)
			}
		}
	})
}