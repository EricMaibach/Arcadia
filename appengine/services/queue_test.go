package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// MockQueueRepository for testing
type MockQueueRepository struct {
	items       map[string]*QueueItem
	mutex       sync.RWMutex
	shouldError bool
	errorMsg    string
}

func NewMockQueueRepository() *MockQueueRepository {
	return &MockQueueRepository{
		items: make(map[string]*QueueItem),
	}
}

func (m *MockQueueRepository) SetError(shouldError bool, errorMsg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldError = shouldError
	m.errorMsg = errorMsg
}

func (m *MockQueueRepository) EnqueueItem(item *QueueItem) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMsg)
	}

	m.items[item.ID] = item
	return nil
}

func (m *MockQueueRepository) DequeueItem() (*QueueItem, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock error: %s", m.errorMsg)
	}

	// Find highest priority pending item
	var best *QueueItem
	for _, item := range m.items {
		if item.Status == "pending" {
			if best == nil || item.Priority > best.Priority ||
				(item.Priority == best.Priority && item.CreatedAt.Before(best.CreatedAt)) {
				best = item
			}
		}
	}

	if best == nil {
		return nil, nil
	}

	// Update status (attempts will be incremented by the service)
	best.Status = "processing"
	now := time.Now()
	best.ProcessedAt = &now

	return best, nil
}

func (m *MockQueueRepository) PeekNext() (*QueueItem, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock error: %s", m.errorMsg)
	}

	// Find highest priority pending item
	var best *QueueItem
	for _, item := range m.items {
		if item.Status == "pending" {
			if best == nil || item.Priority > best.Priority ||
				(item.Priority == best.Priority && item.CreatedAt.Before(best.CreatedAt)) {
				best = item
			}
		}
	}

	return best, nil
}

func (m *MockQueueRepository) GetItemByID(id string) (*QueueItem, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock error: %s", m.errorMsg)
	}

	item, exists := m.items[id]
	if !exists {
		return nil, nil
	}
	return item, nil
}

func (m *MockQueueRepository) UpdateItem(item *QueueItem) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMsg)
	}

	m.items[item.ID] = item
	return nil
}

func (m *MockQueueRepository) DeleteItem(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMsg)
	}

	delete(m.items, id)
	return nil
}

func (m *MockQueueRepository) GetQueueLength() (int, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.shouldError {
		return 0, fmt.Errorf("mock error: %s", m.errorMsg)
	}

	count := 0
	for _, item := range m.items {
		if item.Status == "pending" {
			count++
		}
	}
	return count, nil
}

func (m *MockQueueRepository) GetPendingItems(limit int) ([]*QueueItem, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock error: %s", m.errorMsg)
	}

	var pending []*QueueItem
	for _, item := range m.items {
		if item.Status == "pending" {
			pending = append(pending, item)
		}
	}

	if limit > 0 && len(pending) > limit {
		pending = pending[:limit]
	}

	return pending, nil
}

func (m *MockQueueRepository) GetAllItems() ([]*QueueItem, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock error: %s", m.errorMsg)
	}

	var all []*QueueItem
	for _, item := range m.items {
		all = append(all, item)
	}
	return all, nil
}

func (m *MockQueueRepository) ClearQueue() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMsg)
	}

	m.items = make(map[string]*QueueItem)
	return nil
}

// Test helper functions
func createTestQueueService() *QueueService {
	repo := NewMockQueueRepository()
	return NewQueueService(repo)
}

func TestNewQueueService(t *testing.T) {
	repo := NewMockQueueRepository()
	queue := NewQueueService(repo)

	if queue == nil {
		t.Error("NewQueueService should return a non-nil queue service")
	}

	if queue.repository != repo {
		t.Error("QueueService should use the provided repository")
	}

	if queue.processors != 1 {
		t.Errorf("Expected default processors to be 1, got %d", queue.processors)
	}
}

