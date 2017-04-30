// workpool allows a bounded set of async tasks to execute concurrently
package workpool

import (
	"fmt"
	"time"
)

var (
	ErrPoolFull        = fmt.Errorf("WorkPool is full")
	ErrFutureCompleted = fmt.Errorf("Future completed")
	ErrTimeout         = fmt.Errorf("Timeout")
)

// Result holds the results of a task execution
type Result struct {
	Value interface{}
	Err   error
}

// Future is a container for the results of an async task that may or may not have completed yet
type Future interface {
	// Get waits for the completion of the async task for the given amount of time.
	// If the task completes within the timeout, result of the task is returned. Otherwise, ErrTimeout
	// is returned as the error. Get can be called as many times as needed until the task completes.
	// When a call to Get has returned the results of the completed task, subsequent calls to Get will
	// return ErrFutureCompleted as an error.
	Get(timeout time.Duration) (interface{}, error)
}

type resultHolder struct {
	resultChan chan *Result
}

func (rh *resultHolder) Get(timeout time.Duration) (interface{}, error) {
	select {
	case r, ok := <-rh.resultChan:
		if ok {
			return r.Value, r.Err
		} else {
			return nil, ErrFutureCompleted
		}
	case <-time.After(timeout):
		return nil, ErrTimeout
	}
}

type Task func() *Result

type WorkPool struct {
	tokens chan struct{}
}

// NewWorkPool creates a new work pool that will execute at most "size" number of goroutines concurrently
func NewWorkPool(size int) *WorkPool {
	return &WorkPool{
		tokens: make(chan struct{}, size),
	}
}

// Submit attempts to execute the given task, provided that the concurrent goroutine limit is not exceeded.
// Returns ErrPoolFull is returned when the goroutine constraint is reached.
func (wp *WorkPool) Submit(task Task) (Future, error) {
	select {
	case wp.tokens <- struct{}{}:
		resultChan := make(chan *Result)
		go wp.doTask(resultChan, task)
		return &resultHolder{resultChan: resultChan}, nil
	default:
		return nil, ErrPoolFull
	}
}

func (wp *WorkPool) doTask(resultChan chan<- *Result, task Task) {
	resultChan <- task()
	close(resultChan)
	<-wp.tokens
}
