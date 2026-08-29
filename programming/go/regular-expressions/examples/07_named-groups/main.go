package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
SubexpNames: [ intl local short]
"Call +1 415 555 0100 now"       -> "intl"
"Reach us at (212) 555-0199"     -> "local"
"Text 48291 to opt out"          -> "short"
"No contact info here"           -> ""
*/

func main() {
	// (?P<intl>...)|(?P<local>...)|(?P<short>...) - an international phone, a local phone, or a 5-6 digit short code
	//
	// branch 1, named "intl": (?P<intl>\+[1-9]\d{0,2}(?:[ .-]?\d{1,4})+) - international number
	//   (?P<intl>...)      - capture group named "intl"
	//   \+                 - a literal plus sign
	//   [1-9]              - first digit 1-9 (country codes never start with 0)
	//   \d{0,2}            - zero to two more digits (the rest of the country code)
	//   (?:[ .-]?\d{1,4})+ - one or more groups: an optional separator then 1-4 digits
	//   [ .-]?             - optional separator: space, dot, or hyphen
	//   \d{1,4}            - one to four digits (part of the subscriber number)
	//
	// branch 2, named "local": (?P<local>\b\(?\d{3}\)?[-. ]\d{3}[-. ]\d{4}\b) - local number
	//   \b    - word boundary (start)
	//   \(?   - an optional opening parenthesis
	//   \d{3} - the area code (3 digits)
	//   \)?   - an optional closing parenthesis
	//   [-. ] - a separator: hyphen, dot, or space
	//   \d{3} - the exchange (3 digits)
	//   [-. ] - a separator
	//   \d{4} - the subscriber number (4 digits)
	//   \b    - word boundary (end)
	//
	// branch 3, named "short": (?P<short>\b\d{5,6}\b) - a short code
	//   \b      - word boundary (start)
	//   \d{5,6} - five or six digits
	//   \b      - word boundary (end)
	classify(`(?P<intl>\+[1-9]\d{0,2}(?:[ .-]?\d{1,4})+)|(?P<local>\b\(?\d{3}\)?[-. ]\d{3}[-. ]\d{4}\b)|(?P<short>\b\d{5,6}\b)`)
}

// classify prints SubexpNames and which named group each sample matches.
func classify(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("SubexpNames: %v\n", re.SubexpNames())
	samples := []string{
		"Call +1 415 555 0100 now",
		"Reach us at (212) 555-0199",
		"Text 48291 to opt out",
		"No contact info here",
	}
	for _, s := range samples {
		fmt.Printf("%-32q -> %q\n", s, signal(re, s))
	}
}

// signal returns the name of the named group that matched, or "" if none.
func signal(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" && m[i] != "" {
			return name
		}
	}
	return ""
}
