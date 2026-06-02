# 05 — Connection pool

A **connection pool** is a bounded stash of pre-created, reusable connections. Clients acquire one, do work, release it. The pool caps how many are in flight at once and avoids paying connection-setup cost on every request.

This is the **capstone** of the early prototypes. Everything we've built so far shows up here:

- **Mutex** — protects the idle-connection slice.
- **Semaphore** — caps total connections in flight.
- **Channels** — implement the semaphore.
- **Worker-pool thinking** — workers (here, "clients") consume from a shared resource.
- **Context** — cancelable, timeout-aware acquire.

If you understand this prototype end-to-end, you understand the concurrency core of every production Go database driver, HTTP client, gRPC channel manager, and message-queue consumer in existence.

## Why pool connections?

Connection setup is **slow**:

| Operation | Typical cost |
|---|---|
| TCP handshake | 1 round-trip (RTT) |
| TLS handshake | 2–3 RTTs |
| Database auth + session setup | 1–3 RTTs + crypto |
| HTTP/2 stream setup over existing connection | ~free |
| Reusing a pooled connection | microseconds |

For a service handling thousands of requests per second, pooling vs. not pooling is the difference between viable and broken. Real databases will refuse new connections under load; real OSes have file-descriptor limits. **A pool is not optional infrastructure.**

## What's in this folder

- `basic/main.go` — smallest possible pool: a buffered channel of pre-created connections. The cleanest demonstration of "a pool is just a channel."
- `dynamic/main.go` — production-shape: lazy creation, idle stash, context-cancellable acquire, graceful close. ~150 lines, but every line earns its place.

## Run

```bash
go run ./05-connection-pool/basic
go run ./05-connection-pool/dynamic
```

In both: 8 clients contend over 3 connections. Watch the timestamps — first three clients get connections immediately, the next three wait until the first three release, and so on.

## The mental model: semaphore + stash

A connection pool is two structures working together:

```
                ┌──────────────────────────────────────────────┐
                │ Pool                                         │
                │                                              │
                │  ┌─────────────────────┐                     │
                │  │ semaphore (channel) │  ← caps concurrency  │
                │  │  [● ● ●            ]│    (capacity = max)  │
                │  └─────────────────────┘                     │
                │                                              │
                │  ┌─────────────────────┐                     │
                │  │ idle stash (slice)  │  ← reusable conns    │
                │  │  [conn-1, conn-2, …]│    (under a mutex)   │
                │  └─────────────────────┘                     │
                └──────────────────────────────────────────────┘
```

The semaphore says **"how many can be in flight."** The stash says **"who's reusable right now."** They're separate concerns:

- A slot in the semaphore is **permission to hold one connection** — but doesn't say *which* connection.
- A connection in the stash is **a reusable resource** — but doesn't say who owns it.

This separation is what lets us have lazy creation: the semaphore caps the total, but the stash starts empty. We create a connection only when:

