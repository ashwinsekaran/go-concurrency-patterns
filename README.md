# Go Concurrency Patterns

Four self-contained examples covering the core concurrency primitives in Go.

---

## The Building Blocks (before any pattern)

### Goroutine

```go
go func() { ... }()
```

A goroutine is Go's version of a lightweight thread. When you write `go`, you're telling the Go runtime: "run this function concurrently, don't wait for it." The current function continues immediately. The goroutine runs independently somewhere in the background. You can have thousands of them — they're cheap (a few KB each vs. MB for OS threads).

**The problem goroutines create:** once you fire them off, you've lost control. You don't know when they finish, what they produced, or if they panicked. That's why you need channels and WaitGroups.

---

### `sync.WaitGroup`

```go
wg := sync.WaitGroup{}
wg.Add(1)   // "I'm about to launch 1 goroutine"
wg.Done()   // called by the goroutine when it finishes
wg.Wait()   // blocks here until the count reaches 0
```

A WaitGroup is just a counter. You increment it before launching a goroutine, decrement it inside the goroutine when done, and block with `Wait()` until it hits zero. It answers the question: **"how do I know when all my goroutines are finished?"**

`defer wg.Done()` is placed at the top of the goroutine so it fires no matter what — even if the goroutine panics or returns early.

---

### Channels

```go
ch := make(chan int)       // unbuffered
ch := make(chan int, 10)   // buffered, capacity 10
```

A channel is a typed pipe between goroutines. One goroutine sends (`ch <- value`), another receives (`value := <-ch`).

**Unbuffered channel:** the sender blocks until someone is ready to receive. The receiver blocks until someone sends. They meet in the middle — it's a synchronization point, like a handshake.

**Buffered channel:** has a queue inside. The sender can push up to `capacity` values without blocking. The receiver can pull whenever it wants. Only blocks when the buffer is full (sender) or empty (receiver).

**Closing a channel:** `close(ch)` signals "no more values will be sent." Receivers can still drain what's left. `for range ch` exits cleanly when the channel is both empty and closed. Sending to a closed channel panics.

---

### The Closure Capture Bug

This is a classic Go gotcha. Consider this broken code:

```go
for i := 1; i <= 5; i++ {
    go func() {
        println(i)  // captures the VARIABLE i, not its value
    }()
}
// All goroutines likely print 6 (the final value of i)
```

All goroutines share the same `i` variable from the loop. By the time they actually run, the loop has already finished and `i == 6`.

The fix — pass `i` as a parameter:

```go
for i := 1; i <= 5; i++ {
    go func(i int) {  // i is now a local copy for this goroutine
        println(i)
    }(i)              // value is passed here at launch time
}
```

Each goroutine gets its own copy of `i` at the moment it was launched.

---

---

## Pattern 1 — Fan-out Channel (`fanout-channel/`)

```
goroutine 1 ──┐
goroutine 2 ──┤
goroutine 3 ──┼──► ch ──► main reads
goroutine 4 ──┤
goroutine 5 ──┘
```

```go
wg := sync.WaitGroup{}
ch := make(chan int, 1)

for i := 1; i <= 5; i++ {
    wg.Add(1)
    go func(i int) {
        defer wg.Done()
        println("worker", i)
        ch <- i           // each goroutine sends its result
    }(i)
}

go func() {
    wg.Wait()   // wait for ALL workers to finish
    close(ch)   // then close the channel
}()

for i := range ch {   // drains until channel is closed
    println("received", i)
}
```

**Step by step:**
1. You launch 5 goroutines. Each does its work and sends one value into `ch`.
2. A separate watcher goroutine calls `wg.Wait()` — it blocks until all 5 workers call `wg.Done()`.
3. Once all workers are done, the watcher closes `ch`.
4. `for i := range ch` in main drains everything from the channel and exits when the channel is closed and empty.

**Why the separate goroutine for `close`?** You can't call `close(ch)` in main right after the loop — main doesn't know when the goroutines finish. You can't call it inside the workers — multiple goroutines closing the same channel panics. The watcher goroutine is the clean solution: one entity, one close.

**Real-world use cases:**
- Firing off N parallel HTTP requests and collecting all responses
- Running N file reads concurrently and aggregating the results
- Parallel database queries, all results fed into one stream

---

## Pattern 2 — Worker Pool Basic (`worker-pool-basic/`)

```
main ──► jobs channel ──► worker 1
                      ──► worker 2
                      ──► worker 3
```

```go
wg := sync.WaitGroup{}
jobs := make(chan int, 10)

// Launch 3 workers FIRST
for i := 1; i <= 3; i++ {
    wg.Add(1)
    go func(workerID int) {
        defer wg.Done()
        for job := range jobs {   // each worker loops, consuming jobs
            println("worker", workerID, "received job", job)
        }
        // range exits when jobs is closed AND empty
    }(i)
}

// Then send 10 jobs
for i := 1; i <= 10; i++ {
    jobs <- i
}
close(jobs)   // signal: no more work coming

wg.Wait()     // wait for all workers to finish draining
```

