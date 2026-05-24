// Prototype 02 (fixed): the same counter, protected by sync.Mutex.
//
// A Mutex (mutual exclusion lock) ensures that only one goroutine at a time
// can execute the code between Lock() and Unlock(). That region is called a
// "critical section."
//
// Pattern:
//   mu.Lock()
//   defer mu.Unlock()   // releases the lock when the function returns
//   // ... touch shared state ...
//
// Run:                    go run ./02-mutex/fixed
// Run with race detector: go run -race ./02-mutex/fixed
//
// Output should always match expected, and -race should NOT complain.
package main

import (
	"fmt"
	"sync"
)

func main() {
	const numGoroutines = 100
	const incrementsPer = 1000

	var counter int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPer; j++ {
				mu.Lock()
				counter++
				mu.Unlock()
				// Note: not using `defer mu.Unlock()` inside the loop —
				// defer only runs when the function returns, so deferring
				// inside a hot loop would hold the lock for the whole loop.
			}
		}()
	}

	wg.Wait()

	expected := numGoroutines * incrementsPer
	fmt.Printf("expected: %d\n", expected)
	fmt.Printf("got:      %d\n", counter)
}
