package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
'alice@acme.com' is an email: true
'not-an-email' is an email: false
  "a@b.co"   valid: true
  "broken"   valid: false
  "x@y.zzz"  valid: true
*/

func main() {
	// ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ - a fully anchored email address
	// ^                 - start-of-string anchor
	// [a-zA-Z0-9._%+-]+ - one or more local-part chars
	// @                 - literal "@"
	// [a-zA-Z0-9.-]+    - one or more domain chars
	// \.                - a literal "."
	// [a-zA-Z]{2,}      - two or more letters (the TLD)
	// $                 - end-of-string anchor
	matchStringOnce(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// the same anchored email pattern, compiled once and reused
	reuseCompiled(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
}

// MatchString: a one-off check that compiles internally.
func matchStringOnce(pattern string) {
	ok, _ := regexp.MatchString(pattern, "alice@acme.com")
	fmt.Printf("'alice@acme.com' is an email: %v\n", ok)

	ok2, _ := regexp.MatchString(pattern, "not-an-email")
	fmt.Printf("'not-an-email' is an email: %v\n", ok2)
}

// For repeated checks, compile once and reuse the *Regexp.
func reuseCompiled(pattern string) {
	re := regexp.MustCompile(pattern)
	for _, candidate := range []string{"a@b.co", "broken", "x@y.zzz"} {
		fmt.Printf("  %-10q valid: %v\n", candidate, re.MatchString(candidate))
	}
}
