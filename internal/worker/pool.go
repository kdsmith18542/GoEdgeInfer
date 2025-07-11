package worker

import (
	"context"
	"sync"

	"github.com/kdsmith18542/GoEdgeInfer/internal/inference"
	"github.com/kdsmith18542/GoEdgeInfer/internal/persistence"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/logging"
	"github.com/kdsmith18542/GoEdgeInfer/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Task represents an inference task
type Task struct {
	ModelID      string
	ModelVersion string // Empty string means latest version
	Input        interface{}
	Result       chan<- *inference.Prediction
	Error        chan<- error
}

// PipelineRunner allows injection of custom pipeline logic for testability
// (You can generate a mock if you use mockgen)
//
//go:generate mockgen -destination=../../mocks/mock_processing.go -package=mocks github.com/kdsmith18542/GoEdgeInfer/internal/worker PipelineRunner
type PipelineRunner interface {
	Run(input interface{}) (interface{}, error)
}

// WorkerPool manages a pool of worker goroutines for processing inference tasks
// If persistentQueue is non-nil, tasks are persisted to disk.
type WorkerPool struct {
	tasks           chan Task
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	engine          inference.Engine
	workerWg        sync.WaitGroup
	persistentQueue *persistence.PersistentQueue
	pipeline        PipelineRunner
	tracer          trace.Tracer
}

// NewWorkerPool creates a new WorkerPool with the specified number of workers and optional persistent queue
func NewWorkerPool(engine inference.Engine, numWorkers int, queue *persistence.PersistentQueue, pipeline PipelineRunner, tracer trace.Tracer) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		tasks:           make(chan Task, 100), // Buffer up to 100 tasks
		engine:          engine,
		ctx:             ctx,
		cancel:          cancel,
		persistentQueue: queue,
		pipeline:        pipeline,
		tracer:          tracer,
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		pool.workerWg.Add(1)
		go pool.worker(i)
	}

	logging.Info("Worker pool started", "workers", numWorkers)
	return pool
}

// worker processes tasks from the queue
func (p *WorkerPool) worker(id int) {
	if p.tracer == nil {
		p.tracer = trace.NewNoopTracerProvider().Tracer("")
	}
	ctx, span := p.tracer.Start(p.ctx, "WorkerPool.worker")
	defer span.End()

	for {
		select {
		case task, ok := <-p.tasks:
			if !ok {
				return // Channel closed, exit
			}

			// Pre-processing pipeline (if needed)
			input := task.Input
			if p.pipeline != nil {
				var pipeErr error
				input, pipeErr = p.pipeline.Run(task.Input)
				if pipeErr != nil {
					span.RecordError(pipeErr)
					task.Error <- pipeErr
					close(task.Error)
					continue
				}
			}

			// Process the task (inference)
			result, err := p.engine.Predict(ctx, task.ModelID, task.ModelVersion, input)
			if err != nil {
				span.RecordError(err)
				task.Error <- err
				close(task.Error)
				continue
			}

			// Post-processing pipeline (if needed)
			output := result.Output
			if p.pipeline != nil {
				var pipeErr error
				output, pipeErr = p.pipeline.Run(result.Output)
				if pipeErr != nil {
					span.RecordError(pipeErr)
					task.Error <- pipeErr
					close(task.Error)
					continue
				}
			}
			result.Output = output
			span.SetAttributes(attribute.String("model_id", task.ModelID))
			task.Result <- result
			close(task.Result)

		case <-p.ctx.Done():
			return // Context cancelled, exit
		}
	}
}

// Submit submits a new task to the worker pool (and persistent queue if enabled)
// If modelVersion is an empty string, the latest version will be used
func (p *WorkerPool) Submit(modelID, modelVersion string, input interface{}) (<-chan *inference.Prediction, <-chan error) {
	resultCh := make(chan *inference.Prediction, 1)
	errCh := make(chan error, 1)

	task := Task{
		ModelID:      modelID,
		ModelVersion: modelVersion,
		Input:        input,
		Result:       resultCh,
		Error:        errCh,
	}

	if p.persistentQueue != nil {
		// Persist the task (only the input/modelID, not channels)
		taskCopy := struct {
			ModelID string
			Input   interface{}
		}{
			ModelID: modelID,
			Input:   input,
		}
		_ = p.persistentQueue.Enqueue(taskCopy) // TODO: handle error/log

		// Update queue depth metric
		if depth, _ := p.persistentQueue.Len(); depth >= 0 {
			metrics.QueueDepthGauge.Set(float64(depth))
		}
	}

	select {
	case p.tasks <- task:
		// Task submitted successfully
	case <-p.ctx.Done():
		errCh <- p.ctx.Err()
		close(errCh)
		close(resultCh)
	}

	return resultCh, errCh
}

// On startup, recover and process any tasks from the persistent queue

type queuedTask struct {
	ModelID      string
	ModelVersion string
	Input        interface{}
}

func (p *WorkerPool) RecoverFromPersistentQueue() {
	if p.persistentQueue == nil {
		return
	}
	for {
		var qt queuedTask
		err := p.persistentQueue.Dequeue(&qt)
		if err != nil {
			break // No more tasks or error
		}
		// Re-enqueue to in-memory queue for processing
		p.Submit(qt.ModelID, "", qt.Input) // Use empty version to get latest
	}
}

// Shutdown gracefully shuts down the worker pool
func (p *WorkerPool) Shutdown() {
	logging.Info("Shutting down worker pool...")
	p.cancel()     // Signal workers to stop
	close(p.tasks) // Close the tasks channel

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		p.workerWg.Wait()
		close(done)
	}()

	// Wait for workers to finish or context timeout
	select {
	case <-done:
		logging.Info("Worker pool shut down gracefully")
	case <-p.ctx.Done():
		logging.Warn("Worker pool shutdown timed out")
	}
}

// Stats returns statistics about the worker pool
func (p *WorkerPool) Stats() (int, int) {
	return len(p.tasks), cap(p.tasks)
}

// UpdatePipeline allows live update of the processing pipeline
func (p *WorkerPool) UpdatePipeline(newPipeline PipelineRunner) {
	p.pipeline = newPipeline
}
