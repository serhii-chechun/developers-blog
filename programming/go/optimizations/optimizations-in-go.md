# Optimizations in Go: A Multi-Layer Framework

Every Go engineer has a story about the day they "optimized" the wrong thing.

Maybe it was rewriting `fmt.Sprintf` calls into `strconv.AppendInt` to save microseconds in a
function that ran 2% of the time - while a single unindexed query quietly added 400ms to every
request. Or dropping `sync.Pool` into a hot path to eliminate allocations, only to find the
real problem was an `O(N^2)` loop that should have been `O(N log N)`.

These stories all share the same shape. Optimization is not a bag of tricks; it's a *search*
problem, and the search has a topology. Some moves are worth weeks of engineering. Others are worth
an afternoon. The trick is knowing which is which *before* spending the time.

This post lays out a five-layer framework for that search, from measurement at the top down to
runtime and OS tuning at the bottom. Every layer has runnable code, and every claim is
reproducible on real hardware.

All the code from this post is available as runnable examples in the
[accompanying repository](https://github.com/serhii-chechun/developers-blog/tree/main/programming/go/optimizations/examples)
- and every snippet in this post has been compiled, tested, and run against Go 1.27.

> **The Golden Rule:** Don't optimize early, and don't optimize blind. Measure first, change one
> thing, measure again. Everything below is a *hypothesis* until the data confirms it.

---

## Table of Contents

1. [Layer 1: Measurement & Diagnostics](#1-layer-1-measurement--diagnostics)
2. [Layer 2: Algorithmic & Architectural Optimizations](#2-layer-2-algorithmic--architectural-optimizations)
3. [Layer 3: Memory & Garbage Collector Management](#3-layer-3-memory--garbage-collector-management)
4. [Layer 4: Micro-Optimizations & Language Idioms](#4-layer-4-micro-optimizations--language-idioms)
5. [Layer 5: Runtime & OS Level](#5-layer-5-runtime--os-level)
6. [The Checklist](#the-checklist)

---

## Why Layers?

Think of optimization as a pyramid - or, if preferred, a funnel. Each layer down costs more
engineering effort and yields less reward than the one above it.

| Layer | Typical win | Effort | Risk of wasted work |
| --- | --- | --- | --- |
| 1. Measurement | Find out what actually matters | Low | None - it's a prerequisite |
| 2. Algorithmic / architectural | **10x-1000x** (order of magnitude) | High | Low - gains build on each other |
| 3. Memory & GC | 2x-10x | Medium | Medium |
| 4. Micro-optimizations | 1x-2x | Medium | **High** - easy to optimize cold code |
| 5. Runtime & OS | 1x-1.5x | Low-Medium | Medium |

The insight is asymmetric: a bad algorithm costs *weeks* no matter how fast string concatenation
is, while a good algorithm performs fine even with unoptimized low-level code. An engineer who jumps
straight to Layer 4 is often optimizing code that was never the bottleneck.

```text
+-------------------------------------------------------------+
|  1. MEASURE  -->   pprof / trace / benchstat                |
+------------------------------+------------------------------+
                               |
                               v
+-------------------------------------------------------------+
|  Where is the time going?                                   |
+------+------------------------------------------------------+
       |
       +--> algorithm / architecture ... 2. Algorithmic O(N^2) -> O(N log N)
       |
       +--> allocations / GC ........... 3. Memory & GC fewer allocs/GC
       |
       +--> CPU instructions ........... 4. Micro-optimizations
       |
       +--> scheduler / syscalls ....... 5. Runtime & OS GOMAXPROCS, PGO
       |
       v
   RE-MEASURE  -->  back to "Where is the time going?"
```

With that framing in place, let's start where every optimization must start: with data.

---

## Layer 1: Measurement & Diagnostics

Before touching a single line of code, it's essential to know where the time and memory actually
go. Intuition is a poor profiler. The Go toolchain ships with everything needed, and it's all
first-class - no third-party instrumentation required.

This is the prerequisite layer. Skipping it is the single most expensive mistake one can make.

### 1.1 Profiling with `pprof`

`pprof` gives a statistical view of where a program spends its time. It samples - it doesn't
instrument every call - so the overhead is low enough to run in production (with care).

The two profiles used most often:

- **CPU profile** - which functions are on-CPU. This is the hot-path map.
- **Heap profile** - where memory is allocated. The distinction between `alloc_objects`/`alloc_space`
  (everything ever allocated) and `inuse_objects`/`inuse_space` (still alive) is *critical*. A
  function can dominate total allocations yet contribute nothing to steady-state memory.

Here's a complete, runnable program that writes a CPU profile to disk:

```go
// main.go
package main

import (
	"log"
	"os"
	"runtime/pprof"
)

// fib is deliberately naive: exponential time, the classic hot path.
func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	total := 0
	for i := range 37 {
		total += fib(i)
	}
	log.Printf("total = %d", total)
}
```

Run it, then open the interactive UI:

```bash
go run main.go
go tool pprof -http=:8080 cpu.prof
```

Now swap the profiler for the heap. The pattern is nearly identical:

```go
// heap.go - write a heap profile after the work is done.
package main

import (
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

type Row struct {
	Key   string
	Value []byte
}

// rows lives at package level so it stays live across the GC below.
// The GC only keeps data that is still reachable; a local variable becomes
// unreachable after its final use, and would be reclaimed before the profile.
var rows []Row

// buildRows allocates a slice of Row values, each with a fresh byte slice.
func buildRows(n int) []Row {
	out := make([]Row, 0, n)
	for range n {
		out = append(out, Row{
			Key:   "row-key",
			Value: make([]byte, 1024),
		})
	}
	return out
}

func main() {
	rows = buildRows(100_000)

	// Force a GC so the profile reflects live objects, not garbage.
	runtime.GC()

	f, err := os.Create("heap.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Fatal(err)
	}
}
```

```bash
go run heap.go
go tool pprof -http=:8080 heap.prof
```

> **Gotcha:** A heap profile shows what the program is still *holding*, not what it once allocated. If
> nothing references the data, it is garbage - and `runtime.GC()` collects it before the profile is
> written. With `rows` as an unused local, the profile totals a couple of megabytes of runtime
> bookkeeping and never mentions `buildRows` at all:

```
Showing nodes accounting for 1.50MB, 100% of 1.50MB total
      flat  flat%   sum%        cum   cum%
    1.50MB   100%   100%     1.50MB   100%  runtime.mallocgc
```

Keeping the data reachable - the package-level sink above, or `runtime.KeepAlive(rows)` after the
profile is written - produces the profile that was actually wanted:

```
Showing nodes accounting for 111.93MB, 100% of 111.93MB total
  110.43MB 98.66% 98.66%   110.43MB 98.66%  main.buildRows (inline)
```

Switching the sample to `alloc_space` tells a different and equally true story: those allocations
still appear, because the profile records allocation *sites* whether or not the memory survived.
`inuse_space` answers "what is the service holding?"; `alloc_space` answers "where is the service
allocating?" - ask whichever matches the question.

> **Gotcha:** *Where* the profiler starts changes what it sees. Profiles started inside `main`
> will miss work done during package `init` or before the profiler arms itself. For servers, expose
> a `/debug/pprof/` endpoint with `net/http/pprof` and capture profiles from the live process -
> that's the workload that actually matters.

For servers, one import is all it takes to get the full suite:

```go
import _ "net/http/pprof"

// elsewhere:
// go func() { log.Println(http.ListenAndServe("localhost:6060", nil)) }()
```

That single blank import mounts `/debug/pprof/`, `/debug/pprof/profile`, `/debug/pprof/heap`,
`/debug/pprof/mutex`, `/debug/pprof/block`, `/debug/pprof/goroutine`, and more.

The three "extra" profiles are worth naming explicitly:

| Profile | Answers the question | Enable with |
| --- | --- | --- |
| `mutex` | Which locks are contended, and how long do goroutines wait? | `runtime.SetMutexProfileFraction(n)` |
| `block` | Which operations block goroutines (channels, `select`, syscalls)? | `runtime.SetBlockProfileRate(n)` |
| `goroutine` | How many goroutines exist, and where are they stuck? | Always on (`/debug/pprof/goroutine?debug=2`) |

The mutex and block profilers are *off* by default because they cost CPU. Turn them on
deliberately, sample, and turn them back off.

### 1.2 The Execution Tracer (`go tool trace`)

Where `pprof` answers "which function is hot?", the execution tracer answers a different and often
more revealing question: *"what were my goroutines doing or waiting for over time?"*

The tracer captures goroutine scheduling, GC pauses, syscalls, network I/O, and synchronization
events with microsecond resolution. It reveals a class of problem that CPU profiles actively hide:
a program that uses very little CPU because it's **blocked**.

A small caveat: the `block` and `mutex` profiles from 1.1 do report blocked time - as aggregate
totals and locations. What they cannot show is the *timeline*: which block came first, how blocks
overlapped, and what each goroutine did between waits. That sequence is the tracer's specialty.

```go
// trace.go
package main

import (
	"fmt"
	"os"
	"runtime/trace"
	"sync"
)

// worker does a bounded amount of work, then hands off through a channel.
func worker(jobs <-chan int, done chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		// Simulate work with a small in-CPU computation.
		sum := 0
		for i := range 1_000_000 {
			sum += i ^ j
		}
		done <- sum
	}
}

func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := trace.Start(f); err != nil {
		panic(err)
	}
	defer trace.Stop()

	jobs := make(chan int, 100)
	done := make(chan int, 100)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go worker(jobs, done, &wg)
	}

	go func() {
		for i := range 100 {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	count := 0
	for range done {
		count++
	}
	fmt.Println("completed jobs:", count)
}
```

```bash
go run trace.go
go tool trace trace.out
```

The browser UI it opens allows inspection of the goroutine timeline, the scheduler's view of each
P, and - most valuably - a latency graph of GC pauses and a "user-defined tasks" view when the code
is annotated with `trace.WithRegion` and `trace.Log`.

> **When to reach for the tracer:** If CPU usage is low but latency is high, `pprof` will show
> almost nothing. The tracer will show a river of goroutines waiting on a channel, a lock, or a
> syscall. That's the tracer's home turf.

### 1.3 Benchmarking with `testing.B` and `benchstat`

Profiling shows *where* time goes in one run. Benchmarks show whether a change *actually helped* -
and `benchstat` shows whether the difference is real or just noise.

A benchmark is a function in a `_test.go` file that takes a `*testing.B`:

```go
// concat_test.go
package concat

import "testing"

// ConcatPlus repeatedly appends to a string, allocating on every step.
func ConcatPlus(n int) string {
	s := ""
	for range n {
		s += "x"
	}
	return s
}

func BenchmarkConcatPlus(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = ConcatPlus(1000)
	}
}
```

`b.Loop()` reports whether the benchmark should run another iteration; the framework chooses that
count, so the loop body never needs to know it. Since Go 1.24 this is the preferred form, for three
reasons. The timer starts on the first call to `b.Loop()` and stops when it returns false, so setup
before the loop and cleanup after it are excluded from the measurement automatically. Values used
inside the body are kept alive, so the compiler cannot optimize the work away. And the framework
ramps up by running the benchmark function once rather than repeatedly with different `b.N`. After
the loop, `b.N` holds the iteration count for any derived metrics.

Always run benchmarks with `-benchmem` (or `b.ReportAllocs()`, as above) so allocation counts and
bytes-per-op appear in the output:

```bash
go test -run=^$ -bench=. -benchmem -count=10 ./... > old.txt
# ...make the change...
go test -run=^$ -bench=. -benchmem -count=10 ./... > new.txt
benchstat old.txt new.txt
```

> **One-time setup:** `benchstat` is not part of the standard toolchain. It ships in
> `golang.org/x/perf` and installs with `go install golang.org/x/perf/cmd/benchstat@latest`,
> landing in `$(go env GOPATH)/bin` - which needs to be on `PATH`.

Three details decide whether a benchmark is trustworthy:

- `-run=^$` skips all unit tests, so only benchmarks run.
- `-count=10` runs each benchmark ten times. `benchstat` needs *repeated* measurements to compute
  confidence intervals. A single run is a coin flip.
- `benchstat` then reports the median, the delta, and a p-value, so the result can be stated as
  "this is 12% faster with 95% confidence" rather than "it felt faster."

A `benchstat` comparison looks like this (numbers from the string-building example in Layer 4.1):

```
goos: darwin
goarch: arm64
              │   old.txt    │             new.txt              │
              │    sec/op    │    sec/op     vs base            │
Concat-8         63.17µ ± 3%    2.130µ ± 1%  -96.63% (p=0.000 n=10)
Concat-8         517.8Ki ± 0%    1.000Ki ± 0%  -99.81% (p=0.000 n=10)
```

> **Gotcha:** A benchmark silently becomes a benchmark *of the optimizer* if the compiler can prove
> the result is unused - an inlined function whose return value is discarded can appear to "run" in
> under a nanosecond, because it isn't running at all. `b.Loop()` closes off that whole class of
> mistake by keeping the loop body's values alive. With an `b.N`-style loop the burden falls back on
> the author: assign the result to a package-level sink, or pass it to a helper that consumes it.
> Two rules still apply to `b.Loop()`: write the condition exactly as `b.Loop()`, and never mix a
> `b.N`-style loop into the same benchmark function.

With measurement understood, we can finally talk about changing code - and the biggest wins live one
layer below.

---

## Layer 2: Algorithmic & Architectural Optimizations

This is where order-of-magnitude wins live. No amount of `sync.Pool` can save an `O(N^2)` algorithm,
because the problem is the growth rate, not the constant factor.

The techniques here are not Go-specific; they're computer science. But Go's execution model makes
some of them more rewarding - and easier to get wrong.

### 2.1 Data Structure Selection

A map is the default choice for looking up a key. Maps are excellent - but for *small* collections,
they can be slower than a linear scan over a slice. A map lookup hashes the key, probes a bucket,
and touches memory that's scattered across the heap. A linear scan over a small slice reads
contiguous cache lines.

The point where the map wins depends on the key type and hardware, but it usually falls somewhere
between a few dozen and a few hundred elements:

```go
// lookup_test.go
package lookup

import "testing"

type entry struct {
	key string
	val int
}

const size = 16

var sliceEntries = func() []entry {
	es := make([]entry, 0, size)
	for i := range size {
		es = append(es, entry{key: string(rune('a' + i)), val: i})
	}
	return es
}()

var mapEntries = func() map[string]int {
	m := make(map[string]int, size)
	for _, e := range sliceEntries {
		m[e.key] = e.val
	}
	return m
}()

// lookupSlice scans linearly - contiguous memory, no hashing.
func lookupSlice(key string) int {
	for i := range sliceEntries {
		if sliceEntries[i].key == key {
			return sliceEntries[i].val
		}
	}
	return -1
}

// lookupMap hashes the key and probes a bucket.
func lookupMap(key string) int {
	return mapEntries[key]
}

func BenchmarkLookupSlice(b *testing.B) {
	for b.Loop() {
		_ = lookupSlice("k")
	}
}

func BenchmarkLookupMap(b *testing.B) {
	for b.Loop() {
		_ = lookupMap("k")
	}
}
```

At 16 elements, the slice scan usually wins: no hashing, no allocations, and contiguous memory. But
**the cutoff is not fixed**. Change the key to a small integer, or raise `size` to 64, and the map
can win.

| N (string keys) | Slice scan | Map |
| --- | --- | --- |
| 8 | often faster | map overhead dominates |
| 16 | frequently faster | competitive |
| 64 | roughly even | typically faster |
| 1000+ | **much slower** | **clearly faster** |

On the machine used for the examples in this post (darwin/arm64, Go 1.27), a 16-element lookup
measured **6.7 ns/op** for the slice scan versus **7.1 ns/op** for the map - a small but repeatable
win.

> **Rule of thumb:** For `N < ~64`, prefer a slice. For everything else, a map. But *measure the
> crossover for the key type and hardware in use* - it is not a constant, and integer keys shift it
> dramatically.

#### Sets and Membership Tests

A very common variant is not a lookup at all but a *membership test*: "is this key in the set?" The
usual shape is a map used purely as a set.

```go
var safeMethods = map[string]struct{}{
	"GET":     {},
	"HEAD":    {},
	"OPTIONS": {},
}

if _, ok := safeMethods[method]; ok {
	// safe to retry
}
```

Note the value type. `map[string]struct{}` gives two states only: absent, or present. The tempting
alternative, `map[string]bool`, allows `if m[key] { ... }` - but it cannot tell "absent" from
"present and false", so it is correct only while `false` is never stored. That is an invariant
nothing enforces: someone later writes `m["feature"] = enabled`, and the meaning of every existing
check changes silently.

Measured for the same 16-key set (`-count=5`, medians):

| Membership test | Hit | Miss |
| --- | --- | --- |
| `map[string]bool` + `if m[key]` | 7.2 ns | 6.1 ns |
| `map[string]struct{}` + `_, ok :=` | 6.4 ns | 5.5 ns |
| `slices.Contains(keys, key)` | 6.4 ns | 10.5 ns |
| `switch` with grouped cases | **2.2 ns** | **2.2 ns** |

Three things fall out:

- **The safe map is also the faster map.** `map[string]struct{}` beats `map[string]bool` on both a
  hit and a miss, because there is no value to load.
- **`slices.Contains` matches the map on a hit, but loses badly on a miss**, because a miss scans the
  whole slice. At 16 elements it is a reasonable choice - as long as misses are rare.
- **A `switch` wins by ~3x**, and for a set it reads well, because cases can group values:
  `case "GET", "HEAD", "OPTIONS":`.

> **Gotcha:** The `switch` option exists only while the keys are compile-time constants. The moment
> the set comes from config, a database, or user input, it is unavailable and the choice is back to
> slice-versus-map. That is why this stays a data-structure decision first, and a code-shape
> decision second.

### 2.2 Batching Operations

The most expensive part of many operations is not the work itself - it's the round trip. A database
query, an HTTP call, a disk write: each has fixed overhead (network latency, syscall entry,
lock acquisition) that dwarfs the payload for small items.

The fix is to pay that overhead once for many items instead of once per item.

```go
// batch.go - illustrative: replace the fake store with a real one.
package main

import (
	"context"
	"fmt"
	"time"
)

type Item struct {
	ID    int
	Value string
}

// store mimics a database with a fixed per-call latency.
type store struct{}

func (store) Exec(_ context.Context, _ string, _ ...any) error {
	time.Sleep(500 * time.Microsecond) // network + query overhead
	return nil
}

// InsertOne does one round trip per item - the N+1 pattern.
func InsertOne(ctx context.Context, s store, items []Item) error {
	for _, it := range items {
		if err := s.Exec(ctx, "INSERT INTO items VALUES (?, ?)", it.ID, it.Value); err != nil {
			return err
		}
	}
	return nil
}

// InsertBatch does one round trip for all items.
func InsertBatch(ctx context.Context, s store, items []Item) error {
	args := make([]any, 0, len(items)*2)
	for _, it := range items {
		args = append(args, it.ID, it.Value)
	}
	return s.Exec(ctx, "INSERT INTO items (id, value) VALUES "+placeholders(len(items)), args...)
}

// placeholders builds "(?, ?), (?, ?), ..." for the batch INSERT.
func placeholders(n int) string {
	var out strings.Builder
	for i := range n {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString("(?, ?)")
	}
	return out.String()
}

func main() {
	ctx := context.Background()
	items := make([]Item, 100)
	for i := range items {
		items[i] = Item{ID: i, Value: fmt.Sprintf("v%d", i)}
	}

	start := time.Now()
	_ = InsertOne(ctx, store{}, items)
	fmt.Printf("one-by-one: %v\n", time.Since(start))

	start = time.Now()
	_ = InsertBatch(ctx, store{}, items)
	fmt.Printf("batched:    %v\n", time.Since(start))
}
```

With 100 items and 0.5ms of per-call overhead, the difference is roughly 50ms versus 0.5ms - a 100x
reduction, achieved by making *one* call instead of 100. No micro-optimization comes close.

Batching applies everywhere the same pattern appears:

| Pattern | Naive | Batched |
| --- | --- | --- |
| Database writes | `INSERT` per row | Multi-row `INSERT`, `COPY`, or transactions |
| HTTP | One request per item | One request with a list payload |
| Disk | `Write` per record | Buffered writer, flushed once |
| Channels | Send per item | Send a slice per item |
| Logging | `fmt.Println` per line | Buffered writer |

> **Gotcha:** Batching trades latency for throughput. A batch of 1000 is useless if the caller
> needs the first result *now*. The right design is usually a bounded batch with a flush timer - a
> `bufio.Writer` with a size limit, or a queue that flushes on "N items **or** T milliseconds."

### 2.3 Concurrency vs. Parallelism

Go makes spawning goroutines trivially easy, which is precisely why it's easy to over-spawn.
Goroutines are cheap - but not free. Each one costs a stack, a scheduling slot, and contention on
the runtime's run queues and any channels involved.

The classic mistake is **one goroutine per item** for a large collection of small items. The
scheduling overhead swamps the work.

```go
// pool_test.go
package pool

import (
	"sync"
	"testing"
)

const (
	items       = 100_000
	workPerItem = 100
)

// work is a small, CPU-bound unit - too small to justify a goroutine.
func work(n int) int {
	s := 0
	for i := range workPerItem {
		s += i ^ n
	}
	return s
}

// GoroutinePerItem spawns one goroutine per item.
func GoroutinePerItem() int {
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0

	for i := range items {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r := work(n)
			mu.Lock()
			total += r
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return total
}

// FixedPool runs the same work across a bounded number of workers.
func FixedPool(workers int) int {
	ch := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0

	for range workers {
		wg.Go(func() {
			local := 0
			for n := range ch {
				local += work(n)
			}
			mu.Lock()
			total += local
			mu.Unlock()
		})
	}

	for i := 0; i < items; i++ {
		ch <- i
	}
	close(ch)
	wg.Wait()
	return total
}

func BenchmarkGoroutinePerItem(b *testing.B) {
	for b.Loop() {
		_ = GoroutinePerItem()
	}
}

func BenchmarkFixedPool(b *testing.B) {
	for b.Loop() {
		_ = FixedPool(8)
	}
}
```

The bounded pool wins by a wide margin here - not because it does less work, but because it has far
less overhead. It also aggregates `total` once per worker (using a local accumulator) instead of once
per item, which removes the mutex from the hot path entirely.

There's a second lesson hiding in that code: the worker accumulates into a *local* variable and
takes the lock once at the end. That's the same "pay the overhead once" idea from batching, applied
to synchronization.

The judgment call is this:

```text
  How much work per goroutine?
       |
       +--> microseconds (tiny) ..... don't spawn: batch into a worker pool
       |
       +--> milliseconds+ ........... concurrency is worth it
                  |
                  v
              work CPU-bound?
                  |
                  +--> yes ...................... cap workers at GOMAXPROCS
                  |
                  +--> no (I/O-bound) ........... more workers is fine;
                                                  the bottleneck is waiting
```

> **The rule:** A goroutine must do enough work to repay its own scheduling cost. For CPU-bound
> work, that usually means a worker count near `runtime.GOMAXPROCS(0)`. For I/O-bound work, waiting
> dominates, so more workers help - the goroutines are parked, not competing for CPUs.

Layer 2 is about making the *shape* of the work right. Once it is, the next lever is memory - and in
Go, memory is GC pressure.

---

## Layer 3: Memory & Garbage Collector Management

Go's garbage collector is built for low latency: pauses are sub-millisecond, marking runs at the
same time as the program, and the heap is never compacted. But "low-latency" is not "free." Every
allocation is a future unit of GC work: it must be traced, marked, swept. The cheapest allocation is
the one that never happens.

This layer is about *allocating less*, not just allocating faster.

### 3.1 Escape Analysis Awareness

Go decides at compile time whether a value lives on the **stack** (freed automatically when the
function returns, essentially free) or the **heap** (managed by the GC). This is *escape analysis*,
and the compiler can be asked to show its reasoning:

```bash
go build -gcflags="-m" ./...
# more detail:
go build -gcflags="-m=2" ./...
```

The output states exactly what escapes and why:

```
./escape.go:14:10: xs does not escape
./escape.go:26:2: moved to heap: p
./escape.go:33:11: dst does not escape
```

Here's a runnable package that demonstrates both outcomes:

```go
// escape.go
package escape

type Point struct {
	X, Y int
}

// Sum reads a slice and returns a scalar. Nothing escapes.
// The slice header is passed by value; only the backing array is shared.
//
//go:noinline
func Sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}

// NewPoint returns a pointer to a local, so `p` escapes to the heap.
//
//go:noinline
func NewPoint(x, y int) *Point {
	p := Point{X: x, Y: y}
	return &p // &p escapes to heap
}

// Fill writes into a caller-provided value. Nothing escapes.
//
//go:noinline
func Fill(dst *Point, x, y int) {
	dst.X = x
	dst.Y = y
}
```

Build it and read the compiler's mind:

```bash
go build -gcflags="-m" ./escape.go
```

The output shows `NewPoint`'s local `p` "moved to heap" because the address escapes, while `Sum`
and `Fill` allocate nothing. `Fill` is the fix for `NewPoint`: by letting the caller own the
storage, the value stays off the heap entirely.

> **Gotcha:** `//go:noinline` is a *diagnostic* tool, not an optimization. It prevents the inliner
> from confusing the escape picture while the compiler output is being read. Don't ship it in hot
> code.

Common causes of accidental escape:

| Cause | Fix |
| --- | --- |
| Returning `&local` | Return the value, or let the caller pass in a destination |
| Storing a value in an `interface{}` | Use generics or a concrete type |
| Closures that capture a loop variable by reference | Restructure the closure |
| Slices/maps that grow past their compile-time-known size | Pre-allocate (see 3.2) |
| Calling a method with a pointer receiver on a value | Use a value receiver for small types (see 3.4) |

Escape analysis is the *why* behind allocation counts. Now let's reduce the counts directly.

### 3.2 Pre-allocation

`append` to a `nil` slice works, but it grows the backing array geometrically - 1, 2, 4, 8, 16,
32... Each growth allocates a new array and copies everything over. For a slice whose final size is
known, that's pure waste.

```go
// grow_test.go
package grow

import "testing"

// AppendNoCap lets the slice grow geometrically - repeated allocs + copies.
func AppendNoCap(n int) []int {
	var out []int
	for i := range n {
		out = append(out, i)
	}
	return out
}

// AppendWithCap pre-allocates exactly once.
func AppendWithCap(n int) []int {
	out := make([]int, 0, n)
	for i := range n {
		out = append(out, i)
	}
	return out
}

// MapNoCap grows the map bucket-by-bucket, rehashing as it goes.
func MapNoCap(n int) map[int]int {
	m := map[int]int{}
	for i := range n {
		m[i] = i
	}
	return m
}

// MapWithCap pre-sizes the map's initial bucket array.
func MapWithCap(n int) map[int]int {
	m := make(map[int]int, n)
	for i := range n {
		m[i] = i
	}
	return m
}

func BenchmarkAppendNoCap(b *testing.B) {
	for b.Loop() {
		_ = AppendNoCap(1000)
	}
}

func BenchmarkAppendWithCap(b *testing.B) {
	for b.Loop() {
		_ = AppendWithCap(1000)
	}
}

func BenchmarkMapNoCap(b *testing.B) {
	for b.Loop() {
		_ = MapNoCap(1000)
	}
}

func BenchmarkMapWithCap(b *testing.B) {
	for b.Loop() {
		_ = MapWithCap(1000)
	}
}
```

Measured medians on darwin/arm64 (Go 1.27), `-count=5`:

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| `AppendNoCap` | 2,340 | 25,208 | 12 |
| `AppendWithCap` | **623** | **0** | **0** |
| `MapNoCap` | 36,400 | 74,264 | 20 |
| `MapWithCap` | **8,890** | **36,944** | **5** |

The `AppendWithCap` row deserves a second look: **zero** bytes and **zero** allocations. Because the
final capacity is a compile-time constant and the slice never escapes the function, the compiler
places the entire backing array on the *stack*. That's Layers 3.1 and 3.2 compounding - escape
analysis and pre-allocation together remove the allocation entirely.

Even where the backing array must live on the heap, pre-allocation cuts allocations sharply
(12 -> 0, 20 -> 5). Two numbers matter here: `B/op` and `allocs/op`. The pre-allocated versions don't
just run faster - they leave the GC with far less to do. That's a compounding win in a long-running
service.

> **Gotcha:** Don't over-allocate blindly. `make([]T, 0, 10_000)` for a typical 12-element result
> wastes memory and can *slow things down* by forcing larger GC scans. Pre-allocate to a *realistic
> estimate*, ideally one that can be measured or bounded.

Pre-allocation is about not paying for growth that isn't needed. The next technique is about not
paying for allocation at all.

### 3.3 Object Reuse with `sync.Pool`

Short-lived, frequently-allocated objects are the GC's worst enemy: they're created, used briefly,
and discarded in high volume. `sync.Pool` keeps a set of such objects around for reuse.

The canonical example is a byte buffer in a request handler:

```go
// pool.go
package main

import (
	"bytes"
	"fmt"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// encodeWithPool reuses a pooled buffer instead of allocating a fresh one.
func encodeWithPool(vals []int) string {
	buf := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset() // clear before returning to the pool
		bufPool.Put(buf)
	}()

	for i, v := range vals {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(buf, "%d", v)
	}
	return buf.String()
}

// encodeNoPool allocates a brand-new buffer on every call.
func encodeNoPool(vals []int) string {
	var buf bytes.Buffer
	for i, v := range vals {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%d", v)
	}
	return buf.String()
}

func main() {
	vals := []int{1, 2, 3, 4, 5}
	fmt.Println(encodeWithPool(vals))
	fmt.Println(encodeNoPool(vals))
}
```

The rules for using `sync.Pool` safely:

- **Always `Reset` before `Put`.** Otherwise the next goroutine inherits the previous caller's
  data - possibly sensitive, possibly a `bytes.Buffer` holding megabytes.
- **Never rely on what `Get` returns.** The pool may be emptied at any GC cycle, so `Get` can always
  fall back to calling `New`.
- **Don't pool large objects casually.** A pool of 1MB buffers that gets drained and refilled by the
  GC creates more pressure, not less.
- **Pool the right thing.** Buffers and encoders are ideal. Objects with complex lifecycle are not.

> **Gotcha:** `sync.Pool` is *not* a cache. It is explicitly allowed to drop its contents at any GC
> cycle, and it may return different objects to different goroutines. For a *cache* with eviction
> semantics, use an LRU - not a `sync.Pool`.

> Not every allocation needs pooling, and pooling isn't free. Measure the allocation rate first
> (Layer 1); if `allocs/op` is already near zero, a pool can't help.

### 3.4 Value vs. Pointer Receivers

This one is subtle because it affects both copying cost and escape behavior. The trade-off:

- A **value receiver** copies the struct on every call. For a small struct (a few fields), that copy
  is cheap - often just one or two cache-friendly loads.
- A **pointer receiver** avoids the copy. For a large struct, that's a big win. But taking a pointer
  can force the value to escape to the heap, which means an allocation *and* GC pressure.

```go
// receiver_test.go
package receiver

import "testing"

// Small is 16 bytes on 64-bit - fits in a single cache line and copies cheaply.
type Small struct {
	A, B int64
}

// Large is 256 bytes - copying it is real work.
type Large struct {
	Data [32]int64
}

func (s Small) SumValue() int64 { return s.A + s.B }
func (s *Small) SumPtr() int64  { return s.A + s.B }

func (l Large) SumValue() int64 {
	var t int64
	for _, v := range l.Data {
		t += v
	}
	return t
}
func (l *Large) SumPtr() int64 {
	var t int64
	for _, v := range l.Data {
		t += v
	}
	return t
}

var (
	sinkI64 int64
	small   = Small{A: 1, B: 2}
	large   = Large{}
)

func BenchmarkSmallValue(b *testing.B) {
	for b.Loop() {
		sinkI64 = small.SumValue()
	}
}

func BenchmarkSmallPtr(b *testing.B) {
	for b.Loop() {
		sinkI64 = small.SumPtr()
	}
}

func BenchmarkLargeValue(b *testing.B) {
	for b.Loop() {
		sinkI64 = large.SumValue() // copies 256 bytes each call
	}
}

func BenchmarkLargePtr(b *testing.B) {
	for b.Loop() {
		sinkI64 = large.SumPtr() // copies 8 bytes (the pointer)
	}
}
```

The guidance that falls out:

| Struct size | Preferred receiver | Why |
| --- | --- | --- |
| Small (<= a few words) | **Value** | Copy is cheaper than a pointer indirection; no escape |
| Large | **Pointer** | Avoids copying hundreds of bytes per call |
| Must mutate | **Pointer** | Value receivers mutate only a copy |
| Contains a `sync.Mutex` | **Pointer** | Copying a lock is a bug |

> **Consistency matters more than either choice.** Pick one receiver type per type and stick to it.
> Mixing value and pointer receivers confuses both readers and the method set - a type with a
> mixture only satisfies interfaces in surprising ways.

Memory is now under control. With allocations minimized, it's affordable to look at the actual CPU
instructions in hot paths - which is exactly what Layer 4 is for.

---

## Layer 4: Micro-Optimizations & Language Idioms

This is the layer people reach for first, and it should be the layer they reach for *last*. These
changes win 1x-2x - real, but small. They're worth doing only after Layers 1-3 have already
been addressed, and only for code the profiler has proven is hot.

With that warning firmly in place, here's the good stuff.

### 4.1 String Efficiency

Strings in Go are immutable. `s += "x"` doesn't append - it allocates a brand-new string and copies
everything over. In a loop, that's `O(N^2)` bytes copied for what should be a linear operation.

`strings.Builder` (or `bytes.Buffer`) fixes this with a buffer that grows in large steps, so most
appends cost nothing extra:

```go
// strings_test.go
package strings

import (
	"strings"
	"testing"
)

const n = 1000

// ConcatPlus allocates a new string on every iteration - O(N^2) copying.
func ConcatPlus() string {
	s := ""
	for range n {
		s += "x"
	}
	return s
}

// ConcatBuilder appends into an amortized buffer, with the final size pre-grown.
func ConcatBuilder() string {
	var b strings.Builder
	b.Grow(n) // one allocation, sized exactly
	for range n {
		b.WriteByte('x')
	}
	return b.String()
}

// ConcatJoin collects into a slice and joins once.
func ConcatJoin() string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "x"
	}
	return strings.Join(parts, "")
}

func BenchmarkConcatPlus(b *testing.B) {
	for b.Loop() {
		_ = ConcatPlus()
	}
}

func BenchmarkConcatBuilder(b *testing.B) {
	for b.Loop() {
		_ = ConcatBuilder()
	}
}

func BenchmarkConcatJoin(b *testing.B) {
	for b.Loop() {
		_ = ConcatJoin()
	}
}
```

Measured medians on the same machine (`-count=5`, `n = 1000`):

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| `ConcatPlus` | 63,200 | 530,278 | 999 |
| `ConcatBuilder` | **2,130** | **1,024** | **1** |
| `ConcatJoin` | 9,090 | 1,024 | 1 |

`ConcatPlus` is ~30x slower *and* allocates ~500x more bytes. That gap is the `O(N^2)` copy cost
showing up exactly where the complexity analysis said it would - and it grows with `n`.

`strings.Builder` is preferred over `bytes.Buffer` for pure string building: it exposes
`WriteString`, `Grow`, and `WriteByte` directly, and its `String()` method can hand back the
internal buffer *without copying* at the end (thanks to an `unsafe` optimization in the standard
library). Note the `b.Grow(n)` - that's the Layer 3 pre-allocation lesson, applied to strings.

For the byte/string boundary, Go 1.20 added zero-copy conversions in the `unsafe` package:

```go
// zero_copy.go
package main

import (
	"fmt"
	"unsafe"
)

// bytesToString converts without copying. The result MUST NOT be mutated,
// and it aliases the input's memory.
func bytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// stringToBytes aliases the string's memory. The result MUST NOT be mutated.
func stringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func main() {
	b := []byte("hello")
	s := bytesToString(b)
	fmt.Println(s)
}
```

> **Gotcha:** These are *aliasing* conversions, not copies. The returned value shares memory with
> the input. Mutating the `[]byte` afterward changes the `string` too - which breaks the
> immutability guarantee the rest of the language relies on, and can cause baffling bugs. Use
> `unsafe` conversions only after a profile proves the copy is a bottleneck, and document the
> contract loudly.

### 4.2 Compiler Directives & Inlining

Go's compiler inlines small functions automatically. Inlining removes call overhead *and* often
unlocks further optimizations (constant folding, escape improvements) that the optimizer couldn't
reason about across a function boundary.

The inliner's decisions are visible in the compiler output:

```bash
go build -gcflags="-m" ./...      # inline + escape summary
go build -gcflags="-m=2" ./...    # verbose: budgets, reasons, decisions
```

The output is explicit about budgets - compiled against the example below, it reads:

```
./hot.go:7:6: can inline add
./hot.go:15:13: xs does not escape
./hot.go:18:14: inlining call to add
```

When a function is too large, the budget is instead exceeded:

```
./hot.go:20:6: cannot inline big: function too complex (cost 87 > 80)
```

The number in parentheses is the inlining *cost* against a budget (80 by default). Small functions
under budget get inlined; large ones don't. There's a directive - `//go:noinline` - but note the
asymmetry: **there is no supported `//go:inline`.** Inlining can't be forced. A function can only be
made small enough to qualify, or the call site restructured.

```go
// hot.go
package hot

// add is tiny: cost well under budget, so it inlines.
func add(a, b int) int {
	return a + b
}

// SumAdd adds values one at a time. Because add inlines,
// the compiler can optimize the loop body without a call boundary.
//
//go:noinline
func SumAdd(xs []int) int {
	total := 0
	for _, x := range xs {
		total = add(total, x)
	}
	return total
}
```

The practical moves:

- **Keep hot-path functions small.** A wrapper that just calls another function is inlined away -
  good. A 200-line function is not.
- **Keep call sites attractive.** Passing a function value or an interface generally blocks
  inlining (until PGO/devirtualization, below).
- **Check with `-m=2` before assuming.** Don't guess whether something inlined.

> **Gotcha:** Inlining is a budget, not a goal. It bloats code size and can hurt instruction-cache
> behavior. The right question isn't "did it inline?" but "did the benchmark get faster?"

### 4.3 Standard Library Alternatives

`encoding/json` is a general-purpose, reflection-based encoder. It's correct, well-tested, and
perfectly adequate for most uses. It's also allocation-heavy, because it discovers struct shape at
runtime via reflection.

For throughput-critical paths, two alternatives dominate:

1. **Hand-rolled, zero-allocation marshaling** for a specific type.
2. **Code-generation or JIT libraries** for general structs.

Here's the hand-rolled version for a small, known type:

```go
// fastjson.go
package fastjson

import (
	"strconv"
)

type Metric struct {
	Name  string
	Value int64
}

// AppendJSON builds the JSON directly into dst - no reflection, no fmt.
func (m Metric) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"name":`...)
	dst = strconv.AppendQuote(dst, m.Name) // no reflection, no fmt
	dst = append(dst, `,"value":`...)
	dst = strconv.AppendInt(dst, m.Value, 10)
	dst = append(dst, '}')
	return dst
}
```

`strconv.AppendQuote` and `strconv.AppendInt` write directly into a caller-owned `[]byte` - no
intermediate string, no reflection, no `fmt`. Combined with the `sync.Pool` from 3.3, a service can
serialize a stream of metrics with **zero** steady-state allocations.

For arbitrary structs, the ecosystem offers libraries with different trade-offs:

| Library | Approach | Best for |
| --- | --- | --- |
| `encoding/json` | Reflection | Correctness, low-volume, any struct |
| `easyjson` | Code generation (`easyjson -all`) | Stable schemas, no JIT budget |
| `sonic` | JIT + SIMD (amd64/arm64) | Very high throughput, modern CPUs |
| `go-json` | Reflection, but much faster | Drop-in, no codegen step |

> **Gotcha:** Every alternative to `encoding/json` is a *compatibility* liability. They must match
> its behavior on edge cases - HTML escaping, `omitempty`, number precision, map ordering. Pin the
> version, and keep a differential test that runs both encoders on a corpus and asserts identical
> output.

Micro-optimizations are the last code-level layer. The final layer isn't about code at all - it's
about the runtime and the operating system hosting it.

---

## Layer 5: Runtime & OS Level

At the bottom of the pyramid, the focus shifts from changing programs to changing the environment
they run in. These settings are cheap to change and often overlooked, but they're also easy to get
wrong - and a misconfigured runtime can undo a lot of good work above.

### 5.1 `GOMAXPROCS` & Memory Limit Tuning

`GOMAXPROCS` controls how many OS threads can execute Go code simultaneously. Historically, it
defaulted to the number of *logical CPUs the machine reports* - which, on a container limited to 2
CPUs on a 64-core host, is badly wrong. The runtime would spin up 64 Ps and thrash.

Since **Go 1.25**, the runtime reads cgroup CPU limits and sets `GOMAXPROCS` to the *container's*
allocation automatically. It can also be set explicitly, at runtime, with
`runtime.SetDefaultGOMAXPROCS`.

`GOMEMLIMIT` (Go 1.19+) is the other half. It's a *soft* memory limit: the runtime uses it as a GC
target, aiming to keep total heap below it. Crucially, it does **not** cause allocation to fail -
it changes *when the GC runs*, not whether `make` succeeds. For a container, it should sit just
below the cgroup memory limit to keep the GC working before the kernel OOM-killer arrives.

```go
// limits.go
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

func main() {
	// Container-aware by default since Go 1.25; here we set it explicitly.
	prev := runtime.GOMAXPROCS(4)
	fmt.Printf("GOMAXPROCS: %d -> %d\n", prev, runtime.GOMAXPROCS(0))

	// Soft heap limit: aim to keep the Go heap under 512 MiB.
	// Set this below the container's hard memory limit.
	debug.SetMemoryLimit(512 << 20) // 512 MiB

	// Report the effective limit.
	fmt.Printf("GOMEMLIMIT: %d bytes\n", debug.SetMemoryLimit(-1))
}
```

The same settings are available via environment variables, which is usually how a container is
configured:

```bash
GOMEMLIMIT=512MiB GOMAXPROCS=4 ./myservice
```

| Setting | Default | When to set it |
| --- | --- | --- |
| `GOMAXPROCS` | Container-aware since Go 1.25 | Rarely - only to override the runtime |
| `GOMEMLIMIT` | Off (unlimited) | **Always, in containers** - set just below the cgroup limit |

> **Gotcha:** `GOMEMLIMIT` is not a hard cap. If the live heap genuinely exceeds it, the GC will
> run continuously ("GC death spiral") and the process will still eventually be killed by the
> kernel. It buys graceful degradation and earlier GC - not immunity from a real leak.

### 5.2 Profile-Guided Optimization (PGO)

PGO is the closest thing Go has to a "free" speedup. Introduced in **Go 1.20**, it feeds a CPU
profile from production back into the compiler, which uses that *actual* runtime behavior to make
more informed decisions. The official docs describe the mechanism plainly:

> ...the compiler may decide to more aggressively inline functions which the profile indicates are
> called frequently.

That's the core of it: **inlining decisions driven by real execution data**, plus related wins such
as devirtualizing interface calls when the profile shows one concrete type dominates.

The workflow is a loop, not a one-off:

```bash
# 1. Release an initial binary (no PGO).
# 2. Collect a profile from production (the /debug/pprof/profile
#    endpoint from 1.1 is ideal).
curl -o cpu.pprof 'http://localhost:6060/debug/pprof/profile?seconds=30'

# 3. Store it next to the main package and rebuild.
cp cpu.pprof ./cmd/myservice/default.pgo
go build ./cmd/myservice      # -pgo=auto is the default since Go 1.21

# 4. Repeat: collect a fresh profile from the new (better) binary.
```

Since Go 1.21, `-pgo=auto` is the **default**: if a `default.pgo` file sits next to the `main`
package, it's used automatically. (Before Go 1.21, the default was `-pgo=off`.) Passing an explicit
path works too - `go build -pgo=/tmp/cpu.pprof` - but note that a path applies to *all* main packages
in the invocation, so per-binary profiles need separate builds.

Two practices from the docs are worth adopting early:

- **Commit `default.pgo` to the repository.** Profiles are build inputs, and checking them in keeps
  builds reproducible (and fast) with no extra fetch step.
- **Merge profiles from several instances** for a more representative input:
  `go tool pprof -proto a.pprof b.pprof > merged.pprof`.

Expectations, honestly stated:

| Aspect | Reality |
| --- | --- |
| Typical gain | **~5%-15%** on representative Go programs (Go 1.22 benchmarks) |
| Scope | The **entire program** - standard library and dependencies included |
| Cost at build time | Slower builds (all packages rebuild on first use of a profile) |
| Cost at run time | None |
| Binary size | Slightly larger (extra inlining) |
| Best candidates | Large, interface-heavy programs with a hot call graph |
| Worst candidates | Microbenchmarks and tiny programs |

> **A correction worth making:** PGO is often described as enabling "tail-call optimizations." The
> official docs don't say that, and it isn't true: the Go compiler performs **no general tail-call
> optimization** (it would break stack traces and `runtime.Callers`). PGO's documented win is
> *aggressive inlining of hot functions*, alongside related optimizations such as devirtualization.

The profile should be *representative*: capture it during a realistic workload, not during startup or
a synthetic benchmark. A profile of the wrong workload optimizes the wrong thing - though, reassuringly,
the docs note it should never make a program *slower* than no PGO. If it does, that's a bug worth
[reporting](https://go.dev/issue/new).

### 5.3 Atomic & Lock-Free Synchronization

A `sync.Mutex` is a heavyweight primitive: it has a fast path, but under contention it parks
goroutines, and even uncontended it involves an atomic compare-and-swap plus memory barriers. For
simple counters and flags, `sync/atomic` can be dramatically cheaper.

```go
// atomic_test.go
package atomicbench

import (
	"sync"
	"sync/atomic"
	"testing"
)

var (
	mu      sync.Mutex
	muCount int64
	atCount atomic.Int64
)

func BenchmarkMutex(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			muCount++
			mu.Unlock()
		}
	})
}

func BenchmarkAtomic(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atCount.Add(1)
		}
	})
}
```

Under parallel contention (`b.RunParallel`, 8 cores), the atomic version measured **38 ns/op**
against the mutex's **87 ns/op** - about 2.3x faster, and the gap widens as contention grows. It's a
single hardware instruction (or a short CAS loop) versus a lock acquisition, a critical section, and
a release. And for read-mostly data, `atomic.Pointer[T]` publishes an immutable snapshot
without any lock at all - the RCU-style pattern:

```go
// snapshot.go
package main

