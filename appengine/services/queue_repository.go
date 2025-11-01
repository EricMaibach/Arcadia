package services

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"arcadia/pkg/logging"
)

// QueueRepository handles persistence of queue items
type QueueRepository struct {
	db     Database
	mutex  sync.RWMutex
	logger logging.Logger
}

// NewQueueRepository creates a new queue repository
func NewQueueRepository(db Database, logger logging.Logger) *QueueRepository {
	repo := &QueueRepository{
		db:     db,
		logger: logger,
	}
	// Initialize tables
	repo.initializeTables()
	return repo
}

// initializeTables creates the necessary tables if they don't exist
func (r *QueueRepository) initializeTables() {
	ctx := context.Background()
	if r.db == nil {
		return // Skip table creation if database is nil
	}

	query := `CREATE TABLE IF NOT EXISTS queue_items (
		id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		priority INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		processed_at DATETIME,
		status TEXT DEFAULT 'pending',
		attempts INTEGER DEFAULT 0,
		last_error TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := r.db.Exec(query); err != nil {
		if r.logger != nil {
			r.logger.Warn(ctx, "Failed to create queue_items table", "error", err)
		}
	}

	// Create index for efficient priority-based dequeue operations
	indexQuery := `CREATE INDEX IF NOT EXISTS idx_queue_priority_created
		ON queue_items(status, priority DESC, created_at ASC)
		WHERE status = 'pending';`

	if _, err := r.db.Exec(indexQuery); err != nil {
		if r.logger != nil {
			r.logger.Warn(ctx, "Failed to create queue priority index", "error", err)
		}
	}
}

// EnqueueItem adds an item to the queue
func (r *QueueRepository) EnqueueItem(item *QueueItem) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `INSERT INTO queue_items 
		(id, data, priority, created_at, status, attempts)
		VALUES (?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query, item.ID, item.Data, item.Priority, 
		item.CreatedAt, item.Status, item.Attempts)

	if err != nil {
		return fmt.Errorf("failed to insert queue item: %v", err)
	}

	return nil
}

// DequeueItem removes and returns the highest priority pending item
func (r *QueueRepository) DequeueItem() (*QueueItem, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// First, get the highest priority pending item
	item, err := r.peekNextInternal()
	if err != nil {
		return nil, err
	}

	if item == nil {
		return nil, nil // Queue is empty
	}

	// Remove the item from pending items by updating its status
	// (We don't actually delete it to maintain history)
	updateQuery := `UPDATE queue_items 
		SET status = 'processing', processed_at = ?, attempts = attempts + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'pending'`

	now := time.Now()
	result, err := r.db.Exec(updateQuery, now, item.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update dequeued item: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		// Item was already dequeued by another process
		return nil, nil
	}

	// Update the item with new status
	item.Status = "processing"
	item.ProcessedAt = &now
	item.Attempts++

	return item, nil
}

// PeekNext returns the next item without removing it
func (r *QueueRepository) PeekNext() (*QueueItem, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.peekNextInternal()
}

// peekNextInternal is the internal implementation of PeekNext (without locking)
func (r *QueueRepository) peekNextInternal() (*QueueItem, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, data, priority, created_at, processed_at, status, attempts, last_error
		FROM queue_items 
		WHERE status = 'pending'
		ORDER BY priority DESC, created_at ASC 
		LIMIT 1`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query queue items: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No items found
	}

	item, err := r.scanQueueItem(rows)
	if err != nil {
		return nil, err
	}

	return item, nil
}

// GetItemByID retrieves an item by its ID
func (r *QueueRepository) GetItemByID(id string) (*QueueItem, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, data, priority, created_at, processed_at, status, attempts, last_error
		FROM queue_items WHERE id = ?`

	rows, err := r.db.Query(query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query queue item: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // Item not found
	}

	return r.scanQueueItem(rows)
}

// UpdateItem updates an existing item
func (r *QueueRepository) UpdateItem(item *QueueItem) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `UPDATE queue_items 
		SET data = ?, priority = ?, processed_at = ?, status = ?, attempts = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	_, err := r.db.Exec(query, item.Data, item.Priority, item.ProcessedAt, 
		item.Status, item.Attempts, item.LastError, item.ID)

	if err != nil {
		return fmt.Errorf("failed to update queue item: %v", err)
	}

	return nil
}

// DeleteItem removes an item from the queue
func (r *QueueRepository) DeleteItem(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM queue_items WHERE id = ?`
	_, err := r.db.Exec(query, id)
	
	if err != nil {
		return fmt.Errorf("failed to delete queue item: %v", err)
	}

	return nil
}

// GetQueueLength returns the number of pending items
func (r *QueueRepository) GetQueueLength() (int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	query := `SELECT COUNT(*) FROM queue_items WHERE status = 'pending'`
	rows, err := r.db.Query(query)
	if err != nil {
		return 0, fmt.Errorf("failed to count queue items: %v", err)
	}
	defer rows.Close()

	var count int
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, fmt.Errorf("failed to scan count: %v", err)
		}
	}

	return count, nil
}

// GetPendingItems returns pending items with optional limit
func (r *QueueRepository) GetPendingItems(limit int) ([]*QueueItem, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, data, priority, created_at, processed_at, status, attempts, last_error
		FROM queue_items 
		WHERE status = 'pending'
		ORDER BY priority DESC, created_at ASC`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending items: %v", err)
	}
	defer rows.Close()

	return r.scanQueueItems(rows)
}

// GetAllItems returns all items in the queue
func (r *QueueRepository) GetAllItems() ([]*QueueItem, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, data, priority, created_at, processed_at, status, attempts, last_error
		FROM queue_items 
		ORDER BY created_at ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all items: %v", err)
	}
	defer rows.Close()

	return r.scanQueueItems(rows)
}

// ClearQueue removes all items from the queue
func (r *QueueRepository) ClearQueue() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM queue_items`
	_, err := r.db.Exec(query)
	
	if err != nil {
		return fmt.Errorf("failed to clear queue: %v", err)
	}

	return nil
}

// Helper methods for scanning database results

// scanQueueItem scans a single row into a QueueItem
func (r *QueueRepository) scanQueueItem(rows *sql.Rows) (*QueueItem, error) {
	var item QueueItem
	var processedAt sql.NullTime
	var lastError sql.NullString

	err := rows.Scan(&item.ID, &item.Data, &item.Priority, &item.CreatedAt,
		&processedAt, &item.Status, &item.Attempts, &lastError)
	
	if err != nil {
		return nil, fmt.Errorf("failed to scan queue item: %v", err)
	}

	if processedAt.Valid {
		item.ProcessedAt = &processedAt.Time
	}

	if lastError.Valid {
		item.LastError = lastError.String
	}

	return &item, nil
}

// scanQueueItems scans multiple rows into a slice of QueueItems
func (r *QueueRepository) scanQueueItems(rows *sql.Rows) ([]*QueueItem, error) {
	var items []*QueueItem

	for rows.Next() {
		item, err := r.scanQueueItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return items, nil
}

// Global repository instance for backward compatibility
var defaultQueueRepository *QueueRepository

// InitQueueRepository initializes the default queue repository
func InitQueueRepository(db Database, logger logging.Logger) {
	defaultQueueRepository = NewQueueRepository(db, logger)
}

// GetQueueRepository returns the default queue repository
func GetQueueRepository() *QueueRepository {
	return defaultQueueRepository
}