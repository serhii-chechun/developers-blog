package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
Find([]byte) returned: "192.168.1.55"
FindString(string) returned: "192.168.1.55"
FindString returns only the leftmost: "10.0.0.1"
FindString returned empty string when nothing matched
*/

func main() {
	ipv4pattern := `\b\d{1,3}(?:\.\d{1,3}){3}\b`
	// \b\d{1,3}(?:\.\d{1,3}){3}\b - an IPv4 address
	//
	// \b               - word boundary (start)
	// \d{1,3}          - one to three digits
	// (?:\.\d{1,3}){3} - three repeats of a literal ("." then 1-3 digits)
	// \b               - word boundary (end)
	findBytes(ipv4pattern)

	// same IPv4 pattern via FindString on string input
	findString(ipv4pattern)

	// only the leftmost IPv4 match is returned
	findStringLeftmost(ipv4pattern)

	// no IPv4 match at all
	findStringNoMatch(ipv4pattern)
}

// Find ([]byte): returns the leftmost match without a string conversion.
func findBytes(pattern string) {
	re := regexp.MustCompile(pattern)
	line := []byte(`192.168.1.55 - - [29/Aug/2026:10:15:32 +0000] "GET /api/users HTTP/1.1" 200 512`)
	fmt.Printf("Find([]byte) returned: %q\n", re.Find(line))
}

// FindString (string): convenience form for string input.
func findString(pattern string) {
	re := regexp.MustCompile(pattern)
	line := `192.168.1.55 - - [29/Aug/2026:10:15:32 +0000] "GET /api/users HTTP/1.1" 200 512`
	fmt.Printf("FindString(string) returned: %q\n", re.FindString(line))
}

// FindString returns only the first (leftmost) match.
func findStringLeftmost(pattern string) {
	re := regexp.MustCompile(pattern)
	multi := `host1=10.0.0.1 host2=10.0.0.2 host3=10.0.0.3`
	fmt.Printf("FindString returns only the leftmost: %q\n", re.FindString(multi))
}

// FindString returns an empty string when there is no match.
func findStringNoMatch(pattern string) {
	re := regexp.MustCompile(pattern)
	if re.FindString("no addresses here") == "" {
		fmt.Println("FindString returned empty string when nothing matched")
	}
}
