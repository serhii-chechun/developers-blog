/*
	17_atomics contrasts a sync.Mutex counter with a sync/atomic counter,
	plus a lock-free pointer snapshot pattern.

$ go test -run=^$ -bench=. -benchmem .
*/
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

// Config is published as an immutable snapshot, read without locks.
type Config struct {
	MaxConns int
	Timeout  int
}

var current atomic.Pointer[Config]

// LoadConfig returns the current config without locking.
func LoadConfig() *Config { return current.Load() }

// PublishConfig swaps in a new config. Never mutate a published *Config.
func PublishConfig(c *Config) { current.Store(c) }

func TestSnapshot(t *testing.T) {
	PublishConfig(&Config{MaxConns: 100, Timeout: 30})
	if got := LoadConfig().MaxConns; got != 100 {
		t.Fatalf("MaxConns = %d, want 100", got)
	}
}
