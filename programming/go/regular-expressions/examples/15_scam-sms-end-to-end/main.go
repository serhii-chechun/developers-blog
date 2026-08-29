package main

import (
	"fmt"
	"regexp"
	"runtime"
	"sync"
)

/*
# expected output:
Per-message signal:
  "URGENT: your parcel +1 800 555 0199"      -> intl
  "Reminder: call (212) 555-0199 today"      -> local
  "Reply 48291 to unsubscribe"               -> short
  "Hey, are we still on for lunch?"          -> (clean)

Parallel flagging (8 workers) flagged 3 message(s):
  "URGENT: your parcel +1 800 555 0199"
  "Reminder: call (212) 555-0199 today"
  "Reply 48291 to unsubscribe"
*/

func main() {
	// (?P<intl>...)|(?P<local>...)|(?P<short>...) - an international phone, a local phone, or a 5-6 digit short code
	//
	// (?P<intl>\+[1-9]\d{0,2}(?:[ .-]?\(?[0-9]{1,4}\)?)?(?:[ .-]?[0-9]{1,4}){2,4}) - "intl": "+", country code, then grouped digits
	// |(?P<local>\b\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}\b)                   - "local": optional area code + local number
	// |(?P<short>\b\d{5,6}\b)                                                      - "short": a 5-6 digit code
	const pattern = `(?P<intl>\+[1-9]\d{0,2}(?:[ .-]?\(?[0-9]{1,4}\)?)?(?:[ .-]?[0-9]{1,4}){2,4})` +
		`|(?P<local>\b\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}\b)` +
		`|(?P<short>\b\d{5,6}\b)`

	perMessage(pattern)
	parallelFlag(pattern)
}

// perMessage prints which signal each incoming message triggers.
func perMessage(pattern string) {
	re := regexp.MustCompile(pattern)
	incoming := []string{
		"URGENT: your parcel +1 800 555 0199",
		"Reminder: call (212) 555-0199 today",
		"Reply 48291 to unsubscribe",
		"Hey, are we still on for lunch?",
	}
	fmt.Println("Per-message signal:")
	for _, msg := range incoming {
		if s := signal(re, msg); s != "" {
			fmt.Printf("  %-42q -> %s\n", msg, s)
		} else {
			fmt.Printf("  %-42q -> (clean)\n", msg)
		}
	}
}

// parallelFlag flags messages across one worker per CPU core.
func parallelFlag(pattern string) {
	re := regexp.MustCompile(pattern)
	incoming := []string{
		"URGENT: your parcel +1 800 555 0199",
		"Reminder: call (212) 555-0199 today",
		"Reply 48291 to unsubscribe",
		"Hey, are we still on for lunch?",
	}

	msgs := make(chan string)
	flagged := make(chan string, len(incoming))

	workers := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for m := range msgs {
				if re.MatchString(m) {
					flagged <- m
				}
			}
		})
	}

	for _, m := range incoming {
		msgs <- m
	}
	close(msgs)
	wg.Wait()
	close(flagged)

	fmt.Printf("\nParallel flagging (%d workers) flagged %d message(s):\n", workers, len(flagged))
	for m := range flagged {
		fmt.Printf("  %q\n", m)
	}
}

// signal returns the name of the matched signal, or "" if none.
func signal(re *regexp.Regexp, msg string) string {
	m := re.FindStringSubmatch(msg)
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
