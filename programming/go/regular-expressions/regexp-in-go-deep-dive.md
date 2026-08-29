# Regular Expressions in Go: A Deep Dive

Go's `regexp` package is one of the most misunderstood pieces of the standard library. Coming from Python, JavaScript, or Ruby, the first thing most developers notice is what's *missing*: no `\1` backreferences, no lookahead, no lookbehind. The second thing they notice is the scary wall of `Find`, `FindAll`, `FindString`, `FindStringSubmatch` methods.

These surprises are not accidental. Both are the direct result of a deliberate design decision that prioritizes **safety and predictable performance over expressiveness**. Once one understands *why* Go's regex engine works the way it does, the API stops feeling limited and starts feeling rational.

This post covers the history behind the engine, how it compares to what other languages use, and how to actually read and use the full API.

---

## Table of Contents

1. [The History and Philosophy of Go's Regexp](#1-the-history-and-philosophy-of-gos-regexp)
2. [Engine Overview: RE2 vs. The World](#2-engine-overview-re2-vs-the-world)
3. [The Full API: The Method Matrix](#3-the-full-api-the-method-matrix)
4. [Grammar Review: What Go Supports](#4-grammar-review-what-go-supports)
5. [Putting It Together: Scam SMS Detection at Scale](#5-putting-it-together-scam-sms-detection-at-scale)

---

## 1. The History and Philosophy of Go's Regexp

To understand Go's regex engine, we have to understand its lineage - it traces back more than half a century to one of the foundational figures in computer science and software engineering.

### The Roots: Ken Thompson

In the 1960s, [Ken Thompson](https://en.wikipedia.org/wiki/Ken_Thompson) - long before he co-created Go - was working on the [QED text editor](https://www.nokia.com/bell-labs/about/dennis-m-ritchie/qed.html) . He wrote the first regular expression compiler for QED, and in doing so introduced what's now known as the [Thompson NFA](https://en.wikipedia.org/wiki/Thompson%27s_construction) (Nondeterministic Finite Automaton) construction algorithm.

The insight was elegant: a regular expression can be mechanically translated into a state machine. That state machine can then scan a string **character by character, in a single forward pass**, and decide whether the string matches.

This is the crucial property. It means the amount of work grows *linearly* with the input, and it's the foundation everything else in this post is built on.

### The Author: Russ Cox

The Go `regexp` package is a **reimplementation of RE2, written in Go** - not the [original C++ library](https://github.com/google/re2), but a faithful port of its design and behavior. RE2 itself was authored by [Russ Cox](https://swtch.com/~rsc/), who is, at the time of this writing, the technical lead of the Go project.

For a deeper dive, his article series, [Regular Expression Matching Can Be Simple And Fast](https://swtch.com/~rsc/regexp/regexp1.html), is the definitive reference.

### Core Design Decision: Safety Over Features

The defining choice in Go's regex engine is a conscious trade-off:

> **Guarantee linear-time execution, and refuse to support anything that would break that guarantee.**

Concretely, the engine runs in **linear time**: for an input of length **n** and a pattern of size **m**, it finishes in **O(n * m)**. That's not a "usually fast" promise - it's a hard guarantee.

This matters enormously in the real world because of [Regular Expression Denial of Service (ReDoS)](https://owasp.org/www-community/attacks/Regular_expression_Denial_of_Service_-_ReDoS). An innocent-looking pattern paired with a carefully crafted input can force a backtracking engine to explore an exponential number of paths, hanging the process. It's a real, actively exploited vulnerability in many languages.

Go eliminates the entire class by construction. Untrusted user input can be run against any regex, at any time, without a timeout, and the engine will still finish in linear time.

### What Was Sacrificed

This safety comes at a price. Two categories of features that are common in [PCRE-style engines](https://www.pcre.org/) are **intentionally omitted**:

- **Backreferences** (`\1`, `\2`, ...) - referring to a previously captured group inside the pattern.
- **Lookaround assertions** - lookahead (`(?=...)`, `(?!...)`) and lookbehind (`(?<=...)`, `(?<!...)`).

Why? Because both of these require the ability to *remember* where you were and *backtrack* to try alternative paths. And backtracking is exactly what destroys the linear-time guarantee - it's what opens the door to **O(2^n)** worst-case behavior and ReDoS.

This is a philosophical position, not a technical oversight. In practice, most real-world regexes can be written *without* backreferences or lookaround, and for the rare cases that genuinely need them, the linear-time guarantee is worth more than the convenience.

---

## 2. Engine Overview: RE2 vs. The World

To really appreciate Go's approach, it helps to contrast it with what most other languages use under the hood.

### The Go Engine: Automaton-Based

Go compiles your regex into a **state machine** (a Thompson NFA, with DFA optimizations where beneficial). It then parses the input in a **single forward pass**, tracking the set of possible states as it consumes each character. Crucially, it **never goes backward**. Once a character is consumed, the engine never revisits it.

This single-pass property is what makes the linear-time guarantee possible. Every character is examined exactly once, and the work per character is bounded.

### The Competition: Backtracking

Most other popular languages - [Python `re`](https://docs.python.org/3/library/re.html), [Java `java.util.regex`](https://docs.oracle.com/javase/8/docs/api/java/util/regex/package-summary.html), [Ruby `Onigmo`](https://github.com/k-takata/onigmo), plus Perl and PHP - use **backtracking** engines that share PCRE-style semantics. Strictly speaking, most aren't literally PCRE - PHP is the exception, wrapping the real PCRE library, and PCRE itself is a C library built to emulate Perl - but they all follow the same family of design and syntax.

A backtracking engine works by trying one matching path. If it fails partway through, it "rewinds" to the last decision point and tries a *different* path. If the pattern has many such decision points (nested quantifiers, alternations, groups), the number of paths can explode.

For typical patterns and inputs, backtracking is fast - often faster than an automaton-based engine, which is part of why these languages keep it. But it has no worst-case guarantee: a bad pattern and input can force exponential growth in the number of paths.

### The Comparison Table

| Feature / Trait | Go (`regexp` / RE2) | Backtracking engines (PCRE-style: Python, Java, Ruby, Node.js) |
| --- | --- | --- |
| **Matching Algorithm** | Thompson NFA / DFA | Backtracking (usually NFA) |
| **Worst-case Time** | Linear **O(n)** | Exponential **O(2^n)** |
| **ReDoS Vulnerability** | No (safe for untrusted user input) | Yes (requires timeouts / safeguards) |
| **Lookahead / Lookbehind** | Not supported | **Fully supported** |
| **Backreferences (`\1`)** | Not supported | **Fully supported** |
| **Execution Predictability** | Extremely high | Can be volatile on complex strings |

The trade-off in one sentence: **backtracking engines offer more expressive power; Go's engine offers a guarantee.** Which approach to choose depends entirely on whether the regexes run against trusted, known inputs or against data the system doesn't control.

> **Gotcha:** If you're porting a regex from Python or JavaScript to Go, and it uses lookahead, lookbehind, or backreferences, it simply **will not compile**. You'll need to rewrite it - often with more verbose alternations or by doing part of the work outside the regex.

---

## 3. The Full API: The Method Matrix

The `regexp` API has dozens of methods, but they follow a single, consistent **naming matrix**: almost every method is built by combining prefixes and suffixes:

```
Find (All)? (String)? (Submatch)? (Index)?
```

The core is `Find`, with optional additions:
- `All` return *all* matches (not just the first)
- `String` operate on and return `string` instead of `[]byte`
- `Submatch` also return captured groups
- `Index` return byte offsets instead of the matched text

So `FindAllStringSubmatchIndex` is the most complete version: *find all matches, as strings, including submatches, as byte-index pairs.* Once the pattern is understood, the rest of the API becomes predictable.

The `ReplaceAll` and `Split` families are the exceptions to this matrix: `ReplaceAll`/`ReplaceAllString`, `ReplaceAllLiteral`/`ReplaceAllLiteralString`, and `ReplaceAllFunc`/`ReplaceAllStringFunc` place the `String` suffix differently (note `ReplaceAllStringFunc`, not `ReplaceAllFuncString`).

### Compilation & Utility Functions

| Function | Purpose | When to use |
| --- | --- | --- |
| `Compile(expr)` | Compiles the regex, returns an `error` if invalid. | User-provided or dynamic regex. |
| `MustCompile(expr)` | Compiles the regex, **panics** if invalid. | Hardcoded regex initialized globally. |
| `CompilePOSIX(expr)` | Compiles using strict POSIX ERE rules (leftmost-longest match). | When POSIX behavior is needed. |
| `MatchString(pattern, s)` | One-off boolean check - compiles internally. | Simple, rare checks. |
| `QuoteMeta(s)` | Escapes all regex metacharacters, returns a `string`. | Treating dynamic strings as literals. |

The key distinction: `Compile` returns an error, while `MustCompile` panics. For a hardcoded pattern in a global `var`, a panic on startup is usually the *right* behavior - a typo in a constant regex is a developer bug, not a runtime condition. That's why `MustCompile` is the idiomatic choice:

```go
var digitsRe = regexp.MustCompile(`\d{3,4}`) // panics at init if the pattern is bad
```

### The `Regexp` Object API

With a compiled `*regexp.Regexp`, here's the method surface. The `String` variants work on and return `string`; the non-`String` variants work on `[]byte`. The rest of the matrix composes on top.

| Method | Argument Type | Returns | Description |
| --- | --- | --- | --- |
| `Match(b)` / `MatchString(s)` | `[]byte` / `string` | `bool` | True if the regex matches any part of the input. |
| `Find(b)` / `FindString(s)` | `[]byte` / `string` | `[]byte` / `string` | The leftmost match. |
| `FindAll(b, n)` / `FindAllString(s, n)` | `[]byte` / `string` | `[][]byte` / `[]string` | Up to `n` matches (`-1` for all). |
| `FindSubmatch(b)` / `FindStringSubmatch(s)` | `[]byte` / `string` | `[][]byte` / `[]string` | The match plus all captured groups `( )`. |
| `FindIndex(b)` / `FindStringIndex(s)` | `[]byte` / `string` | `[]int` | `[start, end]` byte index pair (or `nil` if no match). |
| `ReplaceAll(b, repl)` / `String` | `[]byte` / `string` | `[]byte` / `string` | Replaces matches with `repl`, expanding `$1`. |
| `ReplaceAllLiteral(b, repl)` / `String` | `[]byte` / `string` | `[]byte` / `string` | Replaces matches with the literal `repl` string (no expansion). |
| `ReplaceAllFunc(b, repl)` / `ReplaceAllStringFunc(s, repl)` | `[]byte` / `string` (`repl` is a `func`) | `[]byte` / `string` | Replaces matches using a custom function. |
| `Split(s, n)` | `string` | `[]string` | Splits the string around the matches. |

Two practical notes on this API:

- **Prefer the `[]byte` methods** when working with I/O and network data. Reading a file or a socket already yields `[]byte`; converting to `string` forces an allocation. The `[]byte` methods avoid that copy entirely.
- **`SubmatchIndex` returns byte offsets.** An optional group that didn't participate in the match reports `-1` for both its start and end - which signals that it was absent.

Here's the `Submatch` semantics in action:

```go
re := regexp.MustCompile(`(?P<year>\d{4})-(?P<month>\d{2})`)
idx := re.FindStringSubmatchIndex("2026-08")
// idx = [0 7 0 4 5 7]
//
// Pair     meaning     value     input slice
// [0, 7]   full match  "2026-08" s[0:7]
// [0, 4]   year group  "2026"    s[0:4]
// [5, 7]   month group "08"      s[5:7]
//
// A group that didn't participate reports -1, -1 for its pair:
re2 := regexp.MustCompile(`(?P<year>\d{4})(?:-(?P<month>\d{2}))?`)
idx2 := re2.FindStringSubmatchIndex("2026")
// idx2 = [0 4 0 4 -1 -1]  <- month didn't participate
```

### The `SubmatchIndex` Contract

When using the `Index` variants, the returned slice is a flat list of `[start, end]` pairs: element `0:1` is the whole match, `2:3` is the first group, `4:5` the second, and so on. This is incredibly useful for slicing out substrings efficiently, or when the *positions* of the matches matter (for highlighting, parsing, or logging) rather than the text.

For example, slicing each pair directly out of the input:

```go
re := regexp.MustCompile(`(\w+)@(\w+)`)
s := "Contact: alice@example"
idx := re.FindStringSubmatchIndex(s) // idx = [9 22 9 14 15 22]

full := s[idx[0]:idx[1]] // s[9:22]  -> "alice@example"
user := s[idx[2]:idx[3]] // s[9:14]  -> "alice"
host := s[idx[4]:idx[5]] // s[15:22] -> "example"
```

---

## 4. Grammar Review: What Go Supports

Now let's go over the syntax Go supports.

> **Syntax note:** Go's regexp uses **RE2 syntax**, so PCRE-isms like `\1` backreferences, lookaround, and atomic/possessive groups do **not** compile. Anything that relies on them must be rewritten.

| Concept | Syntax | Description | Example |
| --- | --- | --- | --- |
| **Single Characters** | `.` | Any character (except newline by default) | `a.c` matches `abc` |
| **Character Classes** | `[a-z0-9]` | Any character in the set | `[A-Z]` matches capital letters |
| **Negated Classes** | `[^a-z]` | Any character *not* in the set | `[^0-9]` matches non-digits |
| **Perl Classes** | `\d`, `\w`, `\s` | Digits, word characters, whitespace | `\w+` matches a word |
| **Negated Perl Classes** | `\D`, `\W`, `\S` | Non-digits, non-word, non-whitespace | `\S+` matches visible text |
| **Anchors** | `^`, `$`, `\b` | Start of line, end of line, word boundary (ASCII-only) | `^\w+$` matches exactly one word |
| **Greedy Quantifiers** | `*`, `+`, `?` | 0+, 1+, or 0–1 of the preceding | `a*` matches ``, `a`, `aa` |
| **Lazy Quantifiers** | `*?`, `+?`, `??` | Same, but match the *minimum* | `<.+?>` matches an HTML tag |
| **Exact Quantifiers** | `{n}`, `{n,m}`, `{n,}` | Exactly `n`, between `n` and `m`, or `n`+ | `\d{3,4}` matches 3 or 4 digits |
| **Capturing Groups** | `(re)` | Groups the regex and captures the text | `(abc)+` matches `abcabc` |
| **Named Captures** | `(?P<name>re)` / `(?<name>re)` | Captures and assigns a name | `(?P<year>\d{4})` |
| **Non-Capturing Group** | `(?:re)` | Groups without capturing | `(?:abc)+` |
| **Inline Flags** | `(?i)`, `(?m)`, `(?s)`, `(?U)` | Case-insensitive, multi-line, dot-matches-all, ungreedy | `(?i)hello` matches `HELLO` |

A few details worth calling out:

- **Greedy vs. lazy.** By default quantifiers are greedy - they consume as much as possible while still allowing the overall match. Appending `?` makes them lazy - they consume as little as possible. The classic example: `<.+>` against `<a><b>` greedily matches the whole string, while `<.+?>` matches just `<a>`.
- **Named captures.** Go supports both the Python-style `(?P<name>re)` and the `.NET`/`(?<name>re)` spelling. They can be referenced by name in replacements with `${name}`.
- **The ungreedy flag `(?U)`.** This flips the default so that quantifiers are lazy unless explicitly made greedy with `?`. It's the same idea as Python's `(?U)` and is easy to forget.

---

## 5. Putting It Together: Scam SMS Detection at Scale

Let's pull everything together with a realistic task: a stream of incoming SMS messages that we want to flag as potential scams. A common signal is the presence of contact details - an international phone number, a local number, or a short code like `48291`.

Start with the individual patterns (each is a separate signal a detection system might care about):

```go
var (
    phoneIntlRe  = regexp.MustCompile(`\+[1-9]\d{0,2}(?:[ .-]?\(?[0-9]{1,4}\)?)?(?:[ .-]?[0-9]{1,4}){2,4}`)
    phoneLocalRe = regexp.MustCompile(`\b\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}\b`)
    shortCodeRe  = regexp.MustCompile(`\b\d{5,6}\b`)
)
```

Now the key design decision. Instead of running these three separately - three full scans per message, or three goroutines per message - combine them into a single alternation. RE2's automaton tracks **all branches simultaneously in one forward pass**:

```go
var scamRe = regexp.MustCompile(
    `\+[1-9]\d{0,2}(?:[ .-]?\(?[0-9]{1,4}\)?)?(?:[ .-]?[0-9]{1,4}){2,4}` +
    `|\b\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}\b` +
    `|\b\d{5,6}\b`,
)
```

To also know *which* signal fired, wrap each branch in a named group and use `SubexpNames()` together with `FindStringSubmatch` - combining the method-matrix pieces we covered earlier:

```go
var scamRe = regexp.MustCompile(
    `(?P<intl>\+[1-9]\d{0,2}(?:[ .-]?\(?[0-9]{1,4}\)?)?(?:[ .-]?[0-9]{1,4}){2,4})` +
    `|(?P<local>\b\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}\b)` +
    `|(?P<short>\b\d{5,6}\b)`,
)

func signal(msg string) string {
    m := scamRe.FindStringSubmatch(msg)
    if m == nil {
        return ""
    }
    for i, name := range scamRe.SubexpNames() {
        if i != 0 && name != "" && m[i] != "" {
            return name // "intl", "local", or "short"
        }
    }
    return ""
}
```

Now for throughput at scale. Because `*regexp.Regexp` is safe for concurrent use, parallelize **across the message stream** - one worker per CPU core, each running the same combined regex. This scales linearly with cores, without the overhead of spawning a goroutine per pattern per message:

```go
func worker(msgs <-chan string, flagged chan<- string) {
    for msg := range msgs {
        if scamRe.MatchString(msg) {
            flagged <- msg
        }
    }
}

func main() {
    workers := runtime.GOMAXPROCS(0)
    msgs := make(chan string)
    flagged := make(chan string)

    for i := 0; i < workers; i++ {
        go worker(msgs, flagged)
    }

    for _, msg := range incomingMessages() {
        msgs <- msg
    }
    close(msgs)
    // consume `flagged` concurrently...
}
```

Why not just run the three regexes concurrently? Benchmarking both approaches on a synthetic corpus showed the combined regex is just as fast - but with **zero allocations** instead of thousands, and at most one goroutine per CPU core instead of three per message. It also does less total work: one pass per message instead of three.

And because the engine is linear-time, this whole pipeline is safe to run against **untrusted SMS content** - no timeout needed, no ReDoS risk. That's the payoff of the design decisions from section 1.

---

## Conclusion

Go's `regexp` package isn't a weaker version of what Python or JavaScript offer - it's a *different trade-off*, and one that's entirely deliberate. Its linear-time guarantee, rooted in Ken Thompson's automaton work and brought to maturity by Russ Cox's RE2, means untrusted input can be run against any pattern without fear of ReDoS.

The cost is losing backreferences and lookaround. In exchange, the engine's performance is predictable and safe, and the API - once the `Find(All)?(String)?(Submatch)?(Index)?` matrix is learned - is completely systematic.

The next time a Go regex refuses to compile a lookahead, remember: it's not a bug. It's the price of a promise.