func TestQueueService_EnqueueDequeue(t *testing.T) {
	queue := createTestQueueService()

	// Test enqueue
	item1, err := queue.Enqueue("test data 1")
	if err != nil {
		t.Errorf("Enqueue failed: %v", err)
	}

	if item1 == nil {
		t.Error("Enqueue should return non-nil item")
	}

	if item1.Data != "test data 1" {
		t.Errorf("Expected data 'test data 1', got '%s'", item1.Data)
	}

	if item1.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", item1.Status)
	}

	// Test dequeue
	dequeued, err := queue.Dequeue()
	if err != nil {
		t.Errorf("Dequeue failed: %v", err)
	}

	if dequeued == nil {
		t.Error("Dequeue should return non-nil item")
	}

	if dequeued.ID != item1.ID {
		t.Error("Dequeued item should be the same as enqueued item")
	}

	if dequeued.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", dequeued.Status)
	}

	if dequeued.Attempts != 1 {
		t.Errorf("Expected attempts to be 1, got %d", dequeued.Attempts)
	}
}

func TestQueueService_Priority(t *testing.T) {
	queue := createTestQueueService()

	// Enqueue items with different priorities
	item1, _ := queue.Enqueue("low priority", 1)
	item2, _ := queue.Enqueue("high priority", 10)
	item3, _ := queue.Enqueue("medium priority", 5)

	// Dequeue should return highest priority first
	dequeued1, err := queue.Dequeue()
	if err != nil {
		t.Errorf("Dequeue failed: %v", err)
	}
	if dequeued1.ID != item2.ID {
		t.Error("Should dequeue highest priority item first")
	}

	dequeued2, err := queue.Dequeue()
	if err != nil {
		t.Errorf("Dequeue failed: %v", err)
	}
	if dequeued2.ID != item3.ID {
		t.Error("Should dequeue medium priority item second")
	}

	dequeued3, err := queue.Dequeue()
	if err != nil {
		t.Errorf("Dequeue failed: %v", err)
	}
	if dequeued3.ID != item1.ID {
		t.Error("Should dequeue low priority item last")
	}
}

func TestQueueService_Peek(t *testing.T) {
	queue := createTestQueueService()

	// Empty queue
	peeked, err := queue.Peek()
	if err != nil {
		t.Errorf("Peek failed: %v", err)
	}
	if peeked != nil {
		t.Error("Peek should return nil for empty queue")
	}

	// Add item
	item, _ := queue.Enqueue("test data")

	// Peek should return the item without removing it
	peeked, err = queue.Peek()
	if err != nil {
		t.Errorf("Peek failed: %v", err)
	}
	if peeked == nil {
		t.Error("Peek should return the item")
	}
	if peeked.ID != item.ID {
		t.Error("Peeked item should match enqueued item")
	}

	// Peek again - should still return the same item
	peeked2, err := queue.Peek()
	if err != nil {
		t.Errorf("Peek failed: %v", err)
	}
	if peeked2 == nil || peeked2.ID != item.ID {
		t.Error("Second peek should return the same item")
	}

	// Dequeue should still work
	dequeued, err := queue.Dequeue()
	if err != nil {
		t.Errorf("Dequeue failed: %v", err)
	}
	if dequeued.ID != item.ID {
		t.Error("Dequeue should return the peeked item")
	}
}

func TestQueueService_Size(t *testing.T) {
	queue := createTestQueueService()

	// Empty queue
	size, err := queue.Size()
	if err != nil {
		t.Errorf("Size failed: %v", err)
	}
	if size != 0 {
		t.Errorf("Expected size 0, got %d", size)
	}

	// Add items
	queue.Enqueue("item1")
	queue.Enqueue("item2")

	size, err = queue.Size()
	if err != nil {
		t.Errorf("Size failed: %v", err)
	}
	if size != 2 {
		t.Errorf("Expected size 2, got %d", size)
	}

	// Dequeue one
	queue.Dequeue()

	size, err = queue.Size()
	if err != nil {
		t.Errorf("Size failed: %v", err)
	}
	if size != 1 {
		t.Errorf("Expected size 1, got %d", size)
	}
}

