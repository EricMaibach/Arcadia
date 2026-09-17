package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// createTestDatabase creates an in-memory SQLite database for testing
func createTestDatabase() (Database, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &SQLiteDatabase{db: db}, nil
}

func TestNewFileWatcherService(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	if service == nil {
		t.Fatal("Service should not be nil")
	}

	if service.IsRunning() {
		t.Error("Service should not be running initially")
	}
}

func TestFileWatcherService_StartStop(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	// Test start
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if !service.IsRunning() {
		t.Error("Service should be running after start")
	}

	// Test start again (should fail)
	err = service.Start()
	if err == nil {
		t.Error("Starting an already running service should return an error")
	}

	// Test stop
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	if service.IsRunning() {
		t.Error("Service should not be running after stop")
	}

	// Test stop again (should fail)
	err = service.Stop()
	if err == nil {
		t.Error("Stopping a non-running service should return an error")
	}
}

func TestFileWatcherService_AddRemoveDirectory(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filewatcher_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test add directory
	err = service.AddDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to add directory: %v", err)
	}

	// Verify directory was added
	dirs, err := service.GetWatchedDirectories()
	if err != nil {
		t.Fatalf("Failed to get watched directories: %v", err)
	}

	absPath, _ := filepath.Abs(tempDir)
	found := false
	for _, dir := range dirs {
		if dir == absPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Directory %s should be in watched list", absPath)
	}

	// Test remove directory
	err = service.RemoveDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to remove directory: %v", err)
	}

	// Verify directory was removed
	dirs, err = service.GetWatchedDirectories()
	if err != nil {
		t.Fatalf("Failed to get watched directories: %v", err)
	}

	for _, dir := range dirs {
		if dir == absPath {
			t.Errorf("Directory %s should not be in watched list after removal", absPath)
		}
	}

	// Test remove non-existent directory
	err = service.RemoveDirectory("/nonexistent/path")
	if err == nil {
		t.Error("Removing non-existent directory should return an error")
	}
}

func TestFileWatcherService_EventHandlers(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	// Test event handler registration
	var eventReceived bool
	var receivedEvent FileEvent
	var wg sync.WaitGroup

	handler := func(event FileEvent) {
		eventReceived = true
		receivedEvent = event
		wg.Done()
	}

	service.RegisterEventHandler(handler)

	// Start the service
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()

	// Access internal fields directly since service is already *FileWatcherService
	fws := service

	// Simulate an event by directly sending to the buffer
	wg.Add(1)
	testEvent := FileEvent{
		Path:      "/test/path",
		Operation: "create",
		Timestamp: time.Now(),
		IsDir:     false,
	}

	// Send event to buffer (this tests the distribution mechanism)
	select {
	case fws.eventBuffer <- testEvent:
		// Event sent successfully
	default:
		t.Fatal("Event buffer is full")
	}

	// Wait for event to be processed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Event processed
	case <-time.After(1 * time.Second):
		t.Fatal("Event handler was not called within timeout")
	}

	if !eventReceived {
		t.Error("Event handler should have been called")
	}

	if receivedEvent.Path != testEvent.Path {
		t.Errorf("Expected event path %s, got %s", testEvent.Path, receivedEvent.Path)
	}

	if receivedEvent.Operation != testEvent.Operation {
		t.Errorf("Expected event operation %s, got %s", testEvent.Operation, receivedEvent.Operation)
	}
}

func TestFileWatcherService_GetStats(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	fws := service

	stats := fws.GetStats()

	if stats.IsRunning {
		t.Error("Service should not be running initially")
	}

	if stats.EventBufferCapacity != config.BufferSize {
		t.Errorf("Expected buffer capacity %d, got %d", config.BufferSize, stats.EventBufferCapacity)
	}

	if stats.RegisteredHandlers != 0 {
		t.Errorf("Expected 0 registered handlers, got %d", stats.RegisteredHandlers)
	}
}

func TestFileWatcherService_MapOperation(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	fws := service

	// Test the mapOperation method
	// Note: We can't easily test the actual fsnotify.Op values in a unit test
	// This is more of a structure test to ensure the function exists
	result := fws.mapOperation(0) // Using 0 as a placeholder
	if result == "" {
		t.Error("mapOperation should return a non-empty string")
	}
}

func TestFileWatcherService_ShouldDebounce(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := FileWatcherConfig{
		BufferSize:    100,
		DebounceDelay: 50 * time.Millisecond,
	}

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	fws := service

	testPath := "/test/file.txt"

	// First call should not be debounced
	if fws.shouldDebounce(testPath) {
		t.Error("First call should not be debounced")
	}

	// Immediate second call should be debounced
	if !fws.shouldDebounce(testPath) {
		t.Error("Immediate second call should be debounced")
	}

	// After delay, should not be debounced
	time.Sleep(60 * time.Millisecond)
	if fws.shouldDebounce(testPath) {
		t.Error("Call after delay should not be debounced")
	}
}

