package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"` // "create", "write", "remove", "rename", "chmod", "discovered", "changed"
	Timestamp time.Time `json:"timestamp"`
	IsDir     bool      `json:"is_dir"`
}

// TrackedFile represents a file being tracked in the database
type TrackedFile struct {
	ID            int64  `json:"id"`
	Path          string `json:"path"`
	DirectoryRoot string `json:"directory_root"`
	LastModified  int64  `json:"last_modified"`  // Unix timestamp
	FileSize      int64  `json:"file_size"`
	LastChecked   int64  `json:"last_checked"`   // Unix timestamp
}

// FileWatcherEventHandler is a callback function for handling file events
type FileWatcherEventHandler func(event FileEvent)

// FileWatcher defines the interface for the file watching service
type FileWatcher interface {
	// Start begins watching all configured directories
	Start() error
	
	// Stop stops watching all directories
	Stop() error
	
	// AddDirectory adds a directory to the watch list
	AddDirectory(path string) error
	
	// RemoveDirectory removes a directory from the watch list
	RemoveDirectory(path string) error
	
	// GetWatchedDirectories returns a list of all watched directories
	GetWatchedDirectories() ([]string, error)
	
	// RegisterEventHandler registers a callback for file events
	RegisterEventHandler(handler FileWatcherEventHandler)
	
	// UnregisterEventHandler removes a previously registered handler
	UnregisterEventHandler(handler FileWatcherEventHandler)
	
	// IsRunning returns true if the watcher is currently running
	IsRunning() bool
}

// FileWatcherService implements the FileWatcher interface
type FileWatcherService struct {
	watcher         *fsnotify.Watcher
	db              Database
	handlers        []FileWatcherEventHandler
	mu              sync.RWMutex
	running         bool
	stopChan        chan struct{}
	eventBuffer     chan FileEvent
	bufferSize      int
	debounceDelay   time.Duration
	recentEvents    map[string]time.Time
	eventsMu        sync.Mutex
}

// FileWatcherConfig contains configuration for the file watcher service
type FileWatcherConfig struct {
	BufferSize    int           // Size of the event buffer channel
	DebounceDelay time.Duration // Delay to debounce rapid file changes
}

// DefaultFileWatcherConfig returns default configuration
func DefaultFileWatcherConfig() FileWatcherConfig {
	return FileWatcherConfig{
		BufferSize:    1000,
		DebounceDelay: 100 * time.Millisecond,
	}
}

// NewFileWatcherService creates a new file watcher service instance
func NewFileWatcherService(db Database, config FileWatcherConfig) (*FileWatcherService, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	service := &FileWatcherService{
		watcher:       watcher,
		db:            db,
		handlers:      make([]FileWatcherEventHandler, 0),
		stopChan:      make(chan struct{}),
		eventBuffer:   make(chan FileEvent, config.BufferSize),
		bufferSize:    config.BufferSize,
		debounceDelay: config.DebounceDelay,
		recentEvents:  make(map[string]time.Time),
	}

	// Initialize database schema
	if err := service.initializeDatabase(); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Load existing watched directories from database
	if err := service.loadWatchedDirectories(); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to load watched directories: %w", err)
	}

	return service, nil
}

// initializeDatabase creates the necessary tables for storing watched directories and tracked files
func (f *FileWatcherService) initializeDatabase() error {
	query := `
	CREATE TABLE IF NOT EXISTS watched_directories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL UNIQUE,
		added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT
	);
	
	CREATE INDEX IF NOT EXISTS idx_watched_directories_path ON watched_directories(path);
	
	CREATE TABLE IF NOT EXISTS tracked_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL UNIQUE,
		directory_root TEXT NOT NULL,
		last_modified INTEGER NOT NULL,
		file_size INTEGER NOT NULL,
		last_checked INTEGER DEFAULT (strftime('%s', 'now')),
		FOREIGN KEY (directory_root) REFERENCES watched_directories(path) ON DELETE CASCADE
	);
	
	CREATE INDEX IF NOT EXISTS idx_tracked_files_directory_root ON tracked_files(directory_root);
	CREATE INDEX IF NOT EXISTS idx_tracked_files_path ON tracked_files(path);
	CREATE INDEX IF NOT EXISTS idx_tracked_files_last_modified ON tracked_files(last_modified);
	`
	
	_, err := f.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create database tables: %w", err)
	}
	
	return nil
}

