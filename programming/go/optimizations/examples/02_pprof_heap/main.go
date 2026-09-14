/*
	02_pprof_heap demonstrates writing a heap profile with runtime/pprof.

$ go run main.go
$ go tool pprof -http=:8080 heap.prof
*/
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
// unreachable after its final use, leaving a heap profile that reports only
// runtime bookkeeping and never mentions buildRows.
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
