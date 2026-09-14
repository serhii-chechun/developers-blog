/*
	15_memlimit shows GOMAXPROCS and GOMEMLIMIT tuning at runtime.

$ go run ./15_memlimit
// Or configure via environment variables:
$ GOMEMLIMIT=512MiB GOMAXPROCS=4 go run main.go
*/
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

	// Report the effective limit (negative input reads without changing it).
	fmt.Printf("GOMEMLIMIT: %d bytes\n", debug.SetMemoryLimit(-1))
}
