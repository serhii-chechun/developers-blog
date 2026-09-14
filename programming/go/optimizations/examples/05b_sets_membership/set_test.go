/*
	05b_sets_membership compares four ways to answer "is this key in the set?",
	for the same 16-key set.

$ go test -run=^$ -bench=. -benchmem -count=5 .
*/
package set

import (
	"slices"
	"testing"
)

const size = 16

// keys holds "a".."p". The benchmarks look up "k" (a hit) and "z" (a miss).
var keys = func() []string {
	ks := make([]string, size)
	for i := range ks {
		ks[i] = string(rune('a' + i))
	}
	return ks
}()

// boolSet allows the tempting `if m[key] { ... }`, but cannot distinguish
// "absent" from "present and false".
var boolSet = func() map[string]bool {
	m := make(map[string]bool, size)
	for _, k := range keys {
		m[k] = true
	}
	return m
}()

// structSet is the idiomatic set: no value to store, no ambiguous state.
var structSet = func() map[string]struct{} {
	m := make(map[string]struct{}, size)
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}()

// containsBoolSet: map[string]bool membership test.
func containsBoolSet(key string) bool { return boolSet[key] }

// containsStructSet: map[string]struct{} membership test.
func containsStructSet(key string) bool {
	_, ok := structSet[key]
	return ok
}

// containsSlice: linear scan - no data structure at all.
func containsSlice(key string) bool { return slices.Contains(keys, key) }

// containsSwitch: compile-time constants, grouped into one case.
func containsSwitch(key string) bool {
	switch key {
	case "a", "b", "c", "d", "e", "f", "g", "h",
		"i", "j", "k", "l", "m", "n", "o", "p":
		return true
	}
	return false
}

// --- hit: "k" is in the set ---

func BenchmarkHitBoolSet(b *testing.B) {
	for b.Loop() {
		_ = containsBoolSet("k")
	}
}

func BenchmarkHitStructSet(b *testing.B) {
	for b.Loop() {
		_ = containsStructSet("k")
	}
}

func BenchmarkHitSlice(b *testing.B) {
	for b.Loop() {
		_ = containsSlice("k")
	}
}

func BenchmarkHitSwitch(b *testing.B) {
	for b.Loop() {
		_ = containsSwitch("k")
	}
}

// --- miss: "z" is not in the set ---

func BenchmarkMissBoolSet(b *testing.B) {
	for b.Loop() {
		_ = containsBoolSet("z")
	}
}

func BenchmarkMissStructSet(b *testing.B) {
	for b.Loop() {
		_ = containsStructSet("z")
	}
}

func BenchmarkMissSlice(b *testing.B) {
	for b.Loop() {
		_ = containsSlice("z")
	}
}

func BenchmarkMissSwitch(b *testing.B) {
	for b.Loop() {
		_ = containsSwitch("z")
	}
}
