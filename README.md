WorkQueue
========

[![GoDoc](https://godoc.org/github.com/charithe/workqueue?status.svg)](https://godoc.org/github.com/charithe/workqueue)

A Go library for queuing and  executing a set of tasks with a user-defined concurrency level. Includes support 
for timing out and/or cancelling the tasks in the queue. 

Provides a primitive `Future`-like abstraction for obtaining the results of a task.


```
go get github.com/charithe/workqueue
```

Usage
-----

```go
wq := workqueue.New(8, 16)
defer wq.Shutdown(true)

// When the task reaches the front of the queue, the associated context will be used to determine whether
// the task should be executed or not. If the context hasn't been cancelled, the task will be started and
// the context will be passed to it as the argument.
ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
defer cancelFunc()

f, err := wq.Submit(ctx, func(c context.Context) *workqueue.Result {
    // do work
    // in case of error, return workqueue.ErrorResult(err) instead
    return workqueue.SuccessResult("result")
})

// If the number of queued tasks exceed the limit, ErrQueueFull will be returned
if err == workqueue.ErrQueueFull {
    fmt.Println("Pool queue is full")
    return
}

// Wait for the task to complete for 10 seconds
v, err := f.GetWithTimeout(10 * time.Second)
if err != nil {
    if err == workqueue.ErrFutureTimeout {
        fmt.Println("Timed out waiting for result")
    } else {
        fmt.Printf("Task failed: %+v\n", err)
    }
    return
}

fmt.Printf("Task result: %s\n", v)
```