// loadWatchedDirectories loads previously watched directories from the database
func (f *FileWatcherService) loadWatchedDirectories() error {
	query := "SELECT path FROM watched_directories"
	rows, err := f.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query watched directories: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			log.Printf("[FileWatcher] Error scanning directory path: %v", err)
			continue
		}
		
		// Add to fsnotify watcher recursively (without adding to DB again)
		addedDirs, err := f.addDirectoryRecursive(path)
		if err != nil {
			log.Printf("[FileWatcher] Failed to add directory %s to watcher recursively: %v", path, err)
			// Continue loading other directories even if one fails
		} else {
			log.Printf("[FileWatcher] Loaded watched directory: %s (including %d subdirectories)", path, len(addedDirs)-1)
			
			// Scan for file changes that occurred while service was stopped
			go func(dirPath string) {
				if err := f.scanDirectoryForFiles(dirPath, true); err != nil {
					log.Printf("[FileWatcher] Warning: failed to scan for file changes in %s: %v", dirPath, err)
				}
			}(path)
		}
	}

	return rows.Err()
}

// Start begins watching all configured directories
func (f *FileWatcherService) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.running {
		return fmt.Errorf("file watcher is already running")
	}

	f.running = true
	f.stopChan = make(chan struct{})

	// Start the event processor goroutine
	go f.processEvents()

	// Start the event distributor goroutine
	go f.distributeEvents()

	log.Println("[FileWatcher] Service started")
	return nil
}

// Stop stops watching all directories
func (f *FileWatcherService) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.running {
		return fmt.Errorf("file watcher is not running")
	}

	f.running = false
	close(f.stopChan)
	
	// Give goroutines time to clean up
	time.Sleep(100 * time.Millisecond)
	
	log.Println("[FileWatcher] Service stopped")
	return nil
}

// AddDirectory adds a directory and all its subdirectories to the watch list
func (f *FileWatcherService) AddDirectory(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", absPath)
	}

	// Add the root directory and all subdirectories recursively
	addedDirs, err := f.addDirectoryRecursive(absPath)
	if err != nil {
		// Clean up any directories that were added before the error
		for _, dir := range addedDirs {
			f.watcher.Remove(dir)
		}
		return fmt.Errorf("failed to add directory recursively: %w", err)
	}

	// Store in database (only store the root directory)
	query := `
	INSERT OR IGNORE INTO watched_directories (path) VALUES (?)
	`
	_, err = f.db.Exec(query, absPath)
	if err != nil {
		// Remove all added directories from watcher if database insert fails
		for _, dir := range addedDirs {
			f.watcher.Remove(dir)
		}
		return fmt.Errorf("failed to store watched directory in database: %w", err)
	}

	// Scan directory for existing files and emit discovery events
	go func() {
		if err := f.scanDirectoryForFiles(absPath, false); err != nil {
			log.Printf("[FileWatcher] Warning: failed to scan directory for files %s: %v", absPath, err)
		}
	}()

	log.Printf("[FileWatcher] Added directory to watch list: %s (including %d subdirectories)", absPath, len(addedDirs)-1)
	return nil
}

// addDirectoryRecursive recursively adds a directory and all its subdirectories to the fsnotify watcher
func (f *FileWatcherService) addDirectoryRecursive(rootPath string) ([]string, error) {
	var addedDirs []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[FileWatcher] Warning: error accessing path %s: %v", path, err)
			return nil // Continue walking, don't fail the entire operation
		}

		// Only add directories, not files
		if !info.IsDir() {
			return nil
		}

		// Skip hidden directories (starting with .)
		if strings.HasPrefix(info.Name(), ".") && path != rootPath {
			return filepath.SkipDir
		}

		// Add directory to fsnotify watcher
		if err := f.watcher.Add(path); err != nil {
			log.Printf("[FileWatcher] Warning: failed to add subdirectory %s to watcher: %v", path, err)
			return nil // Continue walking, don't fail the entire operation
		}

		addedDirs = append(addedDirs, path)
		return nil
	})

	if err != nil {
		return addedDirs, fmt.Errorf("error walking directory tree: %w", err)
	}

	return addedDirs, nil
}

