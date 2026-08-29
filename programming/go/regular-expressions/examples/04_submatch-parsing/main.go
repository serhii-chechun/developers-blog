package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
FindStringSubmatch -> [alice@acme.com alice acme.com]
  user: "alice"  host: "acme.com"
FindSubmatch -> ["alice@acme.com" "alice" "acme.com"]
Optional group absent -> ["@example" "example" ""] (index 2 is empty)
*/

func main() {
	// ([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+) - user@host, capturing both parts
	//
	//   ([a-zA-Z0-9._%+-]+) - group 1: the user (capturing)
	//     [a-zA-Z0-9._%+-]  - one char from the local-part set (letters, digits, . _ % + -)
	//     +                 - one or more of the preceding
	//   @                   - literal "@"
	//   ([a-zA-Z0-9.-]+)    - group 2: the host (capturing)
	//     [a-zA-Z0-9.-]     - one char from the domain set (letters, digits, dot, hyphen)
	//     +                 - one or more of the preceding
	submatchString(`([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+)`)

	// same email pattern, read from []byte data
	submatchBytes(`([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+)`)

	// (?i)@([a-z]+)(?:\.(\w+))? - host with an optional TLD capture group, case-insensitive
	//
	//   (?i)         - inline flag: case-insensitive matching
	//   @            - literal "@"
	//   ([a-z]+)     - group 1: the host letters (capturing)
	//     [a-z]      - one lowercase letter (matched case-insensitively due to (?i))
	//     +          - one or more of the preceding
	//   (?:\.(\w+))? - optional non-capturing group: a literal "." then a captured TLD
	//     (?:...)    - non-capturing group (no capture)
	//     \.         - a literal "."
	//     (\w+)      - group 2: the TLD (one or more word chars), capturing
	//     ?          - the whole group is optional (zero or one time)
	optionalGroup(`(?i)@([a-z]+)(?:\.(\w+))?`)
}

// FindStringSubmatch: full match plus captured groups.
func submatchString(pattern string) {
	re := regexp.MustCompile(pattern)
	sub := re.FindStringSubmatch("Primary: alice@acme.com (backup: bob@example.org)")
	fmt.Printf("FindStringSubmatch -> %v\n", sub)
	fmt.Printf("  user: %q  host: %q\n", sub[1], sub[2])
}

// FindSubmatch: the []byte variant.
func submatchBytes(pattern string) {
	re := regexp.MustCompile(pattern)
	raw := []byte("Primary: alice@acme.com (backup: bob@example.org)")
	fmt.Printf("FindSubmatch -> %q\n", re.FindSubmatch(raw))
}

// Optional group that didn't participate comes back empty.
func optionalGroup(pattern string) {
	re := regexp.MustCompile(pattern)
	optional := re.FindStringSubmatch("user@example")
	fmt.Printf("Optional group absent -> %q (index 2 is empty)\n", optional)
}
