# Concurrency prototypes in Go

Small, self-contained prototypes for learning concurrency and systems concepts hands-on. Each folder is a runnable mini-program with a README explaining the *why*.

## Learning path

| # | Prototype | Concept |
|---|---|---|
| 01 | [goroutines](./01-goroutines) | Spawn concurrent work, wait for it with `sync.WaitGroup` |
| 02 | [mutex](./02-mutex) | Data races and how `sync.Mutex` fixes them |
| 03 | semaphore | _coming next_ — limit concurrency to N |
| 04 | worker-pool | _planned_ — N workers consume from a job channel |
| 05 | connection-pool | _planned_ — acquire/release expensive resources with timeouts |
| 06 | rate-limiter | _planned_ — token bucket |
| 07 | circuit-breaker | _planned_ — state machine + concurrency |

## How to run any prototype

From the repo root:

```bash
go run ./01-goroutines
go run -race ./02-mutex/race    # always try with -race while learning
```

## Conventions

- Each prototype lives in its own folder, `NN-name/`.
- If a prototype has multiple variants (e.g. broken vs. fixed), they live in subfolders.
- Each folder has a `README.md` with: the concept, what to run, and "things to try."
- Comments in code lean explanatory — this is a learning repo, not production.

## Requirements

- Go 1.21+ (tested on 1.26)