// RemoveDirectory removes a directory and all its subdirectories from the watch list
func (f *FileWatcherService) RemoveDirectory(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Remove the root directory and all subdirectories from fsnotify watcher
	removedCount := f.removeDirectoryRecursive(absPath)

	// Remove from database
	query := `DELETE FROM watched_directories WHERE path = ?`
	result, err := f.db.Exec(query, absPath)
	if err != nil {
		return fmt.Errorf("failed to remove watched directory from database: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("directory not found in watch list: %s", absPath)
	}

	// Remove tracked files for this directory
	if err := f.removeTrackedFilesForDirectory(absPath); err != nil {
		log.Printf("[FileWatcher] Warning: failed to remove tracked files for directory %s: %v", absPath, err)
	}

	log.Printf("[FileWatcher] Removed directory from watch list: %s (including %d subdirectories)", absPath, removedCount-1)
	return nil
}

// removeDirectoryRecursive recursively removes a directory and all its subdirectories from the fsnotify watcher
func (f *FileWatcherService) removeDirectoryRecursive(rootPath string) int {
	removedCount := 0

	// If the directory still exists, walk through it to remove all subdirectories
	if _, err := os.Stat(rootPath); err == nil {
		filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Continue walking
			}

			// Only process directories
			if !info.IsDir() {
				return nil
			}

			// Remove directory from fsnotify watcher
			if err := f.watcher.Remove(path); err != nil {
				log.Printf("[FileWatcher] Warning: failed to remove subdirectory %s from watcher: %v", path, err)
			} else {
				removedCount++
			}

			return nil
		})
	} else {
		// Directory doesn't exist, just try to remove the root path
		if err := f.watcher.Remove(rootPath); err != nil {
			log.Printf("[FileWatcher] Warning: failed to remove directory %s from watcher: %v", rootPath, err)
		} else {
			removedCount++
		}
	}

	return removedCount
}

// GetWatchedDirectories returns a list of all watched directories
func (f *FileWatcherService) GetWatchedDirectories() ([]string, error) {
	query := "SELECT path FROM watched_directories ORDER BY path"
	rows, err := f.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query watched directories: %w", err)
	}
	defer rows.Close()

	var directories []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan directory path: %w", err)
		}
		directories = append(directories, path)
	}

	return directories, rows.Err()
}

// RegisterEventHandler registers a callback for file events
func (f *FileWatcherService) RegisterEventHandler(handler FileWatcherEventHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	f.handlers = append(f.handlers, handler)
	log.Printf("[FileWatcher] Registered new event handler (total: %d)", len(f.handlers))
}

// UnregisterEventHandler removes a previously registered handler
func (f *FileWatcherService) UnregisterEventHandler(handler FileWatcherEventHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Note: This compares function pointers, which works for the same function instance
	// For more complex scenarios, you might want to use an ID-based system
	newHandlers := make([]FileWatcherEventHandler, 0, len(f.handlers))
	for _, h := range f.handlers {
		if fmt.Sprintf("%p", h) != fmt.Sprintf("%p", handler) {
			newHandlers = append(newHandlers, h)
		}
	}
	f.handlers = newHandlers
	log.Printf("[FileWatcher] Unregistered event handler (remaining: %d)", len(f.handlers))
}

// IsRunning returns true if the watcher is currently running
func (f *FileWatcherService) IsRunning() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.running
}

