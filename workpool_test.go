package workpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func ExampleWorkPool_Submit() {
	wp := New(8, 16)
	defer wp.Shutdown(true)

	// When the task reaches the front of the queue, the associated context will be used to determine whether
	// the task should be executed or not. If the context hasn't been cancelled, the task will be started and
	// the context will be passed to it as the argument.
	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunc()

	f, err := wp.Submit(ctx, func(c context.Context) *Result {
		// do work
		// in case of error, return ErrorResult(err) instead
		return SuccessResult("result")
	})

	// If the number of queued tasks exceed the limit, ErrPoolFull will be returned
	if err == ErrPoolFull {
		fmt.Println("Pool queue is full")
		return
	}

	// Wait for the task to complete for 10 seconds
	v, err := f.GetWithTimeout(10 * time.Second)
	if err != nil {
		if err == ErrFutureTimeout {
			fmt.Println("Timed out waiting for result")
		} else {
			fmt.Printf("Task failed: %+v\n", err)
		}
		return
	}

	fmt.Printf("Task result: %s\n", v)
}

func TestTaskExecution(t *testing.T) {
	testCases := []struct {
		desc        string
		ctxTimeout  time.Duration
		getTimeout  time.Duration
		task        Task
		expectedVal interface{}
		expectedErr error
	}{
		{
			desc:        "Successful task execution",
			ctxTimeout:  5 * time.Millisecond,
			getTimeout:  5 * time.Millisecond,
			task:        func(ctx context.Context) *Result { return SuccessResult("value") },
			expectedVal: "value",
		},
		{
			desc:        "Context timeout",
			ctxTimeout:  0 * time.Millisecond,
			getTimeout:  5 * time.Millisecond,
			task:        func(ctx context.Context) *Result { return SuccessResult("value") },
			expectedErr: context.DeadlineExceeded,
		},
		{
			desc:        "Future GetWithTimeout",
			ctxTimeout:  5 * time.Millisecond,
			getTimeout:  5 * time.Millisecond,
			task:        func(ctx context.Context) *Result { time.Sleep(10 * time.Millisecond); return SuccessResult("value") },
			expectedErr: ErrFutureTimeout,
		},
	}

	wp := New(2, 2)
	defer wp.Shutdown(true)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancelFunc := context.WithTimeout(context.Background(), tc.ctxTimeout)
			defer cancelFunc()

			f, err := wp.Submit(ctx, tc.task)
			assert.NoError(t, err)
			assert.NotNil(t, f)

			val, err := f.GetWithTimeout(tc.ctxTimeout)
			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tc.expectedVal, val)
			}
		})
	}
}

func TestFutureReuse(t *testing.T) {
	wp := New(1, 1)
	defer wp.Shutdown(true)

	f1, err := wp.Submit(context.Background(), func(c context.Context) *Result { return SuccessResult("value") })
	assert.NoError(t, err)

	v1, err := f1.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value", v1)

	v1, err = f1.GetWithTimeout(1 * time.Second)
	assert.Error(t, err)
	assert.EqualError(t, err, ErrFutureCompleted.Error())
}

func TestFutureCancellation(t *testing.T) {
	wp := New(1, 1)
	defer wp.Shutdown(true)

	f1, err := wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(20 * time.Millisecond); return SuccessResult("value1") })
	assert.NoError(t, err)

	f2, err := wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(10 * time.Millisecond); return SuccessResult("value2") })
	assert.NoError(t, err)

	f2.Cancel()

	v1, err := f1.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value1", v1)

	_, err = f2.GetWithTimeout(1 * time.Second)
	assert.Error(t, err)
	assert.EqualError(t, err, context.Canceled.Error())
}

func TestWorkPoolSizeConstraints(t *testing.T) {
	wp := New(1, 1)
	defer wp.Shutdown(true)

	f1, err := wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(20 * time.Millisecond); return SuccessResult("value1") })
	assert.NoError(t, err)

	f2, err := wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(20 * time.Millisecond); return SuccessResult("value2") })
	assert.NoError(t, err)

	_, err = wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(20 * time.Millisecond); return SuccessResult("value3") })
	assert.Error(t, err)
	assert.EqualError(t, err, ErrPoolFull.Error())

	v1, err := f1.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value1", v1)

	v2, err := f2.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value2", v2)

	f3, err := wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(10 * time.Millisecond); return SuccessResult("value3") })
	assert.NoError(t, err)

	v3, err := f3.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value3", v3)
}

func TestWorkPoolShutdown(t *testing.T) {
	wp := New(1, 2)

	f1, err := wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(10 * time.Millisecond); return SuccessResult("value1") })
	assert.NoError(t, err)

	f2, err := wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(10 * time.Millisecond); return SuccessResult("value2") })
	assert.NoError(t, err)

	wp.Shutdown(true)

	_, err = wp.Submit(context.Background(), func(c context.Context) *Result { time.Sleep(10 * time.Millisecond); return SuccessResult("value3") })
	assert.Error(t, err)
	assert.EqualError(t, err, ErrPoolShutdown.Error())

	v1, err := f1.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value1", v1)

	v2, err := f2.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value2", v2)
}
