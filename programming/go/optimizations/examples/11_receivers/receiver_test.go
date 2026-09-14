/*
	11_receivers contrasts value and pointer receivers by struct size.

$ go test -run=^$ -bench=. -benchmem .
*/
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
