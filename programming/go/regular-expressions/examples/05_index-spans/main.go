package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
FindStringIndex -> [5 11]
  slice doc[5:11] = "regexp"
FindIndex -> [5 11]
All 'regexp' spans -> [[5 11] [25 31]]
  "regexp" at bytes [5, 11)
  "regexp" at bytes [25, 31)
FindStringIndex returned nil when nothing matched
*/

func main() {
	// (?i)regexp - the word "regexp", case-insensitive
	// (?i)   - inline flag: case-insensitive matching
	// regexp - literal text "regexp"
	findStringIndex(`(?i)regexp`)

	// same pattern via FindIndex on []byte
	findIndex(`(?i)regexp`)

	// every span of the case-insensitive pattern
	findAllStringIndex(`(?i)regexp`)

	// no match -> nil
	findStringIndexNoMatch(`(?i)regexp`)
}

// FindStringIndex: byte offsets [start, end] of the first match.
func findStringIndex(pattern string) {
	re := regexp.MustCompile(pattern)
	doc := "Go's regexp package. The regexp engine is fast."
	idx := re.FindStringIndex(doc)
	fmt.Printf("FindStringIndex -> %v\n", idx)
	fmt.Printf("  slice doc[%d:%d] = %q\n", idx[0], idx[1], doc[idx[0]:idx[1]])
}

// FindIndex: the []byte variant.
func findIndex(pattern string) {
	re := regexp.MustCompile(pattern)
	doc := []byte("Go's regexp package. The regexp engine is fast.")
	fmt.Printf("FindIndex -> %v\n", re.FindIndex(doc))
}

// FindAllStringIndex: every match's span, useful for highlighting.
func findAllStringIndex(pattern string) {
	re := regexp.MustCompile(pattern)
	doc := "Go's regexp package. The regexp engine is fast."
	all := re.FindAllStringIndex(doc, -1)
	fmt.Printf("All 'regexp' spans -> %v\n", all)
	for _, pair := range all {
		fmt.Printf("  %q at bytes [%d, %d)\n", doc[pair[0]:pair[1]], pair[0], pair[1])
	}
}

// No match returns nil.
func findStringIndexNoMatch(pattern string) {
	re := regexp.MustCompile(pattern)
	if re.FindStringIndex("nothing here") == nil {
		fmt.Println("FindStringIndex returned nil when nothing matched")
	}
}
