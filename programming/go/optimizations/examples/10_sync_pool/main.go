/*
	10_sync_pool demonstrates reusing buffers with sync.Pool.

$ go run main.go
*/
package main

import (
	"bytes"
	"fmt"
	"sync"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// encodeWithPool reuses a pooled buffer instead of allocating a fresh one.
func encodeWithPool(vals []int) string {
	buf := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset() // clear before returning to the pool
		bufPool.Put(buf)
	}()

	for i, v := range vals {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(buf, "%d", v)
	}
	return buf.String()
}

// encodeNoPool allocates a brand-new buffer on every call.
func encodeNoPool(vals []int) string {
	var buf bytes.Buffer
	for i, v := range vals {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%d", v)
	}
	return buf.String()
}

func main() {
	vals := []int{1, 2, 3, 4, 5}
	fmt.Println(encodeWithPool(vals))
	fmt.Println(encodeNoPool(vals))
}
