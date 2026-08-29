package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
TSV fields: [alice 42 sales]
Command tokens: [ go run ./main.go ]
Split with n=3: [a b c d e]
*/

func main() {
	// \t - a literal tab (0x09)
	splitTsv(`\t`)

	// \s+ - one or more whitespace chars (space, tab, newline, ...)
	splitCommand(`\s+`)

	// \s+ - same whitespace pattern, limited to 3 substrings
	splitLimited(`\s+`)
}

// Split a TSV row on tab characters.
func splitTsv(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("TSV fields: %v\n", re.Split("alice\t42\tsales", -1))
}

// Split on runs of whitespace to tokenize a command line.
func splitCommand(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("Command tokens: %v\n", re.Split("  go   run  ./main.go ", -1))
}

// Split with n limits the number of resulting substrings.
func splitLimited(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("Split with n=3: %v\n", re.Split("a b c d e", 3))
}
