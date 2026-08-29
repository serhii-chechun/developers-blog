package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
Quoted: "cost: \\$99\\.99 \\(before 2026\\)"
Matches the literal phrase: true
Does NOT match a similar string: false
Unquoted match on same text: false
*/

func main() {
	// "cost: $99.99 (before 2026)" - a user phrase full of regex metacharacters, matched literally
	//
	// $ ( ) . - metacharacters that QuoteMeta escapes so the phrase matches literally
	quoteMeta(`cost: $99.99 (before 2026)`)
}

// quoteMeta escapes metacharacters so a phrase is matched literally.
func quoteMeta(phrase string) {
	escaped := regexp.QuoteMeta(phrase)
	fmt.Printf("Quoted: %q\n", escaped)

	re := regexp.MustCompile(escaped)
	fmt.Printf("Matches the literal phrase: %v\n", re.MatchString("Price is cost: $99.99 (before 2026), ok?"))
	fmt.Printf("Does NOT match a similar string: %v\n", re.MatchString("cost: 99.99 before 2026"))

	// Without QuoteMeta, "." and "$" act as regex syntax.
	raw := regexp.MustCompile(phrase)
	fmt.Printf("Unquoted match on same text: %v\n", raw.MatchString("Price is cost: $99.99 (before 2026), ok?"))
}
