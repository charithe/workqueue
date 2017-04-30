WorkPool
========

[![GoDoc](https://godoc.org/github.com/charithe/workpool?status.svg)](https://godoc.org/github.com/charithe/workpool)

A Golang library for executing a bounded number of tasks concurrently. 


```go
// Create new workpool that will execute at most 8 concurrent tasks
wp := NewWorkPool(8)

// Submit a task
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
```


