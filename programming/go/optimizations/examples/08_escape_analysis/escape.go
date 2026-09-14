/*
	08_escape_analysis shows what the compiler puts on the heap and why.

$ go build -gcflags="-m" escape.go
*/
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
