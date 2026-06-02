// Prototype 05 (basic): connection pool, the simplest possible version.
//
// CORE INSIGHT: a connection pool is just a buffered channel of pre-created
// connections. Acquire = receive from the channel; Release = send back.
// That's the entire mechanism. The same channel-as-semaphore trick from
// prototype 03 — only now the "tokens" in the channel are real connections.
//
// Why pool connections at all? Because connection setup is EXPENSIVE:
//   - TCP handshake (~RTT)
//   - TLS handshake (~3 RTTs)
//   - Database auth, session setup, prepared-statement cache warmup, etc.
// A fresh connection can take 100ms+; reusing one takes microseconds. If your
// service handles 10,000 req/s, pooling vs. not pooling is the difference
// between viable and unviable.
//
// Run:    go run ./05-connection-pool/basic
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Conn is our mock "connection." In real life this would be a *sql.Conn,
// an HTTP/2 client, an open TCP socket, a Redis pipeline, etc.
type Conn struct {
	id int
}

func newConn(id int) *Conn {
	// Simulate expensive setup. Real connection setup involves TCP+TLS+auth.
	time.Sleep(100 * time.Millisecond)
	return &Conn{id: id}
}

func (c *Conn) Query(s string) string {
	time.Sleep(50 * time.Millisecond) // pretend to talk to the remote
	return fmt.Sprintf("conn-%d: result for %q", c.id, s)
}

// Pool is a fixed-size collection of *Conn.
// The channel buffer IS the pool — values currently inside are "available."
type Pool struct {
	conns chan *Conn
}

// NewPool pre-creates `size` connections upfront and parks them in the channel.
// All connection setup cost is paid at startup, not on the request path.
func NewPool(size int) *Pool {
	p := &Pool{conns: make(chan *Conn, size)}
	for i := 1; i <= size; i++ {
		p.conns <- newConn(i)
	}
	return p
}

// Acquire takes a connection from the pool. Blocks if none is available.
func (p *Pool) Acquire() *Conn {
	return <-p.conns
}

// Release returns a connection to the pool. Never blocks because we always
// release exactly the connection we acquired — total in flight never exceeds
// the channel's capacity.
func (p *Pool) Release(c *Conn) {
	p.conns <- c
}

func main() {
	const (
		poolSize    = 3
		numClients  = 8
	)

	fmt.Println("creating pool (pre-warming connections)...")
	pool := NewPool(poolSize)
	fmt.Printf("pool ready with %d connections\n\n", poolSize)

	var (
		inUse       atomic.Int32
		maxObserved atomic.Int32
		wg          sync.WaitGroup
	)

	start := time.Now()

	for i := 1; i <= numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			fmt.Printf("[%4dms] client %d: waiting for connection...\n",
				ms(start), id)

			c := pool.Acquire()
			defer pool.Release(c)

			now := inUse.Add(1)
			updateMax(&maxObserved, now)

			fmt.Printf("[%4dms] client %d: got conn-%d  (in-use=%d)\n",
				ms(start), id, c.id, now)

			resp := c.Query(fmt.Sprintf("SELECT %d", id))
			fmt.Printf("[%4dms] client %d: %s\n", ms(start), id, resp)

			inUse.Add(-1)
		}(i)
	}

	wg.Wait()
	fmt.Printf("\ntotal time: %dms\n", ms(start))
	fmt.Printf("max connections in use simultaneously: %d (pool size: %d)\n",
		maxObserved.Load(), poolSize)
}

func ms(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

// CAS loop to maintain a high-water mark, same idea as prototype 03.
func updateMax(m *atomic.Int32, v int32) {
	for {
		cur := m.Load()
		if v <= cur || m.CompareAndSwap(cur, v) {
			return
		}
	}
}
