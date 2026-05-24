# 02 — Mutex

When multiple goroutines touch the same memory and at least one writes, you have a **data race**. The result is undefined — corrupted values, weird crashes, or (often worse) "mostly right" answers that hide the bug.

A **mutex** (mutual exclusion lock) is the simplest fix: only one goroutine at a time may hold it, so only one at a time enters the critical section.

## What's in this folder

- `race/main.go` — broken version. Demonstrates lost updates.
- `fixed/main.go` — same logic, protected by `sync.Mutex`.

## Run it

```bash
# See the bug:
go run ./02-mutex/race

# See the bug detected by Go's built-in race detector:
go run -race ./02-mutex/race

# See it fixed:
go run ./02-mutex/fixed

# Confirm -race is now silent:
go run -race ./02-mutex/fixed
```

## The race detector is your best friend

`go run -race` (and `go test -race`) instruments memory accesses and reports unsynchronized read/write conflicts at runtime. **Always run tests with `-race` during development.** It catches bugs that would otherwise show up only under production load.

## Things to try

1. Replace `sync.Mutex` with `sync/atomic`'s `atomic.AddInt64`. Faster, lock-free, but only works for simple integer ops.
2. Change `mu.Lock()` to `mu.RLock()` (a `sync.RWMutex`). It compiles — but is it correct? (No: RLock is for readers, and we're writing.) This is a great example of "compiles fine, broken at runtime."
3. Forget to call `Unlock()` in some branch. Watch the program deadlock. Now you understand why `defer mu.Unlock()` is the standard pattern.

## Mental model

A mutex is just a flag plus a queue of waiters. `Lock` either sets the flag (if free) or parks the goroutine on the queue. `Unlock` clears the flag and wakes one waiter. Everything else in concurrency builds on this primitive.
