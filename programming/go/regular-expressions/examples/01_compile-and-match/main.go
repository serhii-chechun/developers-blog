package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
MustCompile pattern matches 'ERROR': true
Compile returned a recoverable error: error parsing regexp: missing closing ]: `[`
Valid user pattern matches 'WARNING': true
*/

func main() {
	// (?i)\berror\b - match the whole word "error", case-insensitive
	//
	// (?i)  - inline flag: case-insensitive matching
	// \b    - word boundary (start)
	// error - literal text "error"
	// \b    - word boundary (end)
	mustCompile(`(?i)\berror\b`)

	// \berror\b[ - broken pattern: missing closing bracket
	//
	// \b    - word boundary
	// error - literal text "error"
	// \b    - word boundary
	// [     - starts a character class that is never closed (syntax error)
	compileBrokenPattern(`\berror\b[`)

	// \b(?i)warn(ing)?\b - match "warn" or "warning", case-insensitive
	//
	// \b     - word boundary (start)
	// (?i)   - inline flag: case-insensitive
	// warn   - literal text "warn"
	// (ing)? - optional group: "ing", matched zero or one time
	// \b     - word boundary (end)
	compileValidPattern(`\b(?i)warn(ing)?\b`)
}

// MustCompile: the pattern is part of our program,
// so a panic on init is the right behavior for a typo.
func mustCompile(pattern string) {
	var hardcodedRe = regexp.MustCompile(pattern)
	fmt.Printf(
		"MustCompile pattern matches 'ERROR': %v\n",
		hardcodedRe.MatchString("some ERROR happened"),
	)
}

// Compile: the pattern comes from the user (e.g. CLI flag or config),
// so we must handle the error gracefully instead of crashing.
func compileBrokenPattern(pattern string) {
	if _, err := regexp.Compile(pattern); err != nil {
		fmt.Printf("Compile returned a recoverable error: %v\n", err)
	}
}

// Compile succeeds for a valid user pattern, giving a reusable object.
func compileValidPattern(pattern string) {
	valid, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"Valid user pattern matches 'WARNING': %v\n",
		valid.MatchString("please take this WARNING seriously"),
	)
}