func TestQueueService_IsEmpty(t *testing.T) {
	queue := createTestQueueService()

	// Empty queue
	isEmpty, err := queue.IsEmpty()
	if err != nil {
		t.Errorf("IsEmpty failed: %v", err)
	}
	if !isEmpty {
		t.Error("Queue should be empty")
	}

	// Add item
	queue.Enqueue("test")

	isEmpty, err = queue.IsEmpty()
	if err != nil {
		t.Errorf("IsEmpty failed: %v", err)
	}
	if isEmpty {
		t.Error("Queue should not be empty")
	}
}

func TestQueueService_Clear(t *testing.T) {
	queue := createTestQueueService()

	// Add items
	queue.Enqueue("item1")
	queue.Enqueue("item2")

	// Clear
	err := queue.Clear()
	if err != nil {
		t.Errorf("Clear failed: %v", err)
	}

	// Should be empty
	isEmpty, _ := queue.IsEmpty()
	if !isEmpty {
		t.Error("Queue should be empty after clear")
	}

	size, _ := queue.Size()
	if size != 0 {
		t.Errorf("Expected size 0 after clear, got %d", size)
	}
}

func TestQueueService_MarkCompleted(t *testing.T) {
	queue := createTestQueueService()

	item, _ := queue.Enqueue("test data")
	queue.Dequeue() // Process the item

	err := queue.MarkCompleted(item.ID)
	if err != nil {
		t.Errorf("MarkCompleted failed: %v", err)
	}

	// Verify status changed
	updatedItem, _ := queue.repository.GetItemByID(item.ID)
	if updatedItem.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", updatedItem.Status)
	}
}

func TestQueueService_MarkFailed(t *testing.T) {
	queue := createTestQueueService()

	item, _ := queue.Enqueue("test data")
	queue.Dequeue() // Process the item

	errorMsg := "processing failed"
	err := queue.MarkFailed(item.ID, errorMsg)
	if err != nil {
		t.Errorf("MarkFailed failed: %v", err)
	}

	// Verify status and error changed
	updatedItem, _ := queue.repository.GetItemByID(item.ID)
	if updatedItem.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", updatedItem.Status)
	}
	if updatedItem.LastError != errorMsg {
		t.Errorf("Expected error '%s', got '%s'", errorMsg, updatedItem.LastError)
	}
}

func TestQueueService_RetryItem(t *testing.T) {
	queue := createTestQueueService()

	item, _ := queue.Enqueue("test data")
	queue.Dequeue() // Process the item
	queue.MarkFailed(item.ID, "some error")

	err := queue.RetryItem(item.ID)
	if err != nil {
		t.Errorf("RetryItem failed: %v", err)
	}

	// Verify status reset
	updatedItem, _ := queue.repository.GetItemByID(item.ID)
	if updatedItem.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", updatedItem.Status)
	}
	if updatedItem.LastError != "" {
		t.Errorf("Expected empty error, got '%s'", updatedItem.LastError)
	}
	if updatedItem.ProcessedAt != nil {
		t.Error("ProcessedAt should be nil after retry")
	}
}

func TestQueueService_StartStop(t *testing.T) {
	queue := createTestQueueService()

	// Start
	err := queue.Start()
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}
	if !queue.isRunning {
		t.Error("Queue should be running after Start")
	}

	// Start again should fail
	err = queue.Start()
	if err == nil {
		t.Error("Start should fail when already running")
	}

	// Stop
	err = queue.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
	if queue.isRunning {
		t.Error("Queue should not be running after Stop")
	}

	// Stop again should be OK
	err = queue.Stop()
	if err != nil {
		t.Errorf("Stop should be OK when not running: %v", err)
	}
}

// Test error conditions
func TestQueueService_RepositoryErrors(t *testing.T) {
	mockRepo := NewMockQueueRepository()
	queue := NewQueueService(mockRepo)

	// Set repository to return errors
	mockRepo.SetError(true, "test error")

	// Test enqueue error
	_, err := queue.Enqueue("test")
	if err == nil {
		t.Error("Enqueue should fail when repository returns error")
	}

	// Test dequeue error
	_, err = queue.Dequeue()
	if err == nil {
		t.Error("Dequeue should fail when repository returns error")
	}

	// Test size error
	_, err = queue.Size()
	if err == nil {
		t.Error("Size should fail when repository returns error")
	}
}

