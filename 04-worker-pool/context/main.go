// Prototype 04 (context): worker pool with cancellation via context.Context.
//
// The basic pool runs to completion. Real systems need to STOP — on a timeout,
// on a signal (SIGINT), on a parent operation being canceled. The Go idiom
// for this is `context.Context`.
//
// A Context carries a deadline + a cancellation signal. It's passed down call
// chains; child operations honor it by selecting on `ctx.Done()`. When the
// parent cancels, every downstream goroutine learns about it and bails out.
//
// In this prototype: we cap the entire pool with a 350ms timeout. Each worker
// takes ~100ms per job, so we expect ~3 jobs to complete before cancellation
// kicks in.
//
// Run:    go run ./04-worker-pool/context
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Result struct {
	Input  int
	Output int
	Worker int
}

func worker(ctx context.Context, id int, jobs <-chan int, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		// `select` lets a goroutine wait on multiple channel operations at once,
		// proceeding with whichever is ready first. If multiple are ready, one
		// is chosen pseudo-randomly.
		select {
		case <-ctx.Done():
			// Parent canceled (timeout or explicit cancel). Stop immediately.
			// We don't try to drain the jobs channel — abandoning work is OK
			// because the caller already gave up.
			return

		case job, ok := <-jobs:
			// `ok` is false when the channel is closed and drained. That's
			// our other exit signal: producer is done, no more work coming.
			if !ok {
				return
			}

			// Simulate work. Note: if the context cancels DURING this sleep,
			// we'll still finish the current job. For long jobs you'd pass
			// ctx down so the work itself can be canceled mid-flight.
			time.Sleep(100 * time.Millisecond)

			// When sending the result, also watch for cancellation — otherwise
			// we could block forever sending to results if the consumer gave up.
			select {
			case results <- Result{Input: job, Output: job * job, Worker: id}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func main() {
	const (
		numWorkers = 3
		numJobs    = 20
	)

	// Build a context that cancels after 350ms. With 3 workers doing 100ms
	// jobs in parallel, we expect ~9 jobs done before the deadline hits.
	// (3 workers × 3 rounds × 100ms = 900ms; cut off at 350ms ≈ 3 rounds.)
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel() // always defer cancel — frees timer resources even on early return

	jobs := make(chan int, numJobs)
	results := make(chan Result, numJobs)

	var wg sync.WaitGroup
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(ctx, w, jobs, results, &wg)
	}

	// Producer also watches the context — don't keep stuffing jobs into the
	// channel if we've been canceled.
	go func() {
		defer close(jobs)
		for j := 1; j <= numJobs; j++ {
			select {
			case jobs <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Closer goroutine: same pattern as basic/.
	go func() {
		wg.Wait()
		close(results)
	}()

	completed := 0
	for r := range results {
		fmt.Printf("worker %d: %d^2 = %d\n", r.Worker, r.Input, r.Output)
		completed++
	}

	fmt.Printf("\ncompleted %d of %d jobs before cancellation (%v)\n", completed, numJobs, ctx.Err())
}
