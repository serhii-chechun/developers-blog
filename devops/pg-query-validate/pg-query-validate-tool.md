# PostgreSQL Query Validation: A Practical Guide to pgqv

There is a specific moment in a deployment when a schema typo stops being free. The migration has started, the tool has opened a transaction against production, and PostgreSQL reports that `type "varchat" does not exist`. The fix is one character. The cost is a failed release, a half-applied migration, and whatever the rollback procedure happens to be on a Friday afternoon.

`pgqv` is a small command-line tool that exists to move that discovery earlier. It reads a `.sql` file, checks it against the PostgreSQL grammar, and reports anything suspicious with the file, the line, the column and a caret pointing at the exact token - in a few milliseconds, with no database anywhere in sight.

This guide covers what it checks, how to install it on each platform, and how to use it in practice: from a one-off check of a schema file to a pre-commit hook that stops the typo before it is even committed.

> **The Golden Rule:** validate before something else does. A typo costs two seconds in a pre-commit hook and a failed release in a migration. The tool that catches it should run early, offline, and fast enough that nobody is tempted to skip it.

---

## Table of Contents

1. [What pgqv Checks (and What It Does Not)](#1-what-pgqv-checks-and-what-it-does-not)
2. [Installing pgqv](#2-installing-pgqv)
3. [Your First Validation](#3-your-first-validation)
4. [Reading the Output](#4-reading-the-output)
5. [Real Examples](#5-real-examples)
6. [Using It in a Workflow](#6-using-it-in-a-workflow)
7. [Troubleshooting](#7-troubleshooting)
8. [Limitations](#8-limitations)
9. [Conclusion](#9-conclusion)
10. [References & Examples](#10-references--examples)

---

## 1. What pgqv Checks (and What It Does Not)

Two checks, run over a single `.sql` file:

| Check | Example it catches |
| --- | --- |
| **Syntax** - the file must parse as PostgreSQL | `select from where;` |
| **Type-name typos** - an unqualified type one edit from a built-in | `VARCHAT(255)`, `BOOLEN`, `TSVETCOR` |

The second one is the reason the tool exists. PostgreSQL accepts *any* identifier as a type name and only resolves it when the statement runs, so `VARCHAT(255)` is valid syntax that fails on a real server. Editing distance is what makes the check usable: flagging every unresolved type name would light up every schema that defines its own enums or domains, so `pgqv` only reports names that are one character away from a built-in type - a substitution, a transposition, an insertion or a deletion.

What it does **not** do is resolve anything against a catalog. It has no connection to a database, so table names, column names and function signatures are all outside its reach. That is the price of running offline, and it is a deliberate trade: a validator that needs a live PostgreSQL cannot run in a pre-commit hook, in CI on a pull request, or on a laptop between two `git` commands.

## 2. Installing pgqv

Four routes, in the order most people will want them. The first requires no compiler at all.

### 2.1 Homebrew (macOS and Linux)

The formula lives in the project's own repository under `Formula/`, so there is no separate `homebrew-tap` to add:

```bash
$ brew tap serhii-chechun/pg-query-validate https://github.com/serhii-chechun/pg-query-validate
$ brew trust serhii-chechun/pg-query-validate
$ brew install pgqv
```

```
==> Tapping serhii-chechun/pg-query-validate
Cloning into '.../homebrew-pg-query-validate'...
Tapped 1 formula (28 files, 94.2KB).
Trusted tap: https://github.com/serhii-chechun/pg-query-validate
==> Fetching downloads for: pgqv
✔︎ Formula pgqv (1.0.0)
==> Installing pgqv from serhii-chechun/pg-query-validate
🍺  /opt/homebrew/Cellar/pgqv/1.0.0: 4 files, 8.5MB, built in 1 second
```

The middle step is not optional, and it is the one most people will trip over. Homebrew trusts its own taps by default and refuses to load formulae from a third-party tap until it is trusted:

```
Error: Refusing to load formula serhii-chechun/pg-query-validate/pgqv from untrusted tap serhii-chechun/pg-query-validate.
Run `brew trust --formula serhii-chechun/pg-query-validate/pgqv` or `brew trust serhii-chechun/pg-query-validate` to trust it.
```

Trusting the whole tap is the sensible choice here - the tap contains exactly one formula.

The formula installs the **prebuilt binary** for the platform rather than building from source. That is deliberate: a Homebrew build has no network access, so it could not fetch the Go modules this project depends on, and it would recompile a large body of bundled C code on every install. The practical consequence is that `brew install pgqv` finishes in about a second, and no compiler is involved.

To remove it again:

```bash
$ brew uninstall pgqv
$ brew untap serhii-chechun/pg-query-validate
```

### 2.2 Prebuilt binaries

Every release attaches the archives below, each containing a single self-contained binary. They need no compiler, no PostgreSQL and no shared library - just the system C library.

| Platform | Archive |
| --- | --- |
| macOS, Apple Silicon | `pgqv_1.0.0_darwin_arm64.tar.gz` |
| macOS, Intel | `pgqv_1.0.0_darwin_amd64.tar.gz` |
| Linux, x86-64 | `pgqv_1.0.0_linux_amd64.tar.gz` |
| Linux, arm64 | `pgqv_1.0.0_linux_arm64.tar.gz` |
| Windows, x86-64 | `pgqv_1.0.0_windows_amd64.zip` |

They are attached to [the v1.0.0 release](https://github.com/serhii-chechun/pg-query-validate/releases/tag/v1.0.0). Download the one that matches the machine, or use `curl` directly - which also makes the step reproducible in a Dockerfile or a bootstrap script:

**macOS and Linux**

```bash
$ curl -LO https://github.com/serhii-chechun/pg-query-validate/releases/download/v1.0.0/pgqv_1.0.0_linux_amd64.tar.gz
$ tar -xzf pgqv_1.0.0_linux_amd64.tar.gz
$ sudo install -m 755 pgqv /usr/local/bin/pgqv
```

Swap `linux_amd64` for the archive that matches the host: `darwin_arm64` on an M-series Mac, `linux_arm64` on a Graviton box or an ARM container.

**Windows**

The Windows archive is a `.zip` containing `pgqv.exe`. Extract it and put the directory on `PATH`:

```powershell
> Expand-Archive pgqv_1.0.0_windows_amd64.zip -DestinationPath .
> .\pgqv.exe .\schema.sql
```

Confirm the install with the usage banner - running `pgqv` with no arguments prints it:

```bash
$ pgqv
```

```
PostgreSQL Query Validator v1.0 (c) 2026, Serhii Chechun
Usage: pgqv <filename.sql>
```

### 2.3 With `go install`

If a Go toolchain is already present, this is the shortest route. It requires **Go 1.27.1 or newer** and **a C compiler**, because the SQL parser is compiled from C:

```bash
$ go install github.com/serhii-chechun/pg-query-validate/cmd/pgqv@v1.0.0
```

```
go: downloading github.com/serhii-chechun/pg-query-validate v1.0.0
```

The binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`), which has to be on `PATH`. With a warm build cache this takes about five seconds; on a cold machine the first build compiles a large amount of bundled C and takes a few minutes.

> **Gotcha:** `CGO_ENABLED=0 go install ...` does not work, and the failure is worth recognising because it is not a network problem:
>
> ```
> # github.com/serhii-chechun/pg-query-validate/cmd/pgqv
> cmd/pgqv/main.go:45:19: undefined: pgq.Parse
> ```
>
> Go has disabled cgo, so the parser simply is not compiled in. On macOS this only bites if `CGO_ENABLED` was exported to `0` globally; the default is fine.

### 2.4 From source

```bash
$ git clone https://github.com/serhii-chechun/pg-query-validate.git
$ cd pg-query-validate
$ go build -o pgqv ./cmd/pgqv
```

This is also the route for platforms without a prebuilt archive - notably musl-based distributions such as Alpine, where the glibc-linked release binary will not run:

```bash
$ apk add --no-cache go build-base
$ go build -o pgqv ./cmd/pgqv
```

### 2.5 Verifying a download

Each release also publishes `SHA256SUMS`. If the archive came from anywhere other than the release page, check it:

```bash
$ curl -LO https://github.com/serhii-chechun/pg-query-validate/releases/download/v1.0.0/SHA256SUMS
$ shasum -a 256 -c SHA256SUMS
```

```
pgqv_1.0.0_darwin_amd64.tar.gz: OK
pgqv_1.0.0_darwin_arm64.tar.gz: OK
pgqv_1.0.0_linux_amd64.tar.gz: OK
pgqv_1.0.0_linux_arm64.tar.gz: OK
pgqv_1.0.0_windows_amd64.zip: OK
```

On Linux, `sha256sum -c SHA256SUMS` is the equivalent command.

## 3. Your First Validation

`pgqv` takes exactly one file:

```bash
$ pgqv <filename.sql>
```

Point it at a schema with a typo in it, and it reports the file, the position and the fix:

```bash
$ pgqv schema.sql
```

```
error: unknown type "varchat" (did you mean "varchar"?)
 --> schema.sql:3:22
  |
3 |     name             VARCHAT(255) NOT NULL,
  |                      ^^^^^^^
```

There is nothing to configure and nothing to connect. The tool reads the file, parses it, walks the result and exits.

## 4. Reading the Output

Every finding is three lines: a message, a `file:line:column` location, and the offending source line with a caret underline sized to the token.

- **The message** names the problem and, for a typo, the type that was probably meant.
- **The location** is clickable in most terminals and editors - `schema.sql:3:22` jumps straight there.
- **The caret** narrows a long line down to the one token that is wrong, which matters in a file full of aligned column definitions.

Typos are reported in lower case even when the source is upper case, because PostgreSQL folds unquoted identifiers before the parser ever sees them. `VARCHAT` in the file becomes `"varchat"` in the message.

Exit codes make the tool usable in scripts:

| Code | Meaning |
| --- | --- |
| `0` | No problems found |
| `1` | Problems found, or the file could not be read or parsed |

## 5. Real Examples

### 5.1 Several typos in one file

The tool reports every finding in a single run rather than stopping at the first, so one pass tells you everything that needs fixing:

```bash
$ pgqv schema.sql
```

```
error: unknown type "varchat" (did you mean "varchar"?)
 --> /tmp/schema.sql:3:10
  |
3 |     name VARCHAT(255) not null,
  |          ^^^^^^^

error: unknown type "smallit" (did you mean "smallint"?)
 --> /tmp/schema.sql:4:21
  |
4 |     interval_months SMALLIT not null
  |                     ^^^^^^^

error: unknown type "tsvetcor" (did you mean "tsvector"?)
 --> /tmp/schema.sql:9:14
  |
9 |     features TSVETCOR not null,
  |              ^^^^^^^^

error: unknown type "boolen" (did you mean "boolean"?)
  --> /tmp/schema.sql:10:15
   |
10 |     is_active BOOLEN not null
   |               ^^^^^^
```

Note the gutter on the last two findings: it widens to fit the line number, so the carets stay aligned on lines 9 and 10 just as they do on line 3. `TSVETCOR` is caught by the transposition case - two adjacent characters the wrong way round, which is exactly the shape of a hand-typed typo, and which a naive "one character different" check would miss.

### 5.2 A clean file

A file with nothing wrong produces no output at all, and exits `0`:

```bash
$ pgqv clean.sql
$ echo $?
0
```

Silence on success is deliberate - it means the output of a run over a directory is exactly the list of files that need attention.

### 5.3 A syntax error is a different animal

Some problems are rejected by the grammar itself, and those are reported differently, because there is no parse tree to walk:

```bash
$ pgqv broken.sql
```

```
Processing issue: PG_SQL parsing: syntax error at or near "where"
```

Both kinds exit `1`, so a script does not have to distinguish them - but the message makes clear whether the file is malformed or merely suspicious.

### 5.4 Errors that are not validation findings

A missing file is an error, not a finding, and says so:

```bash
$ pgqv missing.sql
```

```
Error opening file: open missing.sql: no such file or directory
```

Running with no arguments prints the usage banner and exits `1`.

## 6. Using It in a Workflow

### 6.1 A loop over a schema directory

`pgqv` validates one file per invocation, which is the honest interface but does mean a directory check is a loop:

```bash
$ for f in schemas/*.sql; do pgqv "$f" || exit 1; done
```

Because a clean file is silent, wrapping the loop is enough to get a usable CI summary - the only output is the list of files that failed.

### 6.2 A CI step

The tool needs no database service, no container and no PostgreSQL client in the job, which is the whole point of running it early:

```yaml
- name: Validate PostgreSQL schemas
  run: |
    set -euo pipefail
    for f in backend/schemas/**/*.sql; do
      echo "checking $f"
      pgqv "$f"
    done
```

Using the prebuilt binary keeps the job fast; using `go install` keeps the job simple if a Go toolchain is already in the image.

### 6.3 A pre-commit hook

The most valuable place to run it is before the typo is committed:

```yaml
repos:
  - repo: local
    hooks:
      - id: pgqv
        name: Validate PostgreSQL SQL
        entry: scripts/pgqv-all.sh
        language: system
        files: \.sql$
```

The wrapper exists because of a trap worth knowing about. `pgqv` reads only its first argument and silently ignores the rest:

```bash
$ pgqv clean.sql typo.sql
$ echo $?
0
```

That second file has a `VARCHAT` in it, and the exit code is still `0` - so a hook that passes a batch of filenames straight to `pgqv` will check one file and report success for all of them. The wrapper makes the one-file interface explicit:

```bash
#!/usr/bin/env bash
# pgqv validates a single file, so check each argument in turn.
set -uo pipefail

status=0
for f in "$@"; do
  pgqv "$f" || status=1
done
exit "$status"
```

> **Gotcha:** a tool that ignores extra arguments turns a batch call into silent under-coverage. Until `pgqv` accepts multiple files or a directory, always drive it from a loop - and prefer `exit 1` on the first failure if a partial check would be misleading.

## 7. Troubleshooting

**"Refusing to load formula ... from untrusted tap."** Homebrew requires an explicit trust step for third-party taps. Run `brew trust serhii-chechun/pg-query-validate` and retry the install.

**The downloaded macOS binary will not run.** A binary downloaded through a browser carries a quarantine flag, and Gatekeeper may refuse an unsigned executable. Either allow it under *System Settings - Privacy & Security*, or clear the attribute:

```bash
$ xattr -d com.apple.quarantine /usr/local/bin/pgqv
```

Installing through Homebrew sidesteps this entirely.

**It will not run on Alpine.** The Linux archives are dynamically linked against glibc, so they do not run on musl-based systems. Build from source there (section 2.4).

**Only the first file was checked.** `pgqv` takes a single filename; extra arguments are ignored. Drive it from a loop (section 6.3).

**Piping into it does nothing.** There is no stdin support - `pgqv < schema.sql` prints the usage banner and exits `1`. Pass a filename.

**Valid PostgreSQL 18 syntax is reported as a syntax error.** This is the grammar ceiling at work:

```bash
$ pgqv pg18.sql
```

```
Processing issue: PG_SQL parsing: syntax error at or near "enforced"
```

The file is valid - `NOT ENFORCED` is a PostgreSQL 18 constraint attribute - but the tool parses with the PostgreSQL 17 grammar, because that is the newest grammar available in its parser dependency. New syntax will be recognised once that dependency ships a PostgreSQL 18 release.

## 8. Limitations

| Limitation | Why | Workaround |
| --- | --- | --- |
| PostgreSQL 17 grammar | Newest available parser dependency | Nothing yet - it is a dependency bump when released |
| No catalog resolution | Deliberately offline | Pair it with a migration dry-run against a test database |
| Type typos only, one edit away | A stricter rule fires on every user-defined type | None - this is the tradeoff that keeps it usable |
| One file per run | The current interface | Loop over files (section 6.1) |
| No stdin, no flags | Unimplemented, not a design position | Pass a filename |

Two of these deserve a sentence each, because they define what the tool is for. It is not a substitute for running migrations against a real database - it will never tell you that a column does not exist. It is a fast first line of defence that catches a specific, embarrassing class of mistake before anything else has to be involved.

## 9. Conclusion

The value of `pgqv` is mostly in what it does not need: no database, no server, no configuration, no container. That is what lets it run in a pre-commit hook - which is the only place where catching a typo is genuinely free - and it is why the tool exists at all rather than a paragraph being added to a migration checklist.

In practice the setup is two commands and a loop. Install it with `brew install pgqv` or by unpacking one archive, point it at the schema files, and let the silence on success be the signal:

```bash
$ for f in schemas/*.sql; do pgqv "$f" || exit 1; done
```

Everything else in this guide - the exit codes, the caret output, the wrapper for batches of files - is in service of that one line being safe to put in CI on a Friday afternoon.

## 10. References & Examples

**The tool**

- [pg-query-validate](https://github.com/serhii-chechun/pg-query-validate) - source, README and release notes
- [v1.0.0 release](https://github.com/serhii-chechun/pg-query-validate/releases/tag/v1.0.0) - the archives and `SHA256SUMS`
- [`Formula/pgqv.rb`](https://github.com/serhii-chechun/pg-query-validate/blob/main/Formula/pgqv.rb) - the Homebrew formula, hosted in the project's own repository

**PostgreSQL**

- [PostgreSQL: Data Types](https://www.postgresql.org/docs/current/datatype.html) - the built-in types the typo check suggests from
- [PostgreSQL 18 release notes](https://www.postgresql.org/docs/18/release-18.html) - the syntax the current grammar does not yet know
- [PostgreSQL: `CREATE TABLE`](https://www.postgresql.org/docs/current/sql-createtable.html) - where `VARCHAT` would have failed

**Tooling used in this guide**

- [Homebrew: Taps](https://docs.brew.sh/Taps) - tapping a repository other than `homebrew-core`
- [Homebrew: Tap Trust](https://docs.brew.sh/Tap-Trust) - why `brew trust` is required for third-party taps
- [pre-commit](https://pre-commit.com/) - the hook configuration in section 6.3
- [GitHub Actions](https://docs.github.com/en/actions) - the CI step in section 6.2