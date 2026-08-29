package main

import (
	"fmt"
	"regexp"
	"strings"
)

/*
# expected output:
ReplaceAllStringFunc (uppercase): GO IS FAST
slugify("Go Regular Expressions: A Deep Dive") -> "go-regular-expressions-a-deep-dive"
slugify("Understanding RE2 Engines (2026)") -> "understanding-re2-engines-2026"
*/

func main() {
	// [A-Za-z]+ - one or more ASCII letters
	uppercaseEach(`[A-Za-z]+`)

	// [A-Za-z0-9]+ - one or more ASCII letters or digits
	slugify(`[A-Za-z0-9]+`)
}

// ReplaceAllStringFunc: each match is replaced by the callback result.
func uppercaseEach(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("ReplaceAllStringFunc (uppercase): %s\n", re.ReplaceAllStringFunc("go is fast", strings.ToUpper))
}

// slugify collects the words and joins them with hyphens.
func slugify(pattern string) {
	re := regexp.MustCompile(pattern)
	for _, title := range []string{
		"Go Regular Expressions: A Deep Dive",
		"Understanding RE2 Engines (2026)",
	} {
		words := re.FindAllString(title, -1)
		fmt.Printf("slugify(%q) -> %q\n", title, strings.ToLower(strings.Join(words, "-")))
	}
}
