package main

import (
	"fmt"
	"regexp"
)

/*
# expected output:
FindStringSubmatchIndex -> [0 5 0 3 4 5]
  full="cpu=4" key="cpu" val="4"
FindAllStringSubmatchIndex -> [[0 5 0 3 4 5] [6 14 6 9 10 14] [15 23 15 19 20 23]]
  key="cpu" val="4" (bytes [0 5 0 3 4 5])
  key="mem" val="8192" (bytes [6 14 6 9 10 14])
  key="disk" val="512" (bytes [15 23 15 19 20 23])
*/

func main() {
	// (\w+)=(\d+) - key=value: word characters = digits
	// (\w+) - group 1: the key (one or more word chars)
	// =     - literal "="
	// (\d+) - group 2: the value (one or more digits)
	submatchIndex(`(\w+)=(\d+)`)

	// same key=value pattern across many rows
	findAllSubmatchIndex(`(\w+)=(\d+)`)
}

// FindStringSubmatchIndex: a flat [start, end] list ([full][group1][group2]...).
func submatchIndex(pattern string) {
	re := regexp.MustCompile(pattern)
	s := "cpu=4 mem=8192"
	idx := re.FindStringSubmatchIndex(s)
	fmt.Printf("FindStringSubmatchIndex -> %v\n", idx)
	fmt.Printf("  full=%q key=%q val=%q\n", s[idx[0]:idx[1]], s[idx[2]:idx[3]], s[idx[4]:idx[5]])
}

// FindAllStringSubmatchIndex: one flat list per match.
func findAllSubmatchIndex(pattern string) {
	re := regexp.MustCompile(pattern)
	rows := "cpu=4 mem=8192 disk=512"
	all := re.FindAllStringSubmatchIndex(rows, -1)
	fmt.Printf("FindAllStringSubmatchIndex -> %v\n", all)
	for _, m := range all {
		fmt.Printf("  key=%q val=%q (bytes %v)\n", rows[m[2]:m[3]], rows[m[4]:m[5]], m)
	}
}
