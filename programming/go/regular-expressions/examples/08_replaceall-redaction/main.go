package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
Redacted card: User alice@acme.com paid with card **** **** **** 1111 on 2026-08-29
Masked email : User ***@***.*** paid with card **** **** **** 1111 on 2026-08-29
ReplaceAll([]byte): card **** charged
*/

func main() {
	// \b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?(\d{4})\b - a 16-digit card, capturing the last 4 digits
	//
	// \b                   - word boundary (start)
	// \d{4}                - four digits
	// [ -]?                - optional separator (space or hyphen)
	// \d{4}[ -]?\d{4}[ -]? - two more 4-digit groups, each with an optional separator
	// (\d{4})              - captured group: the last 4 digits
	// \b                   - word boundary (end)

	redactCard(`\b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?(\d{4})\b`)
	// [a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,} - an email address
	//
	//   [a-zA-Z0-9._%+-]+ - one or more local-part chars
	//   @                 - literal "@"
	//   [a-zA-Z0-9.-]+    - one or more domain chars
	//   \.                - a literal "."
	//   [a-zA-Z]{2,}      - two or more letters (the TLD)

	maskEmails(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	// \b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?\d{4}\b - a 16-digit card (no capture group)
	//
	// \b                    - word boundary
	// \d{4}                 - four digits
	// [ -]?                 - optional separator
	// (repeated four times) - four 4-digit groups with separators
	replaceAllBytes(`\b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?\d{4}\b`)
}

// ReplaceAllString expands $1; capture the last 4 digits to keep them.
func redactCard(pattern string) {
	re := regexp.MustCompile(pattern)
	log := "User alice@acme.com paid with card 4111 1111 1111 1111 on 2026-08-29"
	fmt.Printf("Redacted card: %s\n", re.ReplaceAllString(log, "**** **** **** $1"))
}

// ReplaceAllString with a fixed masked replacement.
func maskEmails(pattern string) {
	re := regexp.MustCompile(pattern)
	log := "User alice@acme.com paid with card **** **** **** 1111 on 2026-08-29"
	fmt.Printf("Masked email : %s\n", re.ReplaceAllString(log, "***@***.***"))
}

// ReplaceAll: the []byte variant for log bytes.
func replaceAllBytes(pattern string) {
	re := regexp.MustCompile(pattern)
	raw := []byte("card 5500 0000 0000 0004 charged")
	fmt.Printf("ReplaceAll([]byte): %s\n", re.ReplaceAll(raw, []byte("****")))
}
