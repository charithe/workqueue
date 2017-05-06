// workpool allows a bounded set of async tasks to execute concurrently
package workqueue

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrQueueFull       = errors.New("Queue is full")
	ErrQueueShutdown   = errors.New("Queue is shutdown")
	ErrFutureCompleted = errors.New("Future completed")
	ErrFutureTimeout   = errors.New("Timeout")
)

// Result holds the results of a task execution
type Result struct {
	Value interface{}
	Err   error
}

func SuccessResult(value interface{}) *Result {
	return &Result{Value: value}
}

func ErrorResult(err error) *Result {
	return &Result{Err: err}
}

// Future is a container for the results of an async task that may or may not have completed yet
type Future interface {
	// GetWithTimeout waits for the completion of the async task for the given amount of time.
	// If the task completes within the timeout, result of the task is returned. Otherwise, ErrFutureTimeout
	// is returned as the error. Get can be called as many times as needed until the task completes.
	// When a call to Get has returned the results of the completed task, subsequent calls to Get will
	// return ErrFutureCompleted as an error.
	GetWithTimeout(timeout time.Duration) (interface{}, error)

	// GetWithContext waits for the completion of the async task until the context times out or is cancelled.
	// The behaviour when this function is called multiple times is identical to GetWithTimeout.
	GetWithContext(ctx context.Context) (interface{}, error)

	// Cancel attempts to cancel the async task. This is a best efforts attempt to perform the cancellation.
	// If the task is still in the queue, its' context will be cancelled, causing the WorkPool to reject the
	// task. If the task is already executing, then it is the user's responsibility to ensure that it honours
	// the cancellation of the context passed to it by the runtime.
	Cancel()
}

type resultHolder struct {
	cancelFunc context.CancelFunc
	resultChan chan *Result
}

func (rh *resultHolder) GetWithTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case r, ok := <-rh.resultChan:
		if ok {
			rh.cancelFunc()
			return r.Value, r.Err
		} else {
			return nil, ErrFutureCompleted
		}
	case <-time.After(timeout):
		return nil, ErrFutureTimeout
	}
}

func (rh *resultHolder) GetWithContext(ctx context.Context) (interface{}, error) {
	select {
	case r, ok := <-rh.resultChan:
		if ok {
			rh.cancelFunc()
			return r.Value, r.Err
		} else {
			return nil, ErrFutureCompleted
		}
	case <-ctx.Done():
		return nil, ErrFutureTimeout
	}
}

func (rh *resultHolder) Cancel() {
	rh.cancelFunc()
}

type Task func(context.Context) *Result

type taskWrapper struct {
	ctx        context.Context
	task       Task
	resultChan chan<- *Result
}

type WorkQueue struct {
	mu       sync.RWMutex
	wg       sync.WaitGroup
	shutdown bool
	queue    chan *taskWrapper
	tokens   chan struct{}
}

// New creates a new WorkQueue that will execute at most maxGoRoutines concurrently and hold queueSize
// tasks in the backlog awaiting an execution slot.
func New(maxGoRoutines, queueSize int) *WorkQueue {
	wq := &WorkQueue{
		queue:  make(chan *taskWrapper, queueSize+maxGoRoutines),
		tokens: make(chan struct{}, queueSize+maxGoRoutines),
	}

	for i := 0; i < maxGoRoutines; i++ {
		wq.wg.Add(1)
		go wq.doTask()
	}
	return wq
}

func (wq *WorkQueue) doTask() {
	for t := range wq.queue {
		select {
		case <-t.ctx.Done():
			t.resultChan <- &Result{Err: t.ctx.Err()}
		default:
			t.resultChan <- t.task(t.ctx)
		}
		close(t.resultChan)
		<-wq.tokens
	}
	wq.wg.Done()
}

// Submit attempts to enqueue the given task. If the queue has enough space, it will be accepted and
// executed as soon as it reaches the front of the queue. The context parameter can be used to cancel the task
// execution if it has been in the queue for too long. It will also be passed to the task at the start of execution.
// Returns ErrQueueFull when the queue is full. If the WorkQueue is shutdown, ErrQueueShutdown will be returned.
func (wq *WorkQueue) Submit(ctx context.Context, task Task) (Future, error) {
	if wq.IsShutdown() {
		return nil, ErrQueueShutdown
	}

	select {
	case wq.tokens <- struct{}{}:
		resultChan := make(chan *Result, 1)
		newCtx, cancelFunc := context.WithCancel(ctx)
		wq.queue <- &taskWrapper{ctx: newCtx, task: task, resultChan: resultChan}
		return &resultHolder{cancelFunc: cancelFunc, resultChan: resultChan}, nil
	default:
		return nil, ErrQueueFull
	}
}

// IsShutdown returns true if the queue has been shutdown
func (wq *WorkQueue) IsShutdown() bool {
	wq.mu.RLock()
	defer wq.mu.RUnlock()
	return wq.shutdown
}

// Shutdown gracefully shuts down the queue. It stops accepting any new tasks but will continue executing
// the already enqueued tasks to their completion. To avoid processing the queue, cancel the contexts associated
// with the enqueued tasks.
// Setting the wait parameter to true will cause the call to block until the queue finishes shutting down.
func (wq *WorkQueue) Shutdown(wait bool) {
	wq.mu.Lock()

	if !wq.shutdown {
		close(wq.queue)
		close(wq.tokens)
		wq.shutdown = true
		wq.mu.Unlock()
		if wait {
			wq.wg.Wait()
		}
	} else {
		wq.mu.Unlock()
	}
}