func TestFileWatcherRepository_Metadata(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	// Initialize the database schema first
	config := DefaultFileWatcherConfig()
	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	service.Close() // We just needed it to initialize the schema

	repo := NewFileWatcherRepository(testDB)

	testPath := "/test/path"

	// Test getting metadata for non-existent path
	_, err = repo.GetWatchedDirectoryMetadata(testPath)
	if err == nil {
		t.Error("Getting metadata for non-existent path should return an error")
	}

	// Test updating metadata for non-existent path
	testMetadata := map[string]interface{}{
		"priority": "high",
		"tags":     []string{"important", "monitor"},
	}
	err = repo.UpdateWatchedDirectoryMetadata(testPath, testMetadata)
	if err == nil {
		t.Error("Updating metadata for non-existent path should return an error")
	}
}

func TestDefaultFileWatcherConfig(t *testing.T) {
	config := DefaultFileWatcherConfig()

	if config.BufferSize <= 0 {
		t.Error("Default buffer size should be positive")
	}

	if config.DebounceDelay <= 0 {
		t.Error("Default debounce delay should be positive")
	}
}

func TestFileWatcherService_GlobalInstance(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	// Reset global instance for test
	globalFileWatcher = nil
	globalFileWatcherOnce = sync.Once{}

	err = InitGlobalFileWatcher(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to initialize global file watcher: %v", err)
	}

	instance := GetGlobalFileWatcher()
	if instance == nil {
		t.Error("Global file watcher instance should not be nil")
	}

	// Test getting instance again (should be same)
	instance2 := GetGlobalFileWatcher()
	if instance != instance2 {
		t.Error("Global file watcher should return same instance")
	}
}

func TestFileWatcherService_HandlerPanicRecovery(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	// Register a handler that panics
	panicHandler := func(event FileEvent) {
		panic("test panic")
	}

	// Register a normal handler to ensure it still works
	var normalHandlerCalled bool
	var wg sync.WaitGroup

	normalHandler := func(event FileEvent) {
		normalHandlerCalled = true
		wg.Done()
	}

	service.RegisterEventHandler(panicHandler)
	service.RegisterEventHandler(normalHandler)

	// Start the service
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()

	// Access internal fields directly since service is already *FileWatcherService
	fws := service

	// Send an event
	wg.Add(1)
	testEvent := FileEvent{
		Path:      "/test/panic",
		Operation: "create",
		Timestamp: time.Now(),
		IsDir:     false,
	}

	select {
	case fws.eventBuffer <- testEvent:
		// Event sent successfully
	default:
		t.Fatal("Event buffer is full")
	}

	// Wait for normal handler to be called
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Event processed
	case <-time.After(1 * time.Second):
		t.Fatal("Normal handler was not called within timeout")
	}

	if !normalHandlerCalled {
		t.Error("Normal handler should have been called despite panic in other handler")
	}
}

func TestFileWatcherService_MultipleDirectories(t *testing.T) {
	testDB, err := createTestDatabase()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	// Create multiple temporary directories
	tempDirs := make([]string, 3)
	for i := range tempDirs {
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("filewatcher_test_%d", i))
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		tempDirs[i] = tempDir
		defer os.RemoveAll(tempDir)
	}

	// Add all directories
	for _, dir := range tempDirs {
		err = service.AddDirectory(dir)
		if err != nil {
			t.Fatalf("Failed to add directory %s: %v", dir, err)
		}
	}

	// Verify all directories are being watched
	watchedDirs, err := service.GetWatchedDirectories()
	if err != nil {
		t.Fatalf("Failed to get watched directories: %v", err)
	}

	if len(watchedDirs) != len(tempDirs) {
		t.Errorf("Expected %d watched directories, got %d", len(tempDirs), len(watchedDirs))
	}

	// Remove one directory
	err = service.RemoveDirectory(tempDirs[1])
	if err != nil {
		t.Fatalf("Failed to remove directory: %v", err)
	}

	// Verify directory was removed
	watchedDirs, err = service.GetWatchedDirectories()
	if err != nil {
		t.Fatalf("Failed to get watched directories: %v", err)
	}

	if len(watchedDirs) != len(tempDirs)-1 {
		t.Errorf("Expected %d watched directories after removal, got %d", len(tempDirs)-1, len(watchedDirs))
	}
}

// Benchmark tests
func BenchmarkFileWatcherService_AddDirectory(b *testing.B) {
	testDB, err := createTestDatabase()
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(b))
	if err != nil {
		b.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "filewatcher_bench")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testDir := filepath.Join(tempDir, fmt.Sprintf("test_%d", i))
		os.MkdirAll(testDir, 0755)
		service.AddDirectory(testDir)
	}
}

func BenchmarkFileWatcherService_EventProcessing(b *testing.B) {
	testDB, err := createTestDatabase()
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	config := DefaultFileWatcherConfig()

	service, err := NewFileWatcherService(testDB, config, newTestLogger(b))
	if err != nil {
		b.Fatalf("Failed to create file watcher service: %v", err)
	}
	defer service.Close()

	fws := service

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := fmt.Sprintf("/test/file_%d.txt", i)
		fws.shouldDebounce(path)
	}
}
