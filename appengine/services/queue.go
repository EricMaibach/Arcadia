package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// QueueItem represents an item in the queue
type QueueItem struct {
	ID        string    `json:"id"`
	Data      string    `json:"data"`
	Priority  int       `json:"priority"`  // Higher number = higher priority
	CreatedAt time.Time `json:"createdAt"`
	ProcessedAt *time.Time `json:"processedAt,omitempty"`
	Status    string    `json:"status"` // "pending", "processing", "completed", "failed"
	Attempts  int       `json:"attempts"`
	LastError string    `json:"lastError,omitempty"`
}

// QueueRepositoryInterface defines the interface for queue persistence
type QueueRepositoryInterface interface {
	EnqueueItem(item *QueueItem) error
	DequeueItem() (*QueueItem, error)
	PeekNext() (*QueueItem, error)
	GetItemByID(id string) (*QueueItem, error)
	UpdateItem(item *QueueItem) error
	DeleteItem(id string) error
	GetQueueLength() (int, error)
	GetPendingItems(limit int) ([]*QueueItem, error)
	GetAllItems() ([]*QueueItem, error)
	ClearQueue() error
}

// QueueInterface defines the interface for queue operations
type QueueInterface interface {
	Enqueue(data string, priority ...int) (*QueueItem, error)
	Dequeue() (*QueueItem, error)
	Peek() (*QueueItem, error)
	Size() (int, error)
	IsEmpty() (bool, error)
	Clear() error
	GetPending(limit int) ([]*QueueItem, error)
	GetAll() ([]*QueueItem, error)
	MarkCompleted(itemID string) error
	MarkFailed(itemID string, errorMsg string) error
	RetryItem(itemID string) error
	Start() error
	Stop() error
}

// QueueService implements the QueueInterface
type QueueService struct {
	repository   QueueRepositoryInterface
	mutex        sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	isRunning    bool
	processors   int // Number of concurrent processors (for future use)
}

// Global queue service instance for backward compatibility
var defaultQueue *QueueService

// NewQueueService creates a new queue service
func NewQueueService(repository QueueRepositoryInterface) *QueueService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &QueueService{
		repository: repository,
		ctx:        ctx,
		cancel:     cancel,
		processors: 1, // Default to 1 processor
	}
}

// generateID generates a random ID for queue items
func generateQueueItemID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "queue_" + hex.EncodeToString(bytes)
}

// Enqueue adds an item to the queue
func (q *QueueService) Enqueue(data string, priority ...int) (*QueueItem, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.repository == nil {
		return nil, fmt.Errorf("queue repository not initialized")
	}

	prio := 0 // Default priority
	if len(priority) > 0 {
		prio = priority[0]
	}

	item := &QueueItem{
		ID:        generateQueueItemID(),
		Data:      data,
		Priority:  prio,
		CreatedAt: time.Now(),
		Status:    "pending",
		Attempts:  0,
	}

	if err := q.repository.EnqueueItem(item); err != nil {
		return nil, fmt.Errorf("failed to enqueue item: %v", err)
	}

	return item, nil
}

// Dequeue removes and returns the highest priority item from the queue
func (q *QueueService) Dequeue() (*QueueItem, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.repository == nil {
		return nil, fmt.Errorf("queue repository not initialized")
	}

	item, err := q.repository.DequeueItem()
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue item: %v", err)
	}

	if item != nil {
		// Mark as processing
		item.Status = "processing"
		now := time.Now()
		item.ProcessedAt = &now
		item.Attempts++

		if err := q.repository.UpdateItem(item); err != nil {
			// Log warning but return the item anyway
			fmt.Printf("Warning: failed to update dequeued item status: %v\n", err)
		}
	}

	return item, nil
}

// Peek returns the next item without removing it from the queue
func (q *QueueService) Peek() (*QueueItem, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if q.repository == nil {
		return nil, fmt.Errorf("queue repository not initialized")
	}

	return q.repository.PeekNext()
}

// Size returns the number of pending items in the queue
func (q *QueueService) Size() (int, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if q.repository == nil {
		return 0, fmt.Errorf("queue repository not initialized")
	}

	return q.repository.GetQueueLength()
}

