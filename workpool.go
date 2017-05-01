// workpool allows a bounded set of async tasks to execute concurrently
package workpool

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrPoolFull        = errors.New("WorkPool is full")
	ErrPoolShutdown    = errors.New("WorkPool is shutdown")
	ErrFutureCompleted = errors.New("Future completed")
	ErrFutureTimeout   = errors.New("Timeout")
)

// Result holds the results of a task execution
type Result struct {
	Value interface{}
	Err   error
}

// Future is a container for the results of an async task that may or may not have completed yet
type Future interface {
	// Get waits for the completion of the async task for the given amount of time.
	// If the task completes within the timeout, result of the task is returned. Otherwise, ErrFutureTimeout
	// is returned as the error. Get can be called as many times as needed until the task completes.
	// When a call to Get has returned the results of the completed task, subsequent calls to Get will
	// return ErrFutureCompleted as an error.
	Get(timeout time.Duration) (interface{}, error)

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

func (rh *resultHolder) Get(timeout time.Duration) (interface{}, error) {
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

func (rh *resultHolder) Cancel() {
	rh.cancelFunc()
}

type Task func(context.Context) *Result

type taskWrapper struct {
	ctx        context.Context
	task       Task
	resultChan chan<- *Result
}

type WorkPool struct {
	mu       sync.RWMutex
	wg       sync.WaitGroup
	shutdown bool
	queue    chan *taskWrapper
	tokens   chan struct{}
}

// NewWorkPool creates a new work pool that will execute at most maxGoRoutines concurrently and hold queueSize
// tasks in the backlog awaiting an execution slot.
func NewWorkPool(maxGoRoutines, queueSize int) *WorkPool {
	wp := &WorkPool{
		queue:  make(chan *taskWrapper, queueSize+maxGoRoutines),
		tokens: make(chan struct{}, queueSize+maxGoRoutines),
	}

	for i := 0; i < maxGoRoutines; i++ {
		wp.wg.Add(1)
		go wp.doTask()
	}
	return wp
}

func (wp *WorkPool) doTask() {
	for t := range wp.queue {
		select {
		case <-t.ctx.Done():
			t.resultChan <- &Result{Err: t.ctx.Err()}
		default:
			t.resultChan <- t.task(t.ctx)
		}
		close(t.resultChan)
		<-wp.tokens
	}
	wp.wg.Done()
}

// Submit attempts to enqueue the given task. If the WorkPool queue has enough space, it will be accepted and
// executed as soon as it reaches the front of the queue. The context parameter can be used to cancel the task
// execution if it has been in the queue for too long. It will also be passed to the task at the start of execution.
// Returns ErrPoolFull when the queue is full. If the WorkPool is shutdown, ErrPoolShutdown will be returned.
func (wp *WorkPool) Submit(ctx context.Context, task Task) (Future, error) {
	if wp.IsShutdown() {
		return nil, ErrPoolShutdown
	}

	select {
	case wp.tokens <- struct{}{}:
		resultChan := make(chan *Result, 1)
		newCtx, cancelFunc := context.WithCancel(ctx)
		wp.queue <- &taskWrapper{ctx: newCtx, task: task, resultChan: resultChan}
		return &resultHolder{cancelFunc: cancelFunc, resultChan: resultChan}, nil
	default:
		return nil, ErrPoolFull
	}
}

// IsShutdown returns true if the pool has been shutdown
func (wp *WorkPool) IsShutdown() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.shutdown
}

// Shutdown gracefully shuts down the WorkPool. It stops accepting any new tasks but will continue executing
// the already enqueued tasks to their completion. To avoid processing the queue, cancel the contexts associated
// with the enqueued tasks.
// Setting the wait parameter to true will cause the call to block until the WorkPool finishes shutting down.
func (wp *WorkPool) Shutdown(wait bool) {
	wp.mu.Lock()

	if !wp.shutdown {
		close(wp.queue)
		close(wp.tokens)
		wp.shutdown = true
		wp.mu.Unlock()
		if wait {
			wp.wg.Wait()
		}
	} else {
		wp.mu.Unlock()
	}
}