func TestQueueService_NilRepository(t *testing.T) {
	queue := NewQueueService(nil)

	_, err := queue.Enqueue("test")
	if err == nil {
		t.Error("Enqueue should fail with nil repository")
	}

	_, err = queue.Dequeue()
	if err == nil {
		t.Error("Dequeue should fail with nil repository")
	}
}

// Test global functions
func TestGlobalQueueFunctions(t *testing.T) {
	// Reset global queue
	defaultQueue = nil

	// Test without initialization
	_, err := EnqueueItem("test")
	if err == nil {
		t.Error("EnqueueItem should fail when not initialized")
	}

	_, err = DequeueItem()
	if err == nil {
		t.Error("DequeueItem should fail when not initialized")
	}

	_, err = GetQueueSize()
	if err == nil {
		t.Error("GetQueueSize should fail when not initialized")
	}

	err = StartQueue()
	if err == nil {
		t.Error("StartQueue should fail when not initialized")
	}

	// Initialize
	repo := NewMockQueueRepository()
	InitDefaultQueue(repo)

	if defaultQueue == nil {
		t.Error("InitDefaultQueue should set defaultQueue")
	}

	if GetDefaultQueue() != defaultQueue {
		t.Error("GetDefaultQueue should return defaultQueue")
	}

	// Test global functions
	item, err := EnqueueItem("test data")
	if err != nil {
		t.Errorf("EnqueueItem failed: %v", err)
	}
	if item == nil {
		t.Error("EnqueueItem should return item")
	}

	size, err := GetQueueSize()
	if err != nil {
		t.Errorf("GetQueueSize failed: %v", err)
	}
	if size != 1 {
		t.Errorf("Expected size 1, got %d", size)
	}

	dequeued, err := DequeueItem()
	if err != nil {
		t.Errorf("DequeueItem failed: %v", err)
	}
	if dequeued == nil {
		t.Error("DequeueItem should return item")
	}

	err = StartQueue()
	if err != nil {
		t.Errorf("StartQueue failed: %v", err)
	}

	err = StopQueue()
	if err != nil {
		t.Errorf("StopQueue failed: %v", err)
	}
}

// Test QueueRepository with real SQLite
func TestQueueRepository_SQLite(t *testing.T) {
	// Create temporary database file
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_queue.db")
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath, newTestLogger(t))
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	repo := NewQueueRepository(db, newTestLogger(t))

	// Test enqueue
	item := &QueueItem{
		ID:        "test-1",
		Data:      "test data",
		Priority:  5,
		CreatedAt: time.Now(),
		Status:    "pending",
		Attempts:  0,
	}

	err = repo.EnqueueItem(item)
	if err != nil {
		t.Errorf("EnqueueItem failed: %v", err)
	}

	// Test get by ID
	retrieved, err := repo.GetItemByID("test-1")
	if err != nil {
		t.Errorf("GetItemByID failed: %v", err)
	}
	if retrieved == nil {
		t.Error("GetItemByID should return item")
	}
	if retrieved.Data != "test data" {
		t.Errorf("Expected data 'test data', got '%s'", retrieved.Data)
	}

	// Test queue length
	length, err := repo.GetQueueLength()
	if err != nil {
		t.Errorf("GetQueueLength failed: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected length 1, got %d", length)
	}

	// Test dequeue
	dequeued, err := repo.DequeueItem()
	if err != nil {
		t.Errorf("DequeueItem failed: %v", err)
	}
	if dequeued == nil {
		t.Error("DequeueItem should return item")
	}
	if dequeued.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", dequeued.Status)
	}

	// Queue should now be empty
	length, err = repo.GetQueueLength()
	if err != nil {
		t.Errorf("GetQueueLength failed: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected length 0 after dequeue, got %d", length)
	}
}
