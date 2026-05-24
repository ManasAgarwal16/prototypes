# 01 — Goroutines

A goroutine is a function running concurrently with other goroutines in the same address space. It's like a thread, but cheap — you can have hundreds of thousands of them.

## Key ideas

- `go f()` starts `f` in a new goroutine and returns immediately.
- The `main` goroutine doesn't wait for other goroutines unless you tell it to. If `main` returns, the program exits — even if goroutines are still running.
- `sync.WaitGroup` is the standard "wait for N things to finish" primitive.
  - `Add(n)` — say you're about to launch n things.
  - `Done()` — one thing finished (usually `defer wg.Done()` at the top of the goroutine).
  - `Wait()` — block until the counter hits zero.

## Run it

```bash
go run ./01-goroutines
```

You should see workers start in order but **finish** in order too (because their sleep duration grows with id). Try shuffling or randomizing the sleeps — the output order will become unpredictable, which is the whole point of concurrency.

## Things to try

1. Remove `wg.Wait()` — what happens? (Hint: `main` exits before workers finish.)
2. Move `wg.Add(1)` *inside* the goroutine instead of before `go worker(...)`. What goes wrong? (Hint: race between Add and Wait.)
3. Launch 100,000 goroutines. They cost ~2KB each, so this is fine — try the same with OS threads in another language and you'll crash.
