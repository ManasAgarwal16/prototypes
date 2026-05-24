// Prototype 01: Goroutines warmup
//
// Goal: see goroutines run concurrently, and learn to wait for them to finish.
//
// A goroutine is a lightweight thread managed by the Go runtime.
// You start one by writing `go someFunc()`. The runtime multiplexes thousands
// of goroutines onto a small number of OS threads.
// Run:    go run ./01-goroutines
package main

import (
	"fmt"
	"sync"
	"time"
)

func worker(id int, wg *sync.WaitGroup) {
	// `defer wg.Done()` runs when this function returns, signaling
	// the WaitGroup that this worker is finished. Always pair Add with Done.
	defer wg.Done()

	fmt.Printf("worker %d: starting\n", id)
	time.Sleep(time.Duration(id) * 100 * time.Millisecond)
	fmt.Printf("worker %d: done\n", id)
}

func main() {
	// WaitGroup is a counter. Add(n) increments it, Done() decrements it,
	// Wait() blocks until it hits zero. It's the simplest way to wait for
	// a known number of goroutines to finish.
	var wg sync.WaitGroup

	const n = 100000
	for i := 1; i <= n; i++ {
		wg.Add(1)
		go worker(i, &wg)
	}

	fmt.Println("main: launched all workers, waiting...")
	wg.Wait()
	fmt.Println("main: all workers done")
}
