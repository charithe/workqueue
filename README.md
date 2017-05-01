WorkPool
========

[![GoDoc](https://godoc.org/github.com/charithe/workpool?status.svg)](https://godoc.org/github.com/charithe/workpool)

A Go library for queuing and  executing a set of tasks with a user-defined concurrency level. Includes support 
for timing out and/or cancelling the tasks in the queue. 
Provides a primitive `Future`-like abstraction for obtaining the results of a task.


```
go get github.com/charithe/workpool
```

Usage
-----

```go
wp := NewWorkPool(8, 16)
defer wp.Shutdown(true)

// When the task reaches the front of the queue, the associated context will be used to determine whether
// the task should be executed or not. If the context hasn't been cancelled, the task will be started and
// the context will be passed to it as the argument.
ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
defer cancelFunc()

f, err := wp.Submit(ctx, func(c context.Context) *Result {
    // do work
    // in case of error, return &Result{Err: err} instead
    return &Result{Value: "result"}
})

// If the number of queued tasks exceed the limit, ErrPoolFull will be returned
if err == ErrPoolFull {
    fmt.Println("Pool queue is full")
    return
}

// Wait for the task to complete for 10 seconds
v, err := f.Get(10 * time.Second)
if err != nil {
    if err == ErrFutureTimeout {
        fmt.Println("Timed out waiting for result")
    } else {
        fmt.Printf("Task failed: %+v\n", err)
    }
    return
}

fmt.Printf("Task result: %s\n", v)
```


