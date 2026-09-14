/*
	13_inlining shows the compiler's inlining decisions.

$ go build -gcflags="-m=2" hot.go
*/
package hot

// add is tiny: cost well under budget, so it inlines.
func add(a, b int) int {
	return a + b
}

// SumAdd adds values one at a time. Because add inlines,
// the compiler can optimize the loop body without a call boundary.
//
//go:noinline
func SumAdd(xs []int) int {
	total := 0
	for _, x := range xs {
		total = add(total, x)
	}
	return total
}
