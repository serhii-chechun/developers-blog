/*
	04_benchmarking shows a baseline benchmark.

$ go test -run=^$ -bench=. -benchmem -count=10 .
*/
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
