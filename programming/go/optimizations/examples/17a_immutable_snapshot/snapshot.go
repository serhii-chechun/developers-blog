/*
	17a_immutable_snapshot RCU-style immutable snapshot publication with atomic.Pointer[T]

$ go run snapshot.go
*/
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