import (
	"fmt"
	"sync/atomic"
)

type Config struct {
	MaxConns int
	Timeout  int
}

// current holds an *immutable* *Config. Readers load it lock-free;
// writers publish a whole new value.
var current atomic.Pointer[Config]

// Load returns the current config without locking.
func Load() *Config {
	return current.Load()
}

// Publish swaps in a new config. Never mutate a published *Config -
// build a fresh one and swap the pointer.
func Publish(c *Config) {
	current.Store(c)
}

func main() {
	Publish(&Config{MaxConns: 100, Timeout: 30})
	fmt.Printf("max conns: %d\n", Load().MaxConns)
}
```

> **Gotcha:** Atomics are not a drop-in replacement for every lock. They shine for *single-word*
> counters, flags, and pointer swaps. Updating *several* fields together, or maintaining an
> invariant across them, requires a lock (or an immutable snapshot, as above). Reaching for
> `atomic` where a mutex is needed produces subtle, data-race-shaped bugs that `-race` will find
> and inspection will not.

Always verify concurrency changes under the race detector:

```bash
go test -race ./...
```

---

## The Checklist

Here's the audit to apply to a Go service, in order.

**Layer 1 - Measure**
- [ ] Capture a CPU profile from *production*, not a dev laptop.
- [ ] Capture a heap profile; distinguish `alloc_*` from `inuse_*`.
- [ ] If CPU is low but latency is high, run the execution tracer.
- [ ] Establish a benchmark baseline with `-benchmem -count=10`.
- [ ] Compare changes with `benchstat`, not by eyeballing `ns/op`.

**Layer 2 - Algorithm & architecture**
- [ ] Check complexity: is there an `O(N^2)` hiding in a loop?
- [ ] Verify indexes exist for hot queries; eliminate N+1 patterns.
- [ ] Batch I/O: writes, requests, log lines, channel sends.
- [ ] Size worker pools to the work; don't spawn per item.
- [ ] Aggregate results per-worker, not per-item.

**Layer 3 - Memory & GC**
- [ ] Read `-gcflags="-m"` for hot functions; eliminate accidental escapes.
- [ ] Pre-allocate slices and maps to a realistic size.
- [ ] Pool genuinely hot, short-lived objects with `sync.Pool`.
- [ ] `Reset()` pooled buffers before `Put()`.
- [ ] Pick one receiver type per type; value for small, pointer for large.

**Layer 4 - Micro**
- [ ] Replace `+` concatenation in loops with `strings.Builder` + `Grow`.
- [ ] Check `-gcflags="-m=2"` for hot functions that failed to inline.
- [ ] Consider a faster JSON path *only* for proven hot paths.
- [ ] Differential-test any `encoding/json` replacement.
- [ ] Use `unsafe` conversions only with a documented aliasing contract.

**Layer 5 - Runtime & OS**
- [ ] Set `GOMEMLIMIT` below the container memory limit.
- [ ] Confirm `GOMAXPROCS` matches the container (automatic since Go 1.25).
- [ ] Generate a `default.pgo` from a representative production profile.
- [ ] Replace single-word locks with atomics where contention is measured.
- [ ] Run `go test -race ./...` after any concurrency change.

---

## Conclusion

Optimization in Go is a *search*, and the five layers are the map. Start at the top: measure until
the data shows where the time actually goes. Fix the algorithm and architecture first, because that's
where the order-of-magnitude wins hide. Then reduce allocation so the GC has less to do. Only then
descend into micro-optimizations - and only for code the profiler has already indicted. Finally,
tune the runtime environment itself.

The deepest truth in this post is the least glamorous one: **the fastest code is the code that
doesn't run.** A batch that replaces 100 round trips with one. A pre-allocated slice that never
regrows. A linear scan that never hashes. A pooled buffer that's never collected. Every one of these
wins comes from *removing work*, not from making work faster.

The compiler is very good at the latter. Our job is the former.

So the next time the urge arises to reach for `unsafe` or rewrite a `fmt.Sprintf` - stop. Run the
profiler first. Let the data reveal which layer the problem is in. That's the whole framework:
measure, descend, verify, repeat.

---

## References & Examples

**Examples**

- [Runnable examples from this post](https://github.com/serhii-chechun/developers-blog/tree/main/programming/go/optimizations/examples) - one folder per layer.

**Profiling & Diagnostics**

- [`runtime/pprof` package](https://pkg.go.dev/runtime/pprof)
- [`net/http/pprof` package](https://pkg.go.dev/net/http/pprof)
- [`runtime/trace` package](https://pkg.go.dev/runtime/trace)
- [Profiling Go Programs](https://go.dev/blog/pprof) - the official Go blog introduction
- [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) - statistical benchmark comparison; install with `go install golang.org/x/perf/cmd/benchstat@latest`

**Runtime & Compiler**

- [`runtime/debug` package](https://pkg.go.dev/runtime/debug) - `SetMemoryLimit`, `SetGCPercent`
- [A Guide to the Go Garbage Collector](https://go.dev/doc/gc-guide) - the definitive reference, including `GOMEMLIMIT`
- [Profile-Guided Optimization](https://go.dev/doc/pgo) - the official PGO documentation
- [Go Compiler Directives](https://pkg.go.dev/cmd/compile#hdr-Compiler_Directives) - `//go:noinline`, `//go:linkname`, and friends
- [Go Execution Tracer](https://go.dev/doc/diagnostics#execution-tracer)

**Language & Libraries**

- [`sync` package](https://pkg.go.dev/sync) - `sync.Pool`, `sync.Mutex`
- [`sync/atomic` package](https://pkg.go.dev/sync/atomic) - `atomic.Int64`, `atomic.Pointer[T]`
- [`strings.Builder`](https://pkg.go.dev/strings#Builder)
- [`unsafe.String`](https://pkg.go.dev/unsafe#String) / [`unsafe.Slice`](https://pkg.go.dev/unsafe#Slice) - zero-copy conversions (Go 1.20+)
- [`encoding/json`](https://pkg.go.dev/encoding/json)
- [`sonic`](https://github.com/bytedance/sonic), [`easyjson`](https://github.com/mailru/easyjson), [`go-json`](https://github.com/goccy/go-json) - high-performance JSON alternatives

**On the Golden Rule**

- [Donald Knuth, "Structured Programming with go to Statements"](https://dl.acm.org/doi/10.1145/356635.356640) - the origin of "premature optimization is the root of all evil"