// IsEmpty checks if the queue is empty
func (q *QueueService) IsEmpty() (bool, error) {
	size, err := q.Size()
	if err != nil {
		return false, err
	}
	return size == 0, nil
}

// Clear removes all items from the queue
func (q *QueueService) Clear() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.repository == nil {
		return fmt.Errorf("queue repository not initialized")
	}

	return q.repository.ClearQueue()
}

// GetPending returns pending items with optional limit
func (q *QueueService) GetPending(limit int) ([]*QueueItem, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if q.repository == nil {
		return nil, fmt.Errorf("queue repository not initialized")
	}

	return q.repository.GetPendingItems(limit)
}

// GetAll returns all items in the queue
func (q *QueueService) GetAll() ([]*QueueItem, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if q.repository == nil {
		return nil, fmt.Errorf("queue repository not initialized")
	}

	return q.repository.GetAllItems()
}

// MarkCompleted marks an item as completed
func (q *QueueService) MarkCompleted(itemID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.repository == nil {
		return fmt.Errorf("queue repository not initialized")
	}

	item, err := q.repository.GetItemByID(itemID)
	if err != nil {
		return fmt.Errorf("failed to get item: %v", err)
	}

	if item == nil {
		return fmt.Errorf("item not found")
	}

	item.Status = "completed"
	return q.repository.UpdateItem(item)
}

// MarkFailed marks an item as failed with an error message
func (q *QueueService) MarkFailed(itemID string, errorMsg string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.repository == nil {
		return fmt.Errorf("queue repository not initialized")
	}

	item, err := q.repository.GetItemByID(itemID)
	if err != nil {
		return fmt.Errorf("failed to get item: %v", err)
	}

	if item == nil {
		return fmt.Errorf("item not found")
	}

	item.Status = "failed"
	item.LastError = errorMsg
	return q.repository.UpdateItem(item)
}

// RetryItem resets a failed item back to pending
func (q *QueueService) RetryItem(itemID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.repository == nil {
		return fmt.Errorf("queue repository not initialized")
	}

	item, err := q.repository.GetItemByID(itemID)
	if err != nil {
		return fmt.Errorf("failed to get item: %v", err)
	}

	if item == nil {
		return fmt.Errorf("item not found")
	}

	item.Status = "pending"
	item.LastError = ""
	item.ProcessedAt = nil
	return q.repository.UpdateItem(item)
}

// Start starts the queue service (placeholder for future processing capabilities)
func (q *QueueService) Start() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.isRunning {
		return fmt.Errorf("queue service is already running")
	}

	q.isRunning = true
	return nil
}

// Stop stops the queue service
func (q *QueueService) Stop() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if !q.isRunning {
		return nil
	}

	q.cancel()
	q.isRunning = false
	return nil
}

// Global queue service functions for backward compatibility

// InitDefaultQueue initializes the default queue service with the given repository
func InitDefaultQueue(repository QueueRepositoryInterface) {
	defaultQueue = NewQueueService(repository)
}

// GetDefaultQueue returns the default queue service
func GetDefaultQueue() *QueueService {
	return defaultQueue
}

// StartQueue starts the default queue service (backward compatibility)
func StartQueue() error {
	if defaultQueue == nil {
		return fmt.Errorf("default queue not initialized - use InitDefaultQueue first")
	}
	return defaultQueue.Start()
}

// StopQueue stops the default queue service (backward compatibility)
func StopQueue() error {
	if defaultQueue == nil {
		return nil
	}
	return defaultQueue.Stop()
}

// EnqueueItem enqueues an item using the default queue (backward compatibility)
func EnqueueItem(data string, priority ...int) (*QueueItem, error) {
	if defaultQueue == nil {
		return nil, fmt.Errorf("default queue not initialized")
	}
	return defaultQueue.Enqueue(data, priority...)
}

// DequeueItem dequeues an item using the default queue (backward compatibility)
func DequeueItem() (*QueueItem, error) {
	if defaultQueue == nil {
		return nil, fmt.Errorf("default queue not initialized")
	}
	return defaultQueue.Dequeue()
}

// GetQueueSize returns the size of the default queue (backward compatibility)
func GetQueueSize() (int, error) {
	if defaultQueue == nil {
		return 0, fmt.Errorf("default queue not initialized")
	}
	return defaultQueue.Size()
}