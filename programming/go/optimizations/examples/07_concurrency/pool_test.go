/*
	07_concurrency contrasts one goroutine per item with a bounded worker pool.

$ go test -run=^$ -bench=. -benchmem .
*/
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

	for i := range items {
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