1. The caller has been granted a semaphore slot (so we're under maxSize), and
2. The stash is empty (no existing connection to reuse).

## The `basic/` version in one paragraph

```go
type Pool struct { conns chan *Conn }

func (p *Pool) Acquire() *Conn { return <-p.conns }
func (p *Pool) Release(c *Conn) { p.conns <- c }
```

That's the entire pool. Three lines. The channel buffer is the pool; the values in it are the available connections; sends and receives are release and acquire. The basic/ version pre-creates the maxSize connections at startup, paying all the setup cost upfront.

This is fine for some workloads — a fixed-size connection pool to a known database is reasonable to pre-warm. But for an HTTP client that might connect to many hosts, or a pool whose maxSize is large, lazy creation is much better.

## What `dynamic/` adds

### 1. Lazy creation

Instead of pre-filling the channel with connections, we pre-fill it with **slot tokens** (`struct{}{}`). Acquire takes a token first, then *either* reuses from the idle stash *or* creates a new connection. Release returns to the stash and gives the token back. New connections are created on the request path, not at startup.

### 2. `Acquire(ctx)` for cancellation

```go
select {
case <-p.slots:
    // got a slot
case <-ctx.Done():
    return nil, ctx.Err()
}
```

`select` lets the caller bail out via context cancellation. This is critical: an HTTP request that's been canceled by the user shouldn't keep waiting for a pool slot — return promptly so the goroutine and its stack can be freed.

### 3. Doing expensive work outside the mutex

Look carefully at this part of Acquire:

```go
p.created++
id := p.created
p.mu.Unlock()
return newConn(id), nil  // <-- 100ms of TLS handshake happens HERE, mutex NOT held
```

We compute `id` under the mutex, but call `newConn` *after* unlocking. Why? Because `newConn` does network I/O and takes 100ms. **Holding a mutex during a network call is a classic anti-pattern** — every other goroutine waiting on that mutex stalls for the duration of the slowest call. The fix is to extract anything safe to compute outside the lock.

Rule: **mutexes protect data, not I/O.** Hold them just long enough to read/modify the data, then release.

### 4. `Close()` for graceful shutdown

After Close, any in-flight Acquire returns ErrPoolClosed. In-use connections aren't interrupted (yanking them mid-query would be worse than letting them finish); they're discarded on Release. Idle connections are dropped.

## Side-by-side: pool vs. semaphore vs. worker pool

| | Semaphore (03) | Worker pool (04) | Connection pool (05) |
|---|---|---|---|
| Concurrency cap | Yes | Yes (= numWorkers) | Yes (= maxSize) |
| Per-holder state | None | Per-worker (long-lived goroutine) | Per-connection (the *resource* is long-lived) |
| Who holds the resource? | Each caller, briefly | Same worker forever | Different callers over time (passed around) |
| Created when? | n/a — just tokens | All upfront | Lazily, as needed |
| Real-world examples | API rate limiters | Background job queues | DB pools, HTTP clients |

The connection pool **separates the worker (caller) from the resource (connection)**. A worker pool has long-lived workers each owning a resource. A connection pool has long-lived resources passed between transient callers. Different shape, similar building blocks.

## Comparison with `database/sql`

Go's `database/sql.DB` is the real-world version of `dynamic/main.go`. The full file (`go-src/database/sql/sql.go`, ~3000 lines) adds:

- **Connection health checks** — discard a connection if it returns a "connection refused"-style error from the driver.
- **Max-idle-time** — close connections that have been idle in the pool for too long (the remote may have dropped them).
- **Max-lifetime** — close connections after a hard cap (force periodic re-connection for load balancing).
- **MaxOpen vs. MaxIdle** — separately tune total open and how many to keep idle.
- **Statistics** — `db.Stats()` returns counts for monitoring.
- **Per-connection prepared statement caches.**
- **Transaction support** — Begin/Commit/Rollback that holds the connection for the whole transaction.

The *core* is what you see in `dynamic/`. Everything else is operational polish layered on top. Worth reading the source if you're curious — search for `func (db *DB) conn` in `database/sql.go`.

## Things to try

**1. Lower the timeout to see cancellation fire.** In `dynamic/main.go`, change the per-client `WithTimeout` from 2 seconds to 150ms. Run again. Some clients will fail with `context deadline exceeded`. The pool gracefully handles it — no leaked goroutines, no leaked slots.

**2. Track creation cost.** In `dynamic/`, count how many calls to `newConn` happen vs. how many `Acquire` calls. With reuse, creations should be ≤ maxSize. Without it, creations would equal numClients. Print both at the end.

**3. Health-check on acquire.** Add a `(c *Conn) Healthy() bool` method that returns false randomly (say, 10% of the time). In Acquire, before returning a reused connection, check Healthy(); if it's broken, discard and create a new one. Now you've implemented the most important production feature: bad-connection eviction.

**4. Add a max-idle-time.** Each `Conn` records its last release time. On Acquire-from-stash, if `time.Since(lastRelease) > maxIdle`, discard and create new. This is how pools cope with remote servers that drop idle connections.

**5. Build a "weighted" pool.** Some operations need 1 slot (a query), some need more (a bulk import that pegs a connection for minutes). Use `golang.org/x/sync/semaphore` instead of a channel — it natively supports `Acquire(ctx, n)` for n>1. Now your pool models real-world resource cost more accurately.

**6. Read the source of `golang.org/x/sync/semaphore`.** It's ~100 lines and uses mutex + condvar (not a channel). A great study in the trade-offs we discussed in prototype 03.

## The principle

> **A connection pool is bounded concurrency to a stash of reusable resources. The semaphore caps concurrency; the stash caps creation cost. Both matter.**

You can build a "limit concurrency" feature with just a semaphore — but you'll pay TCP+TLS+auth on every request, which is unworkable. You can build a "reuse" feature with just a stash — but you'll over-create connections under burst load. The combination is what makes it work.

## Where the series goes next

You've now seen the entire shape of concurrent infrastructure: goroutines, synchronization, bounded concurrency, dynamic resource management. Some good next directions if you want to keep going:

- **Rate limiter** — token bucket. Not just "N at a time" but "N per second." Different math, related shape.
- **Circuit breaker** — state machine for "stop trying when the downstream is failing." Composes well with pools.
- **Distributed pools / leases** — extending pooling across multiple servers (e.g., Redlock, ZooKeeper).
- **Read Go's `net/http` Transport pooling** — how Go reuses HTTP connections under the hood. Same patterns, real code.