// processEvents processes fsnotify events and converts them to FileEvents
func (f *FileWatcherService) processEvents() {
	for {
		select {
		case <-f.stopChan:
			return
			
		case event, ok := <-f.watcher.Events:
			if !ok {
				return
			}
			
			// Debounce rapid events for the same file
			if f.shouldDebounce(event.Name) {
				continue
			}
			
			// Check if this is a new directory being created
			isDir := f.isDirectory(event.Name)
			operation := f.mapOperation(event.Op)
			
			// If a new directory was created, add it to the watcher
			if operation == "create" && isDir && !strings.HasPrefix(filepath.Base(event.Name), ".") {
				if err := f.watcher.Add(event.Name); err != nil {
					log.Printf("[FileWatcher] Warning: failed to add new directory %s to watcher: %v", event.Name, err)
				} else {
					log.Printf("[FileWatcher] Automatically added new directory to watcher: %s", event.Name)
				}
			}
			
			// Update tracked file database for file events (not directories)
			if !isDir && (operation == "create" || operation == "write") {
				go func(filePath string) {
					// Find which root directory this file belongs to
					var rootDir string
					for _, dir := range f.getWatchedDirectoryRoots() {
						if strings.HasPrefix(filePath, dir) {
							rootDir = dir
							break
						}
					}
					
					if rootDir != "" {
						if fileInfo, err := f.getFileInfo(filePath); err == nil && fileInfo != nil {
							fileInfo.DirectoryRoot = rootDir
							if err := f.upsertTrackedFile(fileInfo); err != nil {
								log.Printf("[FileWatcher] Warning: failed to update tracked file %s: %v", filePath, err)
							}
						}
					}
				}(event.Name)
			}

			fileEvent := FileEvent{
				Path:      event.Name,
				Operation: operation,
				Timestamp: time.Now(),
				IsDir:     isDir,
			}
			
			// Try to send event to buffer, drop if buffer is full
			select {
			case f.eventBuffer <- fileEvent:
				// Event sent successfully
			default:
				log.Printf("[FileWatcher] Warning: event buffer full, dropping event for %s", event.Name)
			}
			
		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[FileWatcher] Error: %v", err)
		}
	}
}

// distributeEvents distributes buffered events to registered handlers
func (f *FileWatcherService) distributeEvents() {
	for {
		select {
		case <-f.stopChan:
			return
			
		case event, ok := <-f.eventBuffer:
			if !ok {
				return
			}
			
			// Get current handlers
			f.mu.RLock()
			handlers := make([]FileWatcherEventHandler, len(f.handlers))
			copy(handlers, f.handlers)
			f.mu.RUnlock()
			
			// Call each handler in a separate goroutine to prevent blocking
			for _, handler := range handlers {
				go func(h FileWatcherEventHandler, e FileEvent) {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[FileWatcher] Handler panic: %v", r)
						}
					}()
					h(e)
				}(handler, event)
			}
		}
	}
}

// shouldDebounce checks if an event should be debounced
func (f *FileWatcherService) shouldDebounce(path string) bool {
	f.eventsMu.Lock()
	defer f.eventsMu.Unlock()
	
	lastTime, exists := f.recentEvents[path]
	now := time.Now()
	
	// Clean up old entries
	if len(f.recentEvents) > 1000 {
		for p, t := range f.recentEvents {
			if now.Sub(t) > time.Minute {
				delete(f.recentEvents, p)
			}
		}
	}
	
	if exists && now.Sub(lastTime) < f.debounceDelay {
		return true
	}
	
	f.recentEvents[path] = now
	return false
}

// mapOperation maps fsnotify operations to our operation strings
func (f *FileWatcherService) mapOperation(op fsnotify.Op) string {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return "create"
	case op&fsnotify.Write == fsnotify.Write:
		return "write"
	case op&fsnotify.Remove == fsnotify.Remove:
		return "remove"
	case op&fsnotify.Rename == fsnotify.Rename:
		return "rename"
	case op&fsnotify.Chmod == fsnotify.Chmod:
		return "chmod"
	default:
		return "unknown"
	}
}

// isDirectory checks if a path is a directory
func (f *FileWatcherService) isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// File tracking helper functions

// getFileInfo extracts tracking information from a file
func (f *FileWatcherService) getFileInfo(filePath string) (*TrackedFile, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	// Skip directories for file tracking
	if info.IsDir() {
		return nil, nil
	}

	return &TrackedFile{
		Path:         filePath,
		LastModified: info.ModTime().Unix(),
		FileSize:     info.Size(),
		LastChecked:  time.Now().Unix(),
	}, nil
}

