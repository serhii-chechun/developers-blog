/* 12b_zero_copy shows the Go 1.20+ unsafe string/byte aliasing conversions.

$ go run main.go
*/

// WARNING: these conversions alias memory. The results must not be mutated,
// and the input must outlive the result.
package main

import (
	"fmt"
	"unsafe"
)

// bytesToString converts without copying. The result MUST NOT be mutated,
// and it aliases the input's memory.
func bytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// stringToBytes aliases the string's memory. The result MUST NOT be mutated.
func stringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func main() {
	b := []byte("hello")
	s := bytesToString(b)
	fmt.Println(s)
	fmt.Println(string(stringToBytes(s)))
}
