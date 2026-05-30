// Prototype 03 (channel version): semaphore limiting concurrency to N.
//
// A semaphore generalizes a mutex: a mutex allows 1 goroutine at a time into
// a critical section, a semaphore allows N. Useful for limiting concurrent
// access to a finite resource — e.g. "no more than 5 in-flight HTTP requests."
//
// The idiomatic Go trick: a buffered channel of capacity N IS a semaphore.
//   - Acquire = send into the channel. Blocks if the buffer is full.
//   - Release = receive from the channel. Frees a slot.
//
// The values you send don't matter; only the *count* of in-flight values does.
// By convention we use `struct{}{}` (zero-size, allocates nothing).
//
// Run:    go run ./03-semaphore/chan
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// `chan struct{}` of capacity N is the semaphore.
type semaphore chan struct{}

func newSem(n int) semaphore { return make(semaphore, n) }

func (s semaphore) acquire() { s <- struct{}{} }     // blocks when full
func (s semaphore) release() { <-s }                 // frees one slot

func main() {
	const (
		numWorkers = 10
		maxInFlight = 3
	)

	sem := newSem(maxInFlight)

	// Track how many are actually running concurrently, so we can prove
	// the semaphore is doing its job.
	var inFlight atomic.Int32
	var maxObserved atomic.Int32

	var wg sync.WaitGroup
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("worker %2d: waiting to work...\n", id)
			sem.acquire()
			fmt.Printf("worker %2d: acquired semaphore\n", id)
			defer sem.release()

			// Critical section: at most `maxInFlight` goroutines here at once.
			now := inFlight.Add(1)
			// Update maxObserved if `now` is a new high-water mark.
			// (Loop because another goroutine could change it between Load and CAS.)
			for {
				cur := maxObserved.Load()
				if now <= cur || maxObserved.CompareAndSwap(cur, now) {
					break
				}
			}

			fmt.Printf("worker %2d: working  (in-flight=%d)\n", id, now)
			time.Sleep(200 * time.Millisecond) // pretend to do work
			fmt.Printf("worker %2d: done\n", id)

			inFlight.Add(-1)
		}(i)
	}

	wg.Wait()
	fmt.Printf("\nmax concurrency observed: %d (limit was %d)\n", maxObserved.Load(), maxInFlight)
}
