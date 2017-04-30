package workpool

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func ExampleWorkPool_Submit() {
	wp := NewWorkPool(8)
	f, err := wp.Submit(func() *Result {
		// do work
		// in case of error, return &Result{Err: err} instead
		return &Result{Value: "result"}
	})

	// If the number of spawned goroutines exceed the limit, ErrPoolFull will be returned
	if err == ErrPoolFull {
		fmt.Println("Pool is full")
		return
	}

	// Wait for the task to complete for 10 seconds
	v, err := f.Get(10 * time.Second)
	if err != nil {
		if err == ErrTimeout {
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
		timeout     time.Duration
		task        Task
		expectedVal interface{}
		expectedErr error
	}{
		{
			desc:        "Successful task execution",
			timeout:     5 * time.Millisecond,
			task:        func() *Result { return &Result{Value: "value"} },
			expectedVal: "value",
		},
		{
			desc:        "Task timeout",
			timeout:     5 * time.Millisecond,
			task:        func() *Result { time.Sleep(10 * time.Millisecond); return &Result{Value: "value"} },
			expectedErr: ErrTimeout,
		},
	}

	wp := NewWorkPool(2)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			f, err := wp.Submit(tc.task)
			assert.NoError(t, err)
			assert.NotNil(t, f)

			val, err := f.Get(tc.timeout)
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

func TestWorkPoolSize(t *testing.T) {
	wp := NewWorkPool(2)
	f1, err := wp.Submit(func() *Result { time.Sleep(20 * time.Millisecond); return &Result{Value: "value1"} })
	assert.NoError(t, err)

	f2, err := wp.Submit(func() *Result { time.Sleep(20 * time.Millisecond); return &Result{Value: "value2"} })
	assert.NoError(t, err)

	_, err = wp.Submit(func() *Result { time.Sleep(20 * time.Millisecond); return &Result{Value: "value3"} })
	assert.Error(t, err)
	assert.EqualError(t, err, ErrPoolFull.Error())

	v1, err := f1.Get(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value1", v1)

	v2, err := f2.Get(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value2", v2)

	f3, err := wp.Submit(func() *Result { time.Sleep(10 * time.Millisecond); return &Result{Value: "value3"} })
	assert.NoError(t, err)

	v3, err := f3.Get(1 * time.Second)
	assert.NoError(t, err)
	assert.EqualValues(t, "value3", v3)
}
