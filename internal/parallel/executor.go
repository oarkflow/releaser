// Package parallel provides concurrent build execution with worker pools.
package parallel

import (
	"context"
	"runtime"
	"sync"

	"github.com/charmbracelet/log"
)

// Task represents a unit of work to be executed
type Task interface {
	// Name returns the task name for logging
	Name() string
	// Execute runs the task
	Execute(ctx context.Context) error
}

// TaskFunc is a function adapter for Task
type TaskFunc struct {
	name string
	fn   func(ctx context.Context) error
}

// NewTask creates a new TaskFunc
func NewTask(name string, fn func(ctx context.Context) error) *TaskFunc {
	return &TaskFunc{name: name, fn: fn}
}

func (t *TaskFunc) Name() string {
	return t.name
}

func (t *TaskFunc) Execute(ctx context.Context) error {
	return t.fn(ctx)
}

// Result represents the outcome of a task execution
type Result struct {
	Task  Task
	Error error
}

// Executor manages parallel task execution
type Executor struct {
	workers    int
	tasks      chan Task
	results    chan Result
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	failFast   bool
	onProgress func(completed, total int, task Task)
}

// ExecutorOption configures an Executor
type ExecutorOption func(*Executor)

// WithWorkers sets the number of workers
func WithWorkers(n int) ExecutorOption {
	return func(e *Executor) {
		if n > 0 {
			e.workers = n
		}
	}
}

// WithFailFast stops execution on first error
func WithFailFast(ff bool) ExecutorOption {
	return func(e *Executor) {
		e.failFast = ff
	}
}

// WithProgress sets a progress callback
func WithProgress(fn func(completed, total int, task Task)) ExecutorOption {
	return func(e *Executor) {
		e.onProgress = fn
	}
}

// NewExecutor creates a new parallel executor
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		workers: runtime.NumCPU(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs tasks in parallel and returns all results
func (e *Executor) Execute(ctx context.Context, tasks []Task) []Result {
	if len(tasks) == 0 {
		return nil
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	defer e.cancel()

	// Create buffered channels
	e.tasks = make(chan Task, len(tasks))
	e.results = make(chan Result, len(tasks))

	// Start workers
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}

	// Send tasks
	go func() {
		for _, task := range tasks {
			select {
			case e.tasks <- task:
			case <-e.ctx.Done():
				break
			}
		}
		close(e.tasks)
	}()

	// Wait for completion
	go func() {
		e.wg.Wait()
		close(e.results)
	}()

	// Collect results
	var results []Result
	completed := 0
	total := len(tasks)
	hasError := false

	for result := range e.results {
		results = append(results, result)
		completed++

		if result.Error != nil {
			hasError = true
			log.Error("Task failed", "task", result.Task.Name(), "error", result.Error)
			if e.failFast {
				e.cancel()
			}
		} else {
			log.Debug("Task completed", "task", result.Task.Name())
		}

		if e.onProgress != nil {
			e.onProgress(completed, total, result.Task)
		}
	}

	if hasError {
		log.Warn("Some tasks failed", "failed", countErrors(results), "total", total)
	}

	return results
}

// worker processes tasks from the task channel
func (e *Executor) worker(id int) {
	defer e.wg.Done()

	for task := range e.tasks {
		select {
		case <-e.ctx.Done():
			e.results <- Result{Task: task, Error: e.ctx.Err()}
			continue
		default:
		}

		log.Debug("Worker starting task", "worker", id, "task", task.Name())
		err := task.Execute(e.ctx)
		e.results <- Result{Task: task, Error: err}
	}
}

// countErrors counts the number of errors in results
func countErrors(results []Result) int {
	count := 0
	for _, r := range results {
		if r.Error != nil {
			count++
		}
	}
	return count
}

// HasErrors returns true if any result has an error
func HasErrors(results []Result) bool {
	return countErrors(results) > 0
}

// Errors returns all errors from results
func Errors(results []Result) []error {
	var errs []error
	for _, r := range results {
		if r.Error != nil {
			errs = append(errs, r.Error)
		}
	}
	return errs
}

// BatchExecutor processes items in batches
type BatchExecutor[T any] struct {
	workers   int
	batchSize int
}

// NewBatchExecutor creates a new batch executor
func NewBatchExecutor[T any](workers, batchSize int) *BatchExecutor[T] {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	return &BatchExecutor[T]{
		workers:   workers,
		batchSize: batchSize,
	}
}

// Process processes items in parallel batches
func (e *BatchExecutor[T]) Process(ctx context.Context, items []T, fn func(ctx context.Context, item T) error) error {
	if len(items) == 0 {
		return nil
	}

	// Create tasks
	tasks := make([]Task, len(items))
	for i, item := range items {
		item := item // capture
		tasks[i] = NewTask("item", func(ctx context.Context) error {
			return fn(ctx, item)
		})
	}

	// Execute
	executor := NewExecutor(WithWorkers(e.workers), WithFailFast(true))
	results := executor.Execute(ctx, tasks)

	// Check for errors
	for _, r := range results {
		if r.Error != nil {
			return r.Error
		}
	}

	return nil
}

// Pipeline represents a series of stages
type Pipeline struct {
	stages []Stage
}

// Stage represents a pipeline stage
type Stage struct {
	Name     string
	Tasks    []Task
	Parallel bool
}

// NewPipeline creates a new pipeline
func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// AddStage adds a stage to the pipeline
func (p *Pipeline) AddStage(name string, parallel bool, tasks ...Task) *Pipeline {
	p.stages = append(p.stages, Stage{
		Name:     name,
		Tasks:    tasks,
		Parallel: parallel,
	})
	return p
}

// Execute runs all stages in order
func (p *Pipeline) Execute(ctx context.Context, workers int) error {
	for _, stage := range p.stages {
		log.Info("Starting stage", "stage", stage.Name, "tasks", len(stage.Tasks))

		if stage.Parallel {
			executor := NewExecutor(WithWorkers(workers), WithFailFast(true))
			results := executor.Execute(ctx, stage.Tasks)
			if HasErrors(results) {
				return Errors(results)[0]
			}
		} else {
			// Sequential execution
			for _, task := range stage.Tasks {
				if err := task.Execute(ctx); err != nil {
					return err
				}
			}
		}

		log.Info("Stage completed", "stage", stage.Name)
	}
	return nil
}

// Semaphore limits concurrent operations
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a new semaphore
func NewSemaphore(limit int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, limit),
	}
}

// Acquire acquires a semaphore slot
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a semaphore slot
func (s *Semaphore) Release() {
	<-s.ch
}

// Map executes a function on each item in parallel and returns results
func Map[T any, R any](ctx context.Context, items []T, workers int, fn func(context.Context, T) (R, error)) ([]R, error) {
	if len(items) == 0 {
		return nil, nil
	}

	type result struct {
		index int
		value R
		err   error
	}

	sem := NewSemaphore(workers)
	results := make(chan result, len(items))
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, it T) {
			defer wg.Done()

			if err := sem.Acquire(ctx); err != nil {
				results <- result{index: idx, err: err}
				return
			}
			defer sem.Release()

			val, err := fn(ctx, it)
			results <- result{index: idx, value: val, err: err}
		}(i, item)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	output := make([]R, len(items))
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		output[r.index] = r.value
	}

	return output, nil
}

// ForEach executes a function on each item in parallel
func ForEach[T any](ctx context.Context, items []T, workers int, fn func(context.Context, T) error) error {
	_, err := Map(ctx, items, workers, func(ctx context.Context, item T) (struct{}, error) {
		return struct{}{}, fn(ctx, item)
	})
	return err
}
