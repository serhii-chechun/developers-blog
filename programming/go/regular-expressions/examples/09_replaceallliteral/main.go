package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
ReplaceAllString        (interprets $1): "the [red] box"
ReplaceAllLiteralString (literal $1):    "the [$1] box"
ANSI-stripped log: "ERROR: disk full"
*/

func main() {
	// (red) - a capturing group around the literal "red"
	literalVsExpansion(`(red)`)

	// \x1b\[[0-9;]*m - an ANSI color escape sequence
	// \x1b           - the ESC control character (0x1b)
	// \[             - a literal "["
	// [0-9;]*.       - zero or more digits or semicolons (the SGR parameters)
	// m              - the terminating letter "m"
	stripAnsi(`\x1b\[[0-9;]*m`)
}

// ReplaceAllString expands $1; ReplaceAllLiteralString keeps it literal.
func literalVsExpansion(pattern string) {
	re := regexp.MustCompile(pattern)
	fmt.Printf("ReplaceAllString        (interprets $1): %q\n", re.ReplaceAllString("the red box", "[$1]"))
	fmt.Printf("ReplaceAllLiteralString (literal $1):    %q\n", re.ReplaceAllLiteralString("the red box", "[$1]"))
}

// Strip ANSI codes with a literal (empty) replacement.
func stripAnsi(pattern string) {
	re := regexp.MustCompile(pattern)
	logWithAnsi := "\x1b[31mERROR\x1b[0m: disk \x1b[1mfull\x1b[0m"
	fmt.Printf("ANSI-stripped log: %q\n", re.ReplaceAllLiteralString(logWithAnsi, ""))
}