// upsertTrackedFile inserts or updates a tracked file in the database
func (f *FileWatcherService) upsertTrackedFile(trackedFile *TrackedFile) error {
	query := `
	INSERT INTO tracked_files (path, directory_root, last_modified, file_size, last_checked) 
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		last_modified = excluded.last_modified,
		file_size = excluded.file_size,
		last_checked = excluded.last_checked
	`
	_, err := f.db.Exec(query, trackedFile.Path, trackedFile.DirectoryRoot, 
		trackedFile.LastModified, trackedFile.FileSize, trackedFile.LastChecked)
	return err
}

// getTrackedFile retrieves a tracked file from the database
func (f *FileWatcherService) getTrackedFile(filePath string) (*TrackedFile, error) {
	query := `SELECT id, path, directory_root, last_modified, file_size, last_checked 
			  FROM tracked_files WHERE path = ?`
	
	rows, err := f.db.Query(query, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to query tracked file: %w", err)
	}
	defer rows.Close()
	
	var tracked TrackedFile
	if rows.Next() {
		err := rows.Scan(&tracked.ID, &tracked.Path, &tracked.DirectoryRoot, 
			&tracked.LastModified, &tracked.FileSize, &tracked.LastChecked)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tracked file: %w", err)
		}
		return &tracked, nil
	}
	
	return nil, nil // File not tracked
}

// removeTrackedFilesForDirectory removes all tracked files for a directory
func (f *FileWatcherService) removeTrackedFilesForDirectory(directoryRoot string) error {
	query := `DELETE FROM tracked_files WHERE directory_root = ?`
	_, err := f.db.Exec(query, directoryRoot)
	if err != nil {
		return fmt.Errorf("failed to remove tracked files for directory %s: %w", directoryRoot, err)
	}
	return nil
}

