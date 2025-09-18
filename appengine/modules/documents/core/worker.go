package core

import (
	"context"
	"sync"
	"time"

	"arcadia/modules/documents/interfaces"
)

// SimpleWorkerPool implements a basic worker pool for background processing
type SimpleWorkerPool struct {
	workers    int
	jobQueue   chan interfaces.Job
	quitChan   chan bool
	waitGroup  sync.WaitGroup
	started    bool
	mutex      sync.RWMutex
	logger     interfaces.Logger
	metrics    interfaces.MetricsCollector
	activeJobs int
}

// NewSimpleWorkerPool creates a new simple worker pool
func NewSimpleWorkerPool(workers int, queueSize int) *SimpleWorkerPool {
	return &SimpleWorkerPool{
		workers:  workers,
		jobQueue: make(chan interfaces.Job, queueSize),
		quitChan: make(chan bool),
	}
}

// WithLogger adds logging to the worker pool
func (swp *SimpleWorkerPool) WithLogger(logger interfaces.Logger) *SimpleWorkerPool {
	swp.logger = logger
	return swp
}

// WithMetrics adds metrics collection to the worker pool
func (swp *SimpleWorkerPool) WithMetrics(metrics interfaces.MetricsCollector) *SimpleWorkerPool {
	swp.metrics = metrics
	return swp
}

// Start starts the worker pool
func (swp *SimpleWorkerPool) Start(ctx context.Context) error {
	swp.mutex.Lock()
	defer swp.mutex.Unlock()

	if swp.started {
		return nil
	}

	// Start workers
	for i := 0; i < swp.workers; i++ {
		swp.waitGroup.Add(1)
		go swp.worker(ctx, i)
	}

	swp.started = true

	if swp.logger != nil {
		swp.logger.Info(ctx, "Worker pool started", "workers", swp.workers)
	}

	return nil
}

// Stop stops the worker pool
func (swp *SimpleWorkerPool) Stop(ctx context.Context) error {
	swp.mutex.Lock()
	defer swp.mutex.Unlock()

	if !swp.started {
		return nil
	}

	// Signal workers to quit
	close(swp.quitChan)

	// Wait for all workers to finish
	swp.waitGroup.Wait()

	swp.started = false

	if swp.logger != nil {
		swp.logger.Info(ctx, "Worker pool stopped")
	}

	return nil
}

// SubmitJob submits a job to the worker pool
func (swp *SimpleWorkerPool) SubmitJob(job interfaces.Job) error {
	swp.mutex.RLock()
	defer swp.mutex.RUnlock()

	if !swp.started {
		return &WorkerPoolError{
			Op:  "submit",
			Err: "worker pool not started",
		}
	}

	select {
	case swp.jobQueue <- job:
		if swp.metrics != nil {
			swp.metrics.IncrementCounter("worker_pool.job.submitted", nil)
		}
		return nil
	default:
		if swp.metrics != nil {
			swp.metrics.IncrementCounter("worker_pool.job.rejected", nil)
		}
		return &WorkerPoolError{
			Op:  "submit",
			Err: "job queue is full",
		}
	}
}

// GetQueueSize returns the current queue size
func (swp *SimpleWorkerPool) GetQueueSize() int {
	return len(swp.jobQueue)
}

// GetActiveWorkers returns the number of active workers
func (swp *SimpleWorkerPool) GetActiveWorkers() int {
	swp.mutex.RLock()
	defer swp.mutex.RUnlock()
	return swp.activeJobs
}

// worker is the main worker goroutine
func (swp *SimpleWorkerPool) worker(ctx context.Context, workerID int) {
	defer swp.waitGroup.Done()

	if swp.logger != nil {
		swp.logger.Debug(ctx, "Worker started", "worker_id", workerID)
	}

	for {
		select {
		case job := <-swp.jobQueue:
			if job == nil {
				continue
			}

			// Track active job
			swp.mutex.Lock()
			swp.activeJobs++
			swp.mutex.Unlock()

			startTime := time.Now()

			// Execute job
			err := job.Execute(ctx)

			duration := time.Since(startTime)

			// Update metrics
			if swp.metrics != nil {
				swp.metrics.RecordTimer("worker_pool.job.duration", duration.Seconds()*1000, map[string]string{
					"worker_id": string(rune(workerID)),
				})

				if err != nil {
					swp.metrics.IncrementCounter("worker_pool.job.failed", nil)
				} else {
					swp.metrics.IncrementCounter("worker_pool.job.completed", nil)
				}
			}

			// Log job completion
			if swp.logger != nil {
				if err != nil {
					swp.logger.Error(ctx, "Job execution failed",
						"worker_id", workerID,
						"job_id", job.GetID(),
						"error", err,
						"duration", duration)
				} else {
					swp.logger.Debug(ctx, "Job completed",
						"worker_id", workerID,
						"job_id", job.GetID(),
						"duration", duration)
				}
			}

			// Track job completion
			swp.mutex.Lock()
			swp.activeJobs--
			swp.mutex.Unlock()

		case <-swp.quitChan:
			if swp.logger != nil {
				swp.logger.Debug(ctx, "Worker stopping", "worker_id", workerID)
			}
			return

		case <-ctx.Done():
			if swp.logger != nil {
				swp.logger.Debug(ctx, "Worker stopped due to context cancellation", "worker_id", workerID)
			}
			return
		}
	}
}

// WorkerPoolError represents an error in the worker pool
type WorkerPoolError struct {
	Op  string
	Err string
}

func (e *WorkerPoolError) Error() string {
	return "worker pool " + e.Op + ": " + e.Err
}

// ProcessingJob implements interfaces.Job for document processing
type ProcessingJob struct {
	id       string
	priority int
	filePath string
	processor interfaces.DocumentProcessor
}

// NewProcessingJob creates a new processing job
func NewProcessingJob(id string, filePath string, processor interfaces.DocumentProcessor) *ProcessingJob {
	return &ProcessingJob{
		id:        id,
		priority:  0,
		filePath:  filePath,
		processor: processor,
	}
}

// Execute executes the processing job
func (pj *ProcessingJob) Execute(ctx context.Context) error {
	_, err := pj.processor.ProcessFile(ctx, pj.filePath)
	return err
}

// GetID returns the job ID
func (pj *ProcessingJob) GetID() string {
	return pj.id
}

// GetPriority returns the job priority
func (pj *ProcessingJob) GetPriority() int {
	return pj.priority
}

// SetPriority sets the job priority
func (pj *ProcessingJob) SetPriority(priority int) {
	pj.priority = priority
}