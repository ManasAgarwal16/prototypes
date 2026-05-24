// Prototype 02 (broken): a counter incremented from many goroutines, with NO synchronization.
//
// Expected behavior if everything were safe: final count == numGoroutines * incrementsPer.
// Actual behavior: final count is LESS, because `counter++` is not atomic.
//
// `counter++` is really three steps:
//   1. read counter into a register
//   2. add 1
//   3. write the register back to counter
// Two goroutines can read the same value, both add 1, both write back — one increment is lost.
//
// Run:                    go run ./02-mutex/race
// Run with race detector: go run -race ./02-mutex/race
//
// The race detector should flag this immediately.
package main

import (
	"fmt"
	"sync"
)

func main() {
	const numGoroutines = 100
	const incrementsPer = 1000

	var counter int
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPer; j++ {
				counter++ // <-- THE BUG
			}
		}()
	}

	wg.Wait()

	expected := numGoroutines * incrementsPer
	fmt.Printf("expected: %d\n", expected)
	fmt.Printf("got:      %d\n", counter)
	fmt.Printf("lost:     %d updates\n", expected-counter)
}
