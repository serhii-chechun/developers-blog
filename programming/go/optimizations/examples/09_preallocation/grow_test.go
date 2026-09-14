/*
	09_preallocation measures geometric growth vs. pre-sized allocation.

$ go test -run=^$ -bench=. -benchmem .
*/
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
