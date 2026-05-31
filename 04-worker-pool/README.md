# 04 — Worker pool

A **worker pool** is a fixed number of long-lived goroutines that consume jobs from a shared channel. Producers send jobs in; workers process them; results (if any) come out on another channel.

This is **the** workhorse pattern of concurrent Go. Almost every production Go service has one somewhere: handling HTTP requests, processing background jobs, draining a queue, fanning out to backends.

## What's in this folder

- `basic/main.go` — canonical fan-out/fan-in pool. 3 workers, 10 jobs, proper shutdown via `close(jobs)` + `wg.Wait` + `close(results)`.
- `context/main.go` — same pool, but cancelable via `context.Context`. Demonstrates timeout-driven shutdown mid-flight.

## Run

```bash
go run ./04-worker-pool/basic
go run ./04-worker-pool/context
```

You'll see jobs distributed across workers (output rotates 1→2→3→1→2…), and in the context version, a partial result count when the 350ms deadline cuts execution short.

## Worker pool vs. semaphore (prototype 03)

Both limit concurrency to N. They're not the same thing.

| | Semaphore | Worker pool |
|---|---|---|
| Goroutines | One per job (parked when capacity reached) | Fixed N, reused across jobs |
| Per-worker state | None — every job is its own goroutine | Workers can hold long-lived resources |
| Queue | Implicit (acquirers piled up at the semaphore) | Explicit (jobs channel) |
| When to use | Cheap, short jobs; one-off limiting | Persistent resources; long-running consumer |

A semaphore is the right shape when you have N independent jobs and just want to cap concurrency. A worker pool is the right shape when each "worker" wants to keep something around — a DB connection, an HTTP/2 client with connection pooling, a pre-allocated buffer.

Connection pools (prototype 05) build on the worker-pool mental model.

## The shutdown dance

The hardest part of worker pools isn't the workers — it's **shutting down cleanly**. The canonical pattern is:

```go
go func() {
    wg.Wait()      // wait for all workers to return
    close(results) // safe NOW, because no workers will ever send again
}()

for r := range results { ... }
```

The reasoning, step by step:

1. We must close `results` so `main`'s `range results` loop terminates.
2. We can't close `results` while workers might still send to it — sending to a closed channel **panics**.
3. So we close `results` only after all workers have returned, which we know via `wg.Wait()`.
4. We can't call `wg.Wait()` directly from `main` — `main` is busy ranging over `results`.
5. **Therefore: spawn a goroutine that calls `wg.Wait()` and then `close(results)`.**

Once you've internalized this pattern, you'll see it everywhere in Go codebases.

## Critical rules for `close()`

These bite every Go developer at least once:

- **Only the sender closes** a channel. Never the receiver.
- **Don't close from multiple goroutines** — if multiple senders share a channel, use `sync.Once` or coordinate via a separate signal.
- **Closing a closed channel panics.**
- **Sending to a closed channel panics.**
- **Receiving from a closed channel** returns the zero value immediately and `ok` is false: `v, ok := <-ch`.

## Directional channel types

In function signatures we used:

```go
func worker(jobs <-chan int, results chan<- Result) { ... }
```

- `<-chan int` — **receive-only** view of a channel. Compiler error if you try to send.
- `chan<- Result` — **send-only** view. Compiler error if you try to receive.

These are not different types under the hood — at the call site, you can pass a bidirectional `chan T` and it implicitly converts. They're just a way to make API misuse a compile error rather than a runtime bug. **Always** use directional types in function signatures.

## `context.Context` in 30 seconds

A `Context` carries two things across goroutines:
- **Cancellation** — a `Done()` channel that closes when the context is canceled.
- **Deadlines/timeouts** — same mechanism, just auto-canceled when the time passes.

Idiom:

```go
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
defer cancel()  // ALWAYS defer cancel — releases the timer

select {
case <-ctx.Done():
    return ctx.Err()   // probably context.DeadlineExceeded or context.Canceled
case result := <-someChannel:
    return process(result)
}
```

Rules:
- `ctx` is always the **first** parameter to a function, conventionally named `ctx`.
- You **never store** a Context in a struct — pass it as a parameter.
- Cancellation propagates: canceling a parent cancels all children automatically.
- Every blocking operation in your function should be cancelable via `ctx.Done()` (or a function that takes a `ctx`).

## Things to try

**1. Reduce the buffer size.** Change `jobs := make(chan int, numJobs)` to `make(chan int, 1)` — a near-unbuffered channel. The producer now slows down whenever workers are busy. This is **backpressure**, and it's *usually what you want*: it prevents memory bloat under load.

**2. Make jobs more variable.** Replace `time.Sleep(100 * time.Millisecond)` with `time.Sleep(time.Duration(50+job*10) * time.Millisecond)`. Now jobs take different times. Watch how workers naturally balance load — fast ones grab more jobs. This is **work-stealing for free**: a single shared channel is already a perfect load-balancer.

**3. Error handling.** Change `Result` to carry an `error` field. Have workers randomly return an error for some inputs. Have main count successes and failures. Real worker pools always carry errors through the result type.

**4. Set `numWorkers = 1`.** Pool of one — sequential processing with the worker-pool plumbing. Useful for serializing access to a single resource (e.g., a non-thread-safe library).

**5. Compare with the semaphore approach.** Rewrite `basic/main.go` to use prototype 03's channel-based semaphore instead. You'll spawn one goroutine per job, each acquiring the semaphore. Same outcome, different shape. Notice: no `close()` dance is needed, but you can't easily reuse expensive per-worker state.

## Where this goes next

**Prototype 05 — connection pool** combines worker-pool thinking with what we've learned about mutexes and condition variables. A connection pool is "a stash of expensive resources, lent out to callers who hand them back when done." It's exactly the kind of problem worker pools were designed for — and it's where all this theory pays off in real-world systems.
