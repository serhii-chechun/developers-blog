package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
'.' matches one char: "hat"
[a-z0-9]+ on 'abc 123 !!!': "abc"
[^0-9]+ on 'abc123': "abc"
\w+\s+\d+ on 'user 42': "user 42"
\S+ on '  word  ': "word"
^\w+$ on 'hello': true
^\w+$ on 'hello world': false
greedy <.+>  -> "<a><b>"
lazy   <.+?> -> "<a>"
\d{3,4} on 'call 555-1234': "555"
groups -> [alice@acme alice acme]
named group year -> "2026"
(?:ab)+ on 'xababy': "abab"
(?i)hello on 'HELLO': true
(?m)^error on 'ok\nerror': true
(?s)a.b on 'a\nb': true
(?U)<.+> on '<a><b>': "<a>"
*/

func main() {
	// h.t - the single-character wildcard "."
	//
	// h - literal "h"
	// . - any character except newline
	// t - literal "t"
	singleChar(`h.t`)

	// [a-z0-9]+ - a character class run
	//
	// [a-z0-9] - any lowercase letter or digit
	// +        - one or more of the preceding
	characterClass(`[a-z0-9]+`)

	// [^0-9]+ - a negated character class (non-digits)
	//
	// [^0-9] - any character that is NOT a digit
	// +      - one or more
	negatedClass(`[^0-9]+`)

	// \w+\s+\d+ - Perl classes: word, whitespace, digit
	//
	// \w+ - one or more word chars
	// \s+ - one or more whitespace chars
	// \d+ - one or more digits
	perlClasses(`\w+\s+\d+`)

	// \S+ - a negated Perl class (non-whitespace)
	negatedPerl(`\S+`)

	// ^\w+$ - anchors: the whole string must be one word
	//
	// ^  - start-of-string anchor
	// \w+ - one or more word chars
	// $  - end-of-string anchor
	anchors(`^\w+$`)

	// <.+> - a greedy quantifier (contrasted with lazy <.+?>)
	//
	// <  - literal "<"
	// .+ - one or more chars, greedy
	// >  - literal ">"
	greedyVsLazy(`<.+>`)

	// \d{3,4} - an exact quantifier range
	//
	// \d    - a digit
	// {3,4} - exactly 3 to 4 of the preceding
	exactQuantifiers(`\d{3,4}`)

	// (\w+)@(\w+) - capturing groups
	//
	// (\w+) - group 1: word chars (capturing)
	// @     - literal "@"
	// (\w+) - group 2: word chars (capturing)
	capturingGroups(`(\w+)@(\w+)`)

	// (?P<year>\d{4}) - a named capture
	//
	// (?P<year>...) - capture group named "year"
	// \d{4}         - exactly 4 digits
	namedCapture(`(?P<year>\d{4})`)

	// (?:ab)+ - a non-capturing group
	//
	// (?:ab) - groups "ab" WITHOUT capturing
	// +      - one or more
	nonCapturing(`(?:ab)+`)

	// (?i)hello - an inline flag (case-insensitive)
	//
	// (?i)  - inline flag: case-insensitive matching
	// hello - literal text "hello"
	inlineFlags(`(?i)hello`)
}

// Single character "." matches any character except newline.
func singleChar(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("'.' matches one char: %q\n", re.FindString("hat hot"))
}

// Character class "[a-z0-9]" matches one char in the set.
func characterClass(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("[a-z0-9]+ on 'abc 123 !!!': %q\n", re.FindString("abc 123 !!!"))
}

// Negated class "[^0-9]" matches any char NOT in the set.
func negatedClass(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("[^0-9]+ on 'abc123': %q\n", re.FindString("abc123"))
}

// Perl classes \d (digits), \w (word), \s (whitespace).
func perlClasses(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("\\w+\\s+\\d+ on 'user 42': %q\n", re.FindString("user 42"))
}

// Negated Perl classes \D \W \S (non-digit, non-word, non-space).
func negatedPerl(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("\\S+ on '  word  ': %q\n", re.FindString("  word  "))
}

// Anchors ^ (start), $ (end), \b (word boundary).
func anchors(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("^\\w+$ on 'hello': %v\n", re.MatchString("hello"))
	fmt.Printf("^\\w+$ on 'hello world': %v\n", re.MatchString("hello world"))
}

// Greedy vs lazy quantifiers.
func greedyVsLazy(pattern string) {
	greedy := regexp.MustCompile(pattern)
	lazy := regexp.MustCompile(`<.+?>`)
	fmt.Printf("greedy <.+>  -> %q\n", greedy.FindString("<a><b>"))
	fmt.Printf("lazy   <.+?> -> %q\n", lazy.FindString("<a><b>"))
}

// Exact quantifiers {n} {n,m} {n,}.
func exactQuantifiers(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("\\d{3,4} on 'call 555-1234': %q\n", re.FindString("call 555-1234"))
}

// Capturing groups "(re)" capture their text.
func capturingGroups(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("groups -> %v\n", re.FindStringSubmatch("alice@acme"))
}

// Named captures "(?P<name>re)" assign a name to a group.
func namedCapture(pattern string) {
	re := regexp.MustCompile(pattern)
	idx := re.FindStringSubmatchIndex("year 2026")
	fmt.Printf("named group year -> %q\n", "year 2026"[idx[2]:idx[3]])
}

// Non-capturing group "(?:re)" groups without capturing.
func nonCapturing(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("(?:ab)+ on 'xababy': %q\n", re.FindString("xababy"))
}

// Inline flags: (?i) (?m) (?s) (?U).
func inlineFlags(pattern string) {
	caseInsensitive := regexp.MustCompile(pattern)
	multiline := regexp.MustCompile(`(?m)^error`)
	dotAll := regexp.MustCompile(`(?s)a.b`)
	ungreedy := regexp.MustCompile(`(?U)<.+>`)

	fmt.Printf("(?i)hello on 'HELLO': %v\n", caseInsensitive.MatchString("HELLO"))
	fmt.Printf("(?m)^error on 'ok\\nerror': %v\n", multiline.MatchString("ok\nerror"))
	fmt.Printf("(?s)a.b on 'a\\nb': %v\n", dotAll.MatchString("a\nb"))
	fmt.Printf("(?U)<.+> on '<a><b>': %q\n", ungreedy.FindString("<a><b>"))
}
