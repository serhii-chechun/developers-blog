package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
CompilePOSIX leftmost-longest -> "foo"
Default engine (leftmost-first) -> "fo"
CompilePOSIX rejects \d: error parsing regexp: invalid escape sequence: `\d`
*/

func main() {
	// fo|foo - matches "fo" or "foo"; POSIX picks the longer "foo"
	// fo  - alternative 1: literal "fo"
	// |   - alternation ("or")
	// foo - alternative 2: literal "foo"
	posixLeftmostLongest(`fo|foo`)

	// fo|foo - same pattern, but the default engine picks the first alternative "fo"
	defaultLeftmostFirst(`fo|foo`)

	// \d+ - one or more digits (a Perl class, not valid POSIX)
	posixRejectsPerl(`\d+`)
}

// CompilePOSIX: leftmost-longest matching.
func posixLeftmostLongest(pattern string) {
	re := regexp.MustCompilePOSIX(pattern)
	fmt.Printf("CompilePOSIX leftmost-longest -> %q\n", re.FindString("foobar"))
}

// Default engine: leftmost-first matching.
func defaultLeftmostFirst(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("Default engine (leftmost-first) -> %q\n", re.FindString("foobar"))
}

// POSIX rejects Perl classes like \d.
func posixRejectsPerl(pattern string) {
	_, err := regexp.CompilePOSIX(pattern)
	if err != nil {
		fmt.Printf("CompilePOSIX rejects \\d: %v\n", err)
	}
}
