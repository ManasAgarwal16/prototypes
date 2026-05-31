// Prototype 04 (basic): a worker pool.
//
// N long-lived "worker" goroutines pull jobs from a shared channel, process
// them, and send results to another channel. This is the canonical "fan-out
// / fan-in" pattern in Go and one of the most useful concurrency shapes you'll
// encounter.
//
// Why a worker pool rather than "spawn one goroutine per job"?
//   1. Bounded concurrency — only N workers run at once, regardless of how
//      many jobs you submit. Goroutines are cheap, but the resources they
//      use (file handles, sockets, CPU) are not.
//   2. Worker-local state — each worker can hold an expensive resource
//      (DB connection, HTTP client, big buffer) and reuse it across jobs.
//   3. Backpressure — a bounded jobs channel slows producers down when
//      workers can't keep up, rather than ballooning memory.
//
// The trickiest part isn't the workers, it's the SHUTDOWN coordination.
// Watch closely how `close()` and `sync.WaitGroup` work together below.
//
// Run:    go run ./04-worker-pool/basic
package main

import (
	"fmt"
	"sync"
	"time"
)

type Result struct {
	Input  int
	Output int
	Worker int
}

// worker takes:
//   - id: just for logging
//   - jobs: receive-only channel of int. `<-chan int` means "I can only receive
//     from this channel," enforced by the compiler. Catches "worker accidentally
//     sends" bugs at build time.
//   - results: send-only channel. `chan<- Result` means "I can only send."
//   - wg: shared WaitGroup so main knows when all workers are done.
func worker(id int, jobs <-chan int, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	// `for job := range jobs` reads values until the channel is closed AND
	// drained. When the loop exits, this goroutine returns and decrements the
	// WaitGroup. This is THE worker-pool idiom. Memorize it.
	for job := range jobs {
		// Simulate work.
		time.Sleep(100 * time.Millisecond)
		results <- Result{Input: job, Output: job * job, Worker: id}
	}
}

func main() {
	const (
		numWorkers = 3
		numJobs    = 10
	)

	// Buffered to avoid producer blocking on send if workers are slow.
	// Capacity = numJobs is generous; smaller capacity creates backpressure.
	jobs := make(chan int, numJobs)
	results := make(chan Result, numJobs)

	// Start the workers FIRST, before sending any jobs. Otherwise the unbuffered
	// case would deadlock (sender waits for a receiver that doesn't exist yet).
	// With buffered channels it still works but it's a good habit.
	var wg sync.WaitGroup
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg)
	}

	// Send all jobs. After the last one, CLOSE the channel.
	// Close is a signal: "no more values are coming." Workers' `range` loops
	// will drain remaining buffered items, then exit naturally.
	for j := 1; j <= numJobs; j++ {
		jobs <- j
	}
	close(jobs)
	// CRITICAL RULES about close:
	//   - Only the SENDER closes a channel. Receivers never close.
	//   - Closing a closed channel panics. Closing nil panics.
	//   - Sending to a closed channel panics.
	//   - Receiving from a closed channel returns the zero value immediately.

	// Now: how do we know when ALL results are in, so main can stop ranging?
	// The shutdown dance:
	//   1. We've closed `jobs` — workers will eventually drain and exit.
	//   2. We need to close `results` once all workers have exited.
	//   3. We can't close it inline here — main is about to range over results.
	// Solution: a separate goroutine that waits on the WaitGroup, then closes.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results. When the closer goroutine above runs `close(results)`,
	// this range loop exits.
	for r := range results {
		fmt.Printf("worker %d: %d^2 = %d\n", r.Worker, r.Input, r.Output)
	}

	fmt.Println("\nall jobs processed")
}
