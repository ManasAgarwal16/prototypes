# 03 — Semaphore

A **semaphore** is a counter with two operations:

- `Acquire()` — if the counter is > 0, decrement and proceed. If 0, block until someone releases.
- `Release()` — increment the counter; wake one blocked acquirer if any.

It generalizes a mutex: a mutex is just a semaphore with capacity 1. A semaphore with capacity N lets up to N goroutines into the critical section simultaneously.

## When you'd use one

You have a resource that can support some concurrency, but not unlimited concurrency:

- **Concurrent HTTP requests** — your API tolerates 100 in-flight, not 10,000.
- **File descriptors** — the OS limits how many you can open.
- **Database connections** — your pool has, say, 20 slots.
- **CPU-bound work** — running more goroutines than cores just adds overhead.
- **API rate limits** — third-party APIs cap concurrent callers.

The pattern is the same every time: wrap the actual work in `Acquire()` / `defer Release()`.

## What's in this folder

- `chan/main.go` — idiomatic Go: a **buffered channel of capacity N is a semaphore**. The whole implementation is `make(chan struct{}, N)` and the two channel operations.
- `manual/main.go` — built from scratch with `sync.Mutex` + `sync.Cond`. Same API, same demo, more code. This is what you'd write in C with pthreads.

Both run the same workload: 10 workers, limit of 3 concurrent. Both print "max concurrency observed: 3" at the end.

## Run

```bash
go run ./03-semaphore/chan
go run ./03-semaphore/manual
```

Watch the output — workers acquire, work, release in waves. You'll see the in-flight count rise to 3 and stay there.

## Which one is "right"?

In Go, **use the channel version.** It's three lines, the runtime is doing the parking/waking for you, and channels integrate with `select` for free (you can add timeouts, cancellation, etc. without writing more code).

The manual version exists for two reasons:
1. **You should understand condition variables.** They're the canonical "wait until a predicate becomes true" primitive across every language with threads. If you only know channels, large swathes of systems literature will read like a foreign language.
2. **Sometimes you need them.** A semaphore is a one-dimensional condition ("count < capacity"). Many real-world coordination problems have multi-dimensional conditions that channels can't express cleanly — e.g. "wait until the queue has at least 5 items AND a writer slot is free." Condvars handle that with a `for !readyToProceed() { cond.Wait() }` loop.

## Things to try

**1. Crank the workers up.** Change `numWorkers` to 1000 with a `maxInFlight` of 50. Watch how the channel version still uses near-zero memory. Goroutines waiting on a channel are parked by the runtime — they don't consume CPU.

**2. See the difference between Signal and Broadcast.** In `manual/main.go`, replace `s.cond.Signal()` with `s.cond.Broadcast()`. It still works correctly (broadcast is always safe), but it's wasteful — every waiter wakes up, re-checks the condition, and most go back to sleep. This is the **thundering herd** problem. `Signal` is better when only one waiter can proceed; `Broadcast` is needed when the state change might unblock multiple waiters at once.

**3. Try the `golang.org/x/sync/semaphore` package.** The standard library has a richer semaphore in `x/sync`: it supports acquiring N at once (weighted semaphores) and cancellation via `context.Context`:

```go
import "golang.org/x/sync/semaphore"

sem := semaphore.NewWeighted(10)
if err := sem.Acquire(ctx, 3); err != nil { ... } // grab 3 slots, cancelable
defer sem.Release(3)
```

This is what production Go code usually reaches for.

**4. Build a `TryAcquire`.** Add a method that returns immediately with a bool — "got a slot? true/false, no blocking." For the channel version it's a non-blocking `select` with a `default`. For the manual version it's `Lock`, check, optionally bump, `Unlock`. Subtle but useful for non-critical work that should be skipped under load.

## Mental model

The bathroom metaphor still works — but now it's a public restroom with N stalls:

- **Capacity** = number of stalls.
- **Acquire** = take a free stall, or wait in the hall if all are occupied.
- **Release** = leave the stall (and signal the next person in the hall).
- **Mutex** = a single-stall bathroom (the N=1 special case).

Everything you learned about mutexes — critical section, contention, fairness — applies, just generalized to N.

## Where this goes next

Semaphores are the building block for **bounded worker pools** (prototype 04) and **connection pools** (prototype 05). A connection pool is essentially "a semaphore guarding a stash of pre-built objects." Once you understand semaphores, those don't look very mysterious.
