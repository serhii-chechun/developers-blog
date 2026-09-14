/*
	12_strings contrasts concatenation, strings.Builder, and strings.Join.

$ go test -run=^$ -bench=. -benchmem .
*/
package strbench

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
