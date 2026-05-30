// Prototype 03 (manual version): semaphore built from scratch using
// sync.Mutex + sync.Cond. This is the "OS textbook" implementation —
// the same approach you'd write in C with pthreads.
//
// The state is two integers:
//   - capacity: max allowed concurrent holders
//   - count:    how many are currently holding
//
// Acquire:
//   lock the mutex
//   while count == capacity: wait on the condition variable
//                            (Wait atomically unlocks + sleeps, then re-locks)
//   count++
//   unlock
//
// Release:
//   lock
//   count--
//   signal one waiter (if any)
//   unlock
//
// sync.Cond is Go's condition variable. It pairs with a Mutex and provides:
//   Wait()       — atomically unlock the mutex and block. When woken, re-lock.
//   Signal()     — wake one waiter (if any).
//   Broadcast()  — wake all waiters.
//
// Run:    go run ./03-semaphore/manual
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Sem struct {
	mu       sync.Mutex
	cond     *sync.Cond
	capacity int
	count    int
}

func NewSem(capacity int) *Sem {
	s := &Sem{capacity: capacity}
	s.cond = sync.NewCond(&s.mu) // condvar is attached to s.mu
	return s
}

func (s *Sem) Acquire() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// MUST be a `for`, not an `if`. Spurious wakeups are allowed in theory,
	// and even without them, between Signal() and our re-locking, another
	// goroutine could have re-acquired and re-filled the slot. Always re-check
	// the condition after waking. This pattern is universal across languages.
	for s.count == s.capacity {
		s.cond.Wait()
	}
	s.count++
}

func (s *Sem) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count--
	s.cond.Signal() // wake one waiter (if any). No-op if none waiting.
}

func main() {
	const (
		numWorkers  = 10
		maxInFlight = 3
	)

	sem := NewSem(maxInFlight)

	var inFlight atomic.Int32
	var maxObserved atomic.Int32

	var wg sync.WaitGroup
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			sem.Acquire()
			defer sem.Release()

			now := inFlight.Add(1)
			for {
				cur := maxObserved.Load()
				if now <= cur || maxObserved.CompareAndSwap(cur, now) {
					break
				}
			}

			fmt.Printf("worker %2d: working  (in-flight=%d)\n", id, now)
			time.Sleep(200 * time.Millisecond)
			fmt.Printf("worker %2d: done\n", id)

			inFlight.Add(-1)
		}(i)
	}

	wg.Wait()
	fmt.Printf("\nmax concurrency observed: %d (limit was %d)\n", maxObserved.Load(), maxInFlight)
}