// scanDirectoryForFiles scans a directory and emits events for all existing files
func (f *FileWatcherService) scanDirectoryForFiles(directoryRoot string, isInitialScan bool) error {
	log.Printf("[FileWatcher] Scanning directory for files: %s", directoryRoot)
	
	err := filepath.Walk(directoryRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[FileWatcher] Warning: error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Get current file info
		currentFile := &TrackedFile{
			Path:          path,
			DirectoryRoot: directoryRoot,
			LastModified:  info.ModTime().Unix(),
			FileSize:      info.Size(),
			LastChecked:   time.Now().Unix(),
		}

		var eventOperation string
		
		if isInitialScan {
			// Check if file changed since last scan
			trackedFile, err := f.getTrackedFile(path)
			if err != nil {
				log.Printf("[FileWatcher] Warning: failed to get tracked file %s: %v", path, err)
				eventOperation = "create" // Default to create if we can't check
			} else if trackedFile == nil {
				eventOperation = "create" // New file
			} else if trackedFile.LastModified != currentFile.LastModified || 
					  trackedFile.FileSize != currentFile.FileSize {
				eventOperation = "modify" // File changed while we were away
			} else {
				// File unchanged, just update last_checked timestamp
				currentFile.ID = trackedFile.ID
				if err := f.upsertTrackedFile(currentFile); err != nil {
					log.Printf("[FileWatcher] Warning: failed to update tracked file %s: %v", path, err)
				}
				return nil // No event needed
			}
		} else {
			eventOperation = "create" // New directory being added
		}

		// Update database
		if err := f.upsertTrackedFile(currentFile); err != nil {
			log.Printf("[FileWatcher] Warning: failed to upsert tracked file %s: %v", path, err)
		}

		// Emit event
		fileEvent := FileEvent{
			Path:      path,
			Operation: eventOperation,
			Timestamp: time.Now(),
			IsDir:     false,
		}

		// Send event to buffer
		select {
		case f.eventBuffer <- fileEvent:
			// Event sent successfully
		default:
			log.Printf("[FileWatcher] Warning: event buffer full, dropping %s event for %s", eventOperation, path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning directory %s: %w", directoryRoot, err)
	}

	return nil
}

// getWatchedDirectoryRoots returns all watched directory root paths
func (f *FileWatcherService) getWatchedDirectoryRoots() []string {
	query := "SELECT path FROM watched_directories ORDER BY length(path) DESC" // Longest paths first
	rows, err := f.db.Query(query)
	if err != nil {
		log.Printf("[FileWatcher] Warning: failed to get watched directory roots: %v", err)
		return []string{}
	}
	defer rows.Close()

	var roots []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		roots = append(roots, path)
	}
	return roots
}

// Close cleans up the file watcher resources
func (f *FileWatcherService) Close() error {
	if f.IsRunning() {
		f.Stop()
	}
	
	if f.watcher != nil {
		return f.watcher.Close()
	}
	
	return nil
}

// GetFileWatcherStats returns statistics about the file watcher
type FileWatcherStats struct {
	IsRunning            bool      `json:"is_running"`
	WatchedDirectories   int       `json:"watched_directories"`
	RegisteredHandlers   int       `json:"registered_handlers"`
	EventBufferSize      int       `json:"event_buffer_size"`
	EventBufferCapacity  int       `json:"event_buffer_capacity"`
	LastEventTime        time.Time `json:"last_event_time,omitempty"`
}

// GetStats returns current statistics about the file watcher
func (f *FileWatcherService) GetStats() FileWatcherStats {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	dirs, _ := f.GetWatchedDirectories()
	
	return FileWatcherStats{
		IsRunning:           f.running,
		WatchedDirectories:  len(dirs),
		RegisteredHandlers:  len(f.handlers),
		EventBufferSize:     len(f.eventBuffer),
		EventBufferCapacity: f.bufferSize,
	}
}

// Global file watcher instance (for singleton pattern if needed)
var (
	globalFileWatcher     FileWatcher
	globalFileWatcherOnce sync.Once
	globalFileWatcherMu   sync.RWMutex
)

// InitGlobalFileWatcher initializes the global file watcher instance
func InitGlobalFileWatcher(db Database, config FileWatcherConfig) error {
	var err error
	globalFileWatcherOnce.Do(func() {
		globalFileWatcherMu.Lock()
		defer globalFileWatcherMu.Unlock()
		
		globalFileWatcher, err = NewFileWatcherService(db, config)
		if err != nil {
			err = fmt.Errorf("failed to initialize global file watcher: %w", err)
			return
		}
		
		log.Println("[FileWatcher] Global instance initialized")
	})
	return err
}

// GetGlobalFileWatcher returns the global file watcher instance
func GetGlobalFileWatcher() FileWatcher {
	globalFileWatcherMu.RLock()
	defer globalFileWatcherMu.RUnlock()
	return globalFileWatcher
}

// FileWatcherRepository provides database operations for file watcher data
type FileWatcherRepository struct {
	db Database
}

// NewFileWatcherRepository creates a new repository instance
func NewFileWatcherRepository(db Database) *FileWatcherRepository {
	return &FileWatcherRepository{db: db}
}

// GetWatchedDirectoryMetadata retrieves metadata for a watched directory
func (r *FileWatcherRepository) GetWatchedDirectoryMetadata(path string) (map[string]interface{}, error) {
	query := "SELECT metadata FROM watched_directories WHERE path = ?"
	rows, err := r.db.Query(query, path)
	if err != nil {
		return nil, fmt.Errorf("failed to query directory metadata: %w", err)
	}
	defer rows.Close()
	
	if !rows.Next() {
		return nil, fmt.Errorf("directory not found: %s", path)
	}
	
	var metadataJSON *string
	if err := rows.Scan(&metadataJSON); err != nil {
		return nil, fmt.Errorf("failed to scan metadata: %w", err)
	}
	
	if metadataJSON == nil || *metadataJSON == "" {
		return make(map[string]interface{}), nil
	}
	
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(*metadataJSON), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	
	return metadata, nil
}

// UpdateWatchedDirectoryMetadata updates metadata for a watched directory
func (r *FileWatcherRepository) UpdateWatchedDirectoryMetadata(path string, metadata map[string]interface{}) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	query := "UPDATE watched_directories SET metadata = ? WHERE path = ?"
	result, err := r.db.Exec(query, string(metadataJSON), path)
	if err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("directory not found: %s", path)
	}
	
	return nil
}