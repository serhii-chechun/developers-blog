package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
All hashtags: [#1042 #billing #followup]
First 2 emails: [john@doe.co.uk alice@acme.com]
All hashtags as [][]byte: ["#1042" "#billing" "#followup"]
*/

func main() {
	// #\w+ - a hashtag (literal # followed by word characters)
	//
	// #   - literal "#"
	// \w+ - one or more word characters [0-9A-Za-z_]
	findAllHashtags(`#\w+`)

	// [a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,} - an email address
	//
	// [a-zA-Z0-9._%+-] - one char: letters, digits, . _ % + -
	// +                - one or more of the preceding
	// @                - literal "@"
	// [a-zA-Z0-9.-]    - one char: letters, digits, dot, hyphen
	// +                - one or more of the preceding
	// \.               - a literal "." (escaped)
	// [a-zA-Z]{2,}     - two or more letters (the TLD)
	// [a-zA-Z]         - one letter
	// {2,}             - two or more of the preceding
	findAllEmails(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	// #\w+ - hashtags extracted from []byte data
	findAllHashtagsBytes(`#\w+`)
}

// FindAllString(s, -1): returns every match.
func findAllHashtags(pattern string) {
	re := regexp.MustCompile(pattern)
	ticket := `Customer #1042 reports #billing issue, please CC alice@acme.com and bob@example.org for #followup.`
	fmt.Printf("All hashtags: %v\n", re.FindAllString(ticket, -1))
}

// FindAllString(s, n): n limits the number of matches.
func findAllEmails(pattern string) {
	re := regexp.MustCompile(pattern)
	ticket := `Customer #1042 reports #billing issue, please CC john@doe.co.uk alice@acme.com and bob@example.org for #followup.`
	fmt.Printf("First 2 emails: %v\n", re.FindAllString(ticket, 2))
}

// FindAll ([]byte): the byte variant for data already in bytes.
func findAllHashtagsBytes(pattern string) {
	re := regexp.MustCompile(pattern)
	ticket := []byte(`Customer #1042 reports #billing issue, please CC alice@acme.com and bob@example.org for #followup.`)
	fmt.Printf("All hashtags as [][]byte: %q\n", re.FindAll(ticket, -1))
}
