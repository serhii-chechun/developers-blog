/*
	05_data_structures contrasts a linear slice scan with a map lookup.

$ go test -run=^$ -bench=. -benchmem .
*/
package datastructures

import "testing"

type entry struct {
	key string
	val int
}

const size = 16

var sliceEntries = func() []entry {
	es := make([]entry, 0, size)
	for i := range size {
		es = append(es, entry{key: string(rune('a' + i)), val: i})
	}
	return es
}()

var mapEntries = func() map[string]int {
	m := make(map[string]int, size)
	for _, e := range sliceEntries {
		m[e.key] = e.val
	}
	return m
}()

// lookupSlice scans linearly - contiguous memory, no hashing.
func lookupSlice(key string) int {
	for i := range sliceEntries {
		if sliceEntries[i].key == key {
			return sliceEntries[i].val
		}
	}
	return -1
}

// lookupMap hashes the key and probes a bucket.
func lookupMap(key string) int {
	return mapEntries[key]
}

func BenchmarkLookupSlice(b *testing.B) {
	for b.Loop() {
		_ = lookupSlice("k")
	}
}

func BenchmarkLookupMap(b *testing.B) {
	for b.Loop() {
		_ = lookupMap("k")
	}
}