**Step by step:**
1. 3 worker goroutines start. Each immediately tries to read from `jobs` — but it's empty, so they all block waiting.
2. Main sends 10 jobs into the channel. The buffer holds 10, so this doesn't block.
3. Workers wake up and start consuming. Which worker gets which job is non-deterministic — Go's scheduler decides.
4. `close(jobs)` signals end of work. Workers' `for range` loops exit once the channel is both empty and closed.
5. `wg.Wait()` ensures main doesn't exit until all workers finish their last job.

**The key insight — workers vs jobs ratio:** You have 3 workers and 10 jobs. Workers are reused — a worker that finishes job 1 immediately picks up job 4. You never have more than 3 goroutines running simultaneously, even with 10 jobs. This is the whole point: **you control concurrency**.

**Fan-out vs Worker Pool — what's the difference?**

| | Fan-out | Worker Pool |
|---|---|---|
| Goroutines | Created per task | Created once, reused |
| Tasks | Each goroutine does one thing | Each goroutine does many things |
| Good for | Small number of parallel tasks | Large number of tasks, controlled concurrency |
| Goroutine count | Equals task count | Fixed, independent of task count |

**Real-world use cases:**
- Rate-limited API calls (e.g., 5 workers = max 5 concurrent requests)
- Image/video processing pipeline (3 workers = 3 CPU cores fully used)
- Database connection pool (workers = available connections)
- Batch processing jobs from a queue

---

## Pattern 3 — Worker Pool Traced (`worker-pool-traced/`)

This is the **same pattern as Pattern 2**, but with `fmt.Println` at every step. The purpose is to let you *see* the concurrency happening.

**What the output reveals:**

```
starting workers
starting worker 1
starting worker 2
starting worker 3
sending jobs
sending job 1
sending job 2
...
sending job 10          ← all 10 fit in the buffer, main never blocks
closing jobs channel
worker goroutine started 3  ← goroutines JUST NOW started (any order)
worker goroutine started 1
worker goroutine started 2
worker 1 received job 1
...
done
```

You'll often see all 10 `sending job` lines print *before* any `worker goroutine started` line. This is because:

1. The buffer has capacity 10 — main can fill it completely without anyone needing to read.
2. Go's scheduler doesn't preempt main to run goroutines at arbitrary points; it yields at blocking points. Since main never blocks (buffer never fills), goroutines don't get CPU until main hits `wg.Wait()`.

This teaches you that **goroutine scheduling is not instantaneous** — `go func()` schedules the goroutine, it doesn't run it immediately.

**Real-world use:** Add this level of tracing in development when trying to understand a race condition or unexpected ordering. Strip it out in production.

---

## Pattern 4 — Select Multichannel (`select-multichannel/`)

```go
ch1 := make(chan string)  // unbuffered
ch2 := make(chan string)  // unbuffered

// Worker 1: sleeps 1s, sends to ch1
// Worker 2: sleeps 2s, sends to ch2

select {
case msg := <-ch1:
    println("ch1:", msg)
case msg := <-ch2:
    println("ch2:", msg)
}
```

**What `select` does:** it waits on multiple channel operations simultaneously and executes the first one that becomes ready. It's Go's way of saying "I don't care which one fires first, just give me whichever is ready."

**Step by step:**
1. Both goroutines start simultaneously.
2. First `select` hits: both channels are empty, so main blocks waiting.
3. After 1 second, worker 1 sends `"hello"` to `ch1`. The select wakes up, `case msg := <-ch1` fires, prints `ch1: hello`.
4. Second `select` hits: `ch1` is now empty, `ch2` still waiting. Main blocks again.
5. After another second (2s total), worker 2 sends `"world"` to `ch2`. Second select fires, prints `ch2: world`.

**If both channels were ready at the same time:** Go picks one at random. This is intentional — no channel gets permanent priority.

**The `default` case (non-blocking select):**
```go
select {
case msg := <-ch1:
    println(msg)
default:
    println("nothing ready, moving on")
}
```
With `default`, select becomes non-blocking — if nothing is ready it falls through immediately.

**Real-world use cases:**

```go
// Timeout pattern — the most common use of select
select {
case result := <-workCh:
    // got a result in time
case <-time.After(5 * time.Second):
    // took too long, give up
}
```

```go
// Cancellation pattern
select {
case job := <-jobs:
    process(job)
case <-ctx.Done():
    // context cancelled (e.g. HTTP request disconnected)
    return
}
```

```go
// Non-blocking send
select {
case ch <- value:
    // sent successfully
default:
    // channel full or no receiver, skip
}
```

---

## Summary

```
Primitive          Purpose
─────────────────────────────────────────────────────
goroutine          run code concurrently
WaitGroup          know when goroutines finish
channel (unbuf)    synchronize two goroutines
channel (buffered) decouple sender/receiver speed
close(ch)          signal "no more values"
for range ch       drain until closed
select             react to first-ready channel

Pattern            When to reach for it
─────────────────────────────────────────────────────
fan-out            N tasks, collect all results
worker pool        N tasks, limit concurrency to K
select             timeouts, cancellation, racing
```

> The mental model: **goroutines are the workers, channels are the conveyor belts between them, WaitGroup is the foreman counting who's still on the floor, and select is the traffic light that reacts to whichever road opens first.**
