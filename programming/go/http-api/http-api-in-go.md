# HTTP APIs in Go: Architecture, Lifecycle & Layered Testing

Every Go codebase has one: a `main.go` that reads the environment, opens a database, registers
thirty routes inline, starts a goroutine that mutates shared state, and exits the moment the process
is asked to stop. It works. It ships. And then the deploy pipeline gets a bit faster, the pod is
replaced mid-request, and a handful of users get a `502` they will never be able to explain.

The failure is not a bug in any single line. It is the absence of decisions that were never made
explicitly: where does the SQL live, who owns the request deadline, what happens between `SIGTERM`
and process exit, and what exactly does each test in the example prove.

This post walks through a small but complete HTTP API in Go - a hotel room booking service - and
follows those decisions through the code. The full example is the
[`hotel-booking-api` example](https://github.com/serhii-chechun/developers-blog/tree/main/programming/go/http-api/examples/hotel-booking-api):
a self-contained Go 1.27 module with Gin, PostgreSQL, `go-sqlmock`, mockery v3, a Taskfile, Docker
Compose and a distroless image. Everything below is copied from it, not paraphrased.

The organizing claim is simple: **a layered structure is only worth its cost if each layer makes a
*different kind* of test possible.** If it does not, it is a folder, not an architecture.

> **The Golden Rule of layering:** every layer must be *individually replaceable*. If replacing the
> repository with an in-memory fake requires touching the handler, the layers are decorative.

---

## Table of Contents

1. [Module & Project Layout](#1-module--project-layout)
2. [The Four Layers](#2-the-four-layers)
3. [Local Setup: Docker, Environment, Tasks](#3-local-setup-docker-environment-tasks)
4. [Bootstrapping & the Container Lifecycle](#4-bootstrapping--the-container-lifecycle)
5. [Testing: One Layer, One Kind of Test](#5-testing-one-layer-one-kind-of-test)
6. [What This Example Deliberately Leaves Out](#6-what-this-example-deliberately-leaves-out)
7. [The Checklist](#7-the-checklist)
8. [Conclusion](#8-conclusion)
9. [References & Examples](#9-references--examples)

---

## 1. Module & Project Layout

### 1.1 The module root is the example root

The module is self-contained: `go.mod` sits at the root of `examples/hotel-booking-api`, and nothing
above it in the tree takes part in the build:

```text
hotel-booking-api/
+-- .mockery.yml
+-- Dockerfile
+-- README.md
+-- Taskfile.yaml
+-- docker-compose.yaml
+-- go.mod
+-- go.sum
+-- cmd/
|   +-- booking-api/
+-- internal/
|   +-- app/
|   |   +-- server/
|   |   +-- storage/
|   |   |   +-- postgres/
|   |   +-- testing/
|   |       +-- handler/
|   |       +-- model/
|   |       +-- service/
|   +-- helper/
|   |   +-- date/
|   |   +-- pagination/
|   +-- hotel/
|   |   +-- handler/
|   |   +-- model/
|   |   +-- repository/
|   |   +-- service/
|   +-- reservation/
|   |   +-- handler/
|   |   +-- model/
|   |   +-- repository/
|   |   +-- service/
|   |   +-- worker/
|   +-- room/
|       +-- handler/
|       +-- model/
|       +-- repository/
|       +-- service/
+-- specs/
    +-- openapi.yaml
    +-- postgres.sql
```

Three properties follow from that single decision, and all three are load-bearing:

- **Everything is inside the module.** `go build ./...` and `go test ./...`, run from the example
  root, cover everything there is - no `cd` into a subdirectory, no remembering which folder is "the
  real project".
- **One module, one dependency graph.** A single `go.mod` at the root means a test-only dependency
  cannot drift into code that ships, and the module can move - another directory, another
  repository - without a single import changing.
- **The Docker build context is the module root**, which is why the Dockerfile can copy `cmd`
  and `internal` and nothing else.

Note also the deliberate absence of `pkg/`. Nothing here is reusable by another module - the whole
module is an application, so everything belongs under `internal/`, which Go's compiler already
enforces: a package under `internal/` cannot be imported from outside the module. `internal/` is the
default; exporting is the exception.

**A note on the word itself.** That collision between the two meanings of "repository" is worth
naming once: from here on a bare *repository* means the data-access layer described in section 2.1,
and the code this post is about is called the *example*.

### 1.2 The module path is a plain name

```go
module hotel-booking-api

go 1.27
```

One line, and every import in the project stays short - no hostname, no owner, no path segment to
repeat:

```go
import (
	"hotel-booking-api/internal/hotel/model"
	"hotel-booking-api/internal/helper/pagination"
)
```

The convention of naming a module after the URL it is cloned from exists so that other people can
resolve it as a dependency. **This is an application, not a library.** Nothing imports it, so that
convention buys nothing here: `go get`-ability is not a feature for a binary, it is a tax paid on
every import line and every rename - and it would tie the module path to wherever the code happens
to be hosted today. The day part of this code is published as a reusable package is the day the
module path earns a hostname.

### 1.3 Dev tooling as dependencies

The `tool` directive in `go.mod` (Go 1.24+) pins the dev CLI tools inside the module itself:

```go
tool (
	github.com/go-task/task/v3/cmd/task
	github.com/vektra/mockery/v3
)
```

Nothing has to be installed globally or version-managed separately - `go tool task` and
`go tool mockery` always resolve to exactly the versions in `go.sum`. The honest cost: the tool
dependency graph is large (the task runner alone pulls in a TUI stack, an AWS SDK and OpenTelemetry
as indirect requirements), so `go.sum` is noisy and the first `go mod download` is slower than it
looks. The alternative - installing tools by hand, or via a version manager - is a longer README
paragraph and a class of "works on my machine" bug that `go.mod` eliminates for free. Also note that
`tool` directives require Go 1.24+, which is one reason the module pins `go 1.27`.

The other two files at the module root are `specs/openapi.yaml` (the contract) and
`specs/postgres.sql` (the schema) - and the same SQL file is mounted as the Compose initialization
script, so there is exactly one source of truth for each concern.

---

## 2. The Four Layers

### 2.1 Responsibilities

| Layer | Package | Responsibility | Test double |
| --- | --- | --- | --- |
| Handler | `internal/<domain>/handler` | Bind and validate input, call the service, map the result to a status code and JSON | mockery mock of the domain service |
| Service | `internal/<domain>/service` | Business rules, orchestration, invariants | mockery mock of the repository |
| Repository | `internal/<domain>/repository` | SQL only; rows to models and back | `go-sqlmock` |
| Helper | `internal/helper/{date,pagination}` | Pure functions used by all layers | none needed - direct table tests |

**The dependency arrow only ever points inward**: handler -> service -> repository -> `sql.DB`. No
layer knows its caller, and the repository has never heard of HTTP.

### 2.2 Domain-first, not layer-first

The alternative layout - `internal/handler/hotel.go`, `internal/service/hotel.go` - groups by
technical role. This example groups by domain instead: `internal/hotel/{handler,service,repository,model}`.

| Layout | Adding a feature touches | Deleting a feature | Cost |
| --- | --- | --- | --- |
| Layer-first | one file in each of three directories | carefully, three deletions | shared package namespace, so every domain name collides |
| Domain-first (chosen) | one directory | `rm -rf internal/room` | import aliases at the composition root |
| Flat (one package per service) | one file | nothing | no seams, no layers, and every test is an integration test |

The cost is real and visible in the composition root, which imports five packages named `handler`,
three named `service` and three named `repository`:

```go
import (
	handlerHotel       "hotel-booking-api/internal/hotel/handler"
	handlerReservation "hotel-booking-api/internal/reservation/handler"
	handlerRoom        "hotel-booking-api/internal/room/handler"
)
```

That is the price of domain-first: aliases in exactly one file (the wiring file), in exchange for
short, unambiguous package names everywhere else.

### 2.3 Interfaces belong to the consumer

The Go idiom is usually quoted as **"accept interfaces, return structs"** - and it has a second half
that matters more here. **Declare the interface where it is consumed, not where it is implemented.**
This is not a style preference; it is the rule as written in
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments#interfaces): *"Go interfaces
generally belong in the package that uses values of the interface type, not the package that
implements them."* The domain service does not know its repository interface lives in `service.go` -
it only knows it needs three methods:

```go
type hotelHandler struct {
	hotelService
}

type hotelService interface {
	FindAllHotels(ctx context.Context, p model.FindHotelsParams) (*model.HotelsPage, error)
}
```

Every interface in the project follows the same shape:

- `hotelService` in the handler package - one method, because the hotel handler lists hotels.
- `hotelRepository` in the service package - three methods: `GetHotelsByName`, `PutHotels`,
  `DeleteAllHotels`, which are the only ones the service calls.
- `storage` in the server package - `Connect()` and `Close()`, nothing more, even though the
  PostgreSQL type behind it does a great deal more.
- `reservationWorker` in the server package - `Start(ctx)`, so the composition root can be tested
  without a running background loop.

Two details are worth flagging for anyone copying the pattern.

> **Gotcha:** constructors here return *unexported* types (`func New(...) *hotelHandler`). That is
> idiomatic inside a module - the caller can hold the value and call its exported methods - but it
> makes the package impossible to fake from outside, and some linters object to it. The safety net
> in this project is `internal/`: nobody outside the module can depend on these packages anyway.

The second detail is the cost of many small interfaces: each one needs a mock. That is why
`internal/<domain>/service/mock` exists at all, and why `.mockery.yml` is curated rather than
generated wholesale (`all: false`, then an explicit list). Where an interface has a single trivial
method and only one consumer needs it, a hand-written stub in the test file is cheaper than a
generated mock - generate mocks for the repository and service seams, hand-write everything else.

### 2.4 Where the business logic actually lives

Honesty first: `hotelService.FindAllHotels` is a pass-through.

```go
// FindAllHotels finds hotels by their name using the hotel repository.
func (s *hotelService) FindAllHotels(ctx context.Context, p model.FindHotelsParams) (*model.HotelsPage, error) {
	return s.hotelRepository.GetHotelsByName(ctx, p)
}
```

Three wrappers around a repository look like ceremony, and if the service layer never did anything
else, calling it that would be fair. It exists as a **seam**: it gives a future invariant somewhere
to live ("never return deactivated hotels"), it keeps handlers from depending on repositories
directly, and it is where the request's `context` gets a chance to be observed. The point is not
that every layer must be thick - it is that each layer must have a reason to change independently
of its neighbours.

The reservation domain is where that bet pays off, because it has rules the database cannot express
in a constraint:

```go
for range maxReferenceAttempts {
	r.Reference = generateReference()

	existing, err := s.reservationRepository.GetReservationByReference(ctx, r.Reference)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		continue
	}

	return s.reservationRepository.PutReservation(ctx, r)
}
```

A booking is made for a *party size*, not a specific room, so the service asks the repository to
allocate one room - or a combination of rooms - that holds the party for the entire date range,
preferring the smallest sufficient room, then falling back to sorting candidates by capacity and
distributing guests across them. The allocation rule itself is out of scope here; what matters for
this post is that it is a rule, not a query - and that it has somewhere to live.

### 2.5 Cross-domain work gets its own domain

Seeding and resetting test data touches hotels, rooms and reservations. Rather than letting
`internal/hotel` know about `internal/room`, the cross-cutting concern becomes a domain of its own
under `internal/app/testing`, composing the three services behind one wider interface - and
importing nothing from the other domains except their models:

```go
type testingService struct {
	hotelService
	roomService
	reservationService
}
```

`SeedTestData` resets everything, loads room types, generates hotels, rotates room types across the
generated rooms, and inserts. `ResetTestData` removes reservations, then rooms, then hotels - in
that order, because the foreign keys demand it.

> **Gotcha:** `PUT /v1/testing` is a demo affordance and nothing else. In a real deployment it must
> be gated - a config flag that skips route registration, or a build tag that omits the package
> entirely. Shipping it silently is the kind of mistake that only shows up in a postmortem.

### 2.6 What the request path actually does

```text
GET /v1/hotels?name_pattern=mercury&page_size=5
        |
        V
gin engine --> hotelHandler.GetAll
                 |  c.ShouldBindQuery(struct{ NamePattern; NextPage; PageSize })
                 |  pagination.NormalizeSize(size)   pagination.Decode(token)
                 V
              hotelService.FindAllHotels(ctx, model.FindHotelsParams)
                 V
              hotelRepository.GetHotelsByName
                 |  ctx, cancel := context.WithTimeout(ctx, 5s)
                 |  QueryContext(selectHotelsQuery, pattern, afterID, pageSize+1)
                 V
              PostgreSQL --> rows --> scan --> trim the extra row --> encode next_page
```

Four decisions hide in that path.

**Validation belongs to the handler.** Each handler package has a `validate.go`, so binding and
rejection are one function away from the route that uses them:

```go
func validateGetAllRequest(c *gin.Context) (model.FindHotelsParams, error) {
	var query struct {
		NamePattern string `form:"name_pattern"`
		NextPage    string `form:"next_page"`
		PageSize    int    `form:"page_size"`
	}
	...
	result := model.FindHotelsParams{
		NamePattern: query.NamePattern,
		PageSize:    pagination.NormalizeSize(query.PageSize),
	}
	...
}
```

The handler returns `400` before the service is ever called - a fact the tests assert by *not*
setting up any mock expectation.

**Pagination is keyset, not `OFFSET`.** The query asks for one row more than the page size:

```sql
SELECT id, name, address, phone
FROM hotels
WHERE name ILIKE '%' || $1 || '%'
  AND ($2 = '' OR id > $2)
ORDER BY id
LIMIT $3
```

The extra row is the "is there a next page" signal; the last kept row's id becomes an opaque token
(`base64.RawURLEncoding` of `{"after_id":"..."}`), which is why the repository can paginate without
counting anything. And the argument is `pageSize+1` - a behavioural contract, not an implementation
detail: a regression that fetches exactly `pageSize` would silently make `next_page` disappear from
every response.

> **Gotcha:** that token is *opaque, not signed*. A client can decode it and forge an `after_id`.
> For public data ordered by a key this is fine; as an authorization cursor it is not, because
> "continue after this id" becomes an invitation to guess ids. Opaque means "do not parse this",
> not "trust me".

**One round trip instead of N.** Bulk inserts use a single statement with array parameters:

```sql
INSERT INTO hotels (id, name, address, phone)
SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::text[])
```

The repository builds the arrays and passes them as `pq.Array(...)`. Three hotels or three hundred,
it is still one statement.

**The deadline is owned by the layer that knows it.** The repository applies `DB_QUERY_TIMEOUT`
per call, and the handler never thinks about time at all:

```go
ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
defer cancel()
```

### 2.7 Errors: wrapping, and one honest gap

Every layer wraps what it returns, with the operation as context:

```go
rows, err := r.db.QueryContext(ctx, selectHotelsQuery, p.NamePattern, p.AfterID, p.PageSize+1)
if err != nil {
	return nil, fmt.Errorf("find hotels by name: %w", err)
}
```

`%w` keeps the chain walkable, so callers can test with `errors.Is`; the tests in this example
assert on those prefixes, which is what makes them behaviour tests rather than line coverage.

The gap, stated plainly: the handlers currently map *any* service error to `500` and echo
`err.Error()` into the response body, which can leak internals to a client. The shape to aim for is
a small vocabulary of sentinel errors in the service and one mapping function in the handler:

```go
func statusFor(err error) int {
	switch {
	case errors.Is(err, model.ErrHotelNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrInsufficientCapacity):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
```

Fixing that is a change of its own and out of scope here; it is named because a known boundary is
more useful to a reader than an example presented as finished.

### 2.8 When *not* to layer

> **Gotcha:** three files and two interfaces around a `SELECT` are a cost. If a service has one
> table, no invariants beyond the schema, and no second consumer, one package is the better design.
> Layers buy replaceability; if nothing will ever be replaced, buy nothing.

---

## 3. Local Setup: Docker, Environment, Tasks

### 3.1 Requirements

Go 1.27+, Docker, and `jq` if one wants pretty-printed responses. Nothing else - the task runner and
the mock generator are module-pinned.

### 3.2 One command for the whole stack

```bash
$ docker compose up --build
```

PostgreSQL starts on `5432` and the API on `8080`. Three details in `docker-compose.yaml` make that
reliable rather than lucky: the `storage` service declares a healthcheck (`pg_isready -U ... -d ...`),
the `server` service waits for it with `depends_on: { storage: { condition: service_healthy } }`,
and the schema is mounted into `/docker-entrypoint-initdb.d/`, so a fresh database is initialized
with `specs/postgres.sql` on first start.

> **Gotcha:** the entrypoint script runs only against an *empty* data volume. After editing
> `specs/postgres.sql`, `docker compose up` will happily reuse the old schema - run
> `docker compose down -v` first. This is the most common local-setup failure in the project's
> README, and it is worth a line there for exactly that reason.

### 3.3 The hybrid loop

For day-to-day work, the container is only needed for the database:

```bash
$ docker compose up -d storage
$ go run ./cmd/booking-api
```

This keeps the edit-run cycle fast while still testing against real PostgreSQL, and it proves
configuration is fully environment-driven - the same binary runs in both setups.

### 3.4 Configuration with working defaults

```go
type config struct {
	ListenAddress   string        `env:"LISTEN_ADDRESS,default=:8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT,default=10s"`

	WorkerInterval time.Duration `env:"WORKER_INTERVAL,default=10s"`

	PostgresUser     string `env:"POSTGRES_USER,default=booking_api"`
	PostgresPassword string `env:"POSTGRES_PASSWORD,default=booking_api"`
	PostgresDB       string `env:"POSTGRES_DB,default=booking_api"`
	PostgresHost     string `env:"POSTGRES_HOST,default=localhost"`
	PostgresPort     int    `env:"POSTGRES_PORT,default=5432"`
	...
}
```

The variable set falls into four groups:

- **Listening and lifecycle** - `LISTEN_ADDRESS`, `SHUTDOWN_TIMEOUT`.
- **Database connection** - `POSTGRES_*`, plus `DB_CONNECT_MAX_RETRIES` and `DB_CONNECT_RETRY_DELAY`.
- **Connection pool** - `DB_MAX_IDLE_CONNS`, `DB_MAX_OPEN_CONNS`, `DB_CONN_MAX_IDLE_TIME`,
  `DB_CONN_MAX_LIFETIME`.
- **Background work** - `WORKER_INTERVAL`, and `DB_QUERY_TIMEOUT` for per-query deadlines.

The useful property is that **the defaults in code are the Compose configuration.** The container
overrides exactly one variable, `POSTGRES_HOST=storage`, because that is the only value that
genuinely differs between a laptop and a container network. There is one place to reason about
configuration even though there are two environments.

### 3.5 Tasks instead of a wiki page

```bash
$ go tool task          # list the available tasks
$ go tool task tests    # per-package coverage plus the overall total
$ go tool task mocks    # delete the generated mocks, then regenerate them
```

The `tests` task looks like this, and every character of it is a decision:

```yaml
tests:
  cmds:
    - defer: rm -f coverage.out
    - go test -coverprofile=coverage.out $(go list ./... | grep -v /mock)
    - go tool cover -func=coverage.out | tail -1
```

- `/mock` packages are excluded, because generated code should not inflate - or deflate - the
  coverage denominator.
- The overall total is printed last, so the one number a reader cares about is the last line on
  screen.
- `defer: rm -f coverage.out` means the task leaves no artifacts behind; a clean tree after a test
  run is a small thing that removes a whole category of accidental commits.

`mocks:clean` runs `find internal -type d -name mock -prune -exec rm -rf {} +` before regenerating,
so **a stale mock cannot survive an interface change**. Without that, a renamed method leaves an old
mock that still compiles, and the next green test run is a lie.

Readers who expect a `Makefile`: `Taskfile.yaml` is the Makefile here. The reason is only that
`task` is pinned alongside the other tools instead of depending on whichever version happens to be
installed; `make test` wrapping the same two `go` commands is an equally good answer. What matters
is that there is exactly one documented entry point, and a new contributor never has to guess the
invocation.

### 3.6 The manual walkthrough

The README doubles as a demo script, and the whole story fits in six commands:

```bash
$ curl -s -X PUT 'http://localhost:8080/v1/testing?max_hotels=1&max_rooms_per_hotel=6' | jq
$ curl -s 'http://localhost:8080/v1/hotels?name_pattern=mercury' | jq
$ curl -s 'http://localhost:8080/v1/rooms?guests_count=2&check_in_date=2026-10-01&check_out_date=2026-10-03' | jq
$ curl -s -X POST http://localhost:8080/v1/reservations -H 'Content-Type: application/json' -d @booking.json | jq
$ curl -s http://localhost:8080/v1/reservations/CYYKSX24PR | jq
$ curl -i -X DELETE http://localhost:8080/v1/testing
```

Seed, search, check availability, book, look the booking up, reset. `PUT /v1/testing` replaces
whatever was there, so the sequence is repeatable.

| Status | Meaning |
| --- | --- |
| `200` | Success |
| `204` | Reset succeeded |
| `400` | Missing or invalid parameters, dates in the past, `check_out_date` not after `check_in_date` |
| `404` | Unknown hotel, unknown requested room, or unknown booking reference |
| `409` | The party cannot be accommodated, a requested room is no longer free, or a booking reference collision |
| `500` | Unexpected server error |

Two API-shape notes: list endpoints are paginated with `page_size` (default 20, maximum 100) and an
opaque `next_page` token, and `GET /health` deliberately sits **outside** the `/v1` group - probes
are infrastructure, not API surface, and their path should not need versioning.

---

## 4. Bootstrapping & the Container Lifecycle

### 4.1 `main` is nearly empty

```go
func main() {
	if err := server.New().Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

No package-level globals, no `init()`, no `log.Fatal` buried in a library package, and `os.Exit`
appears exactly once - in `main`, where it belongs. The entry point is untestable by design, and
that is fine, because everything it delegates to is not.

### 4.2 `Run()` as an explicit lifecycle

`apiServer` composes its collaborators as embedded fields - the storage engine, the workers, the
router, the configuration and the logger - and `Run()` executes the lifecycle in a fixed order:

1. **Process the configuration.** Invalid values fail *before* any resource is opened.
2. **Initialize.** Build the logger, connect to storage (with retries), then wire repositories into
   services into handlers into routes.
3. **Start the workers** in their own goroutine, with their own cancelable context.
4. **Build the HTTP server** - `Addr`, `Handler`, and a `BaseContext` returning the `Run`-scoped
   context.
5. **Serve** in a goroutine; a failure that is not `http.ErrServerClosed` lands on a buffered
   `failure` channel of size one.
6. **Wait** on `select` for either that failure or a signal.
7. **Drain** with `Shutdown` bounded by `SHUTDOWN_TIMEOUT`, falling back to `Close()` if the drain
   times out.
8. **Stop the workers**, wait for them to return, then close storage.
9. **Return** `errors.Join(runErr, closeErr)`, so a shutdown error is not lost when a run error
   already exists.

The wiring step is the one place where the whole graph is visible - and the only place that needs
to know about every layer:

```go
	reservationService := serviceReservation.New(
		repositoryReservation.New(db, s.config.DatabaseQueryTimeout),
	)
	...
	s.handlerMux = s.registerRoutes(gin.Default(), &handlers{
		hotelHandler:       handlerHotel.New(hotelService),
		roomHandler:        handlerRoom.New(roomService),
		reservationHandler: handlerReservation.New(reservationService),
		...
	})
```

**Shutdown runs in the reverse order of startup, and the database is closed last** - because both
in-flight requests and the background worker are still using it.

### 4.3 Why the workers get their own context

The HTTP server already has a drain mechanism, so the worker cannot share the server's lifetime: it
needs a signal that fires *after* the drain finishes but *before* storage closes.

```text
SIGTERM --> httpServer.Shutdown(timeout)   stop accepting, drain in-flight
                 | (up to SHUTDOWN_TIMEOUT)
                 V
          cancel workersCtx --> worker returns from its select loop
                 V
          <-workersDone --> storage.Close() --> Run() returns
```

The temptation is to reuse the root context for everything - and that is exactly the mistake
`BaseContext` makes subtle. The context handed to `BaseContext` becomes the parent of every
request's context, so cancelling the root would abort in-flight requests instead of letting them
finish: the precise opposite of a graceful shutdown. Two contexts, two jobs.

### 4.4 The worker

```go
func (w *reservationWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("reservation worker started")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("reservation worker stopped")
			return
		case <-ticker.C:
			processed := w.processPendingReservations(ctx)
			w.log.Info("processed pending reservations", "count", processed)
		}
	}
}
```

Pending bookings are confirmed on a ticker, and the simulated confirmation email lives inside a
`select` that also watches the context - so the two-second wait is interruptible rather than an
unconditional `time.Sleep`. The batch loop checks `ctx.Err()` between items for the same reason: a
worker that only checks on tick boundaries will be `SIGKILL`ed in the middle of a batch.

The `recover()` around the batch is a pragmatic guard so one bad reservation cannot take the process
down - not a substitute for fixing whatever panics, and it is written that way on purpose.

### 4.5 Why signals matter in containers

- **PID 1 semantics.** The image is `gcr.io/distroless/static-debian12:nonroot` with an exec-form
  entrypoint (`ENTRYPOINT ["./booking-api"]`), so the binary *is* PID 1 and receives `SIGTERM`
  directly. A shell-form `CMD ./booking-api` wraps it in `/bin/sh`, which does not forward signals -
  the process would be killed after the grace period with no cleanup at all.
- **The default action of `SIGTERM` terminates the process immediately.** Go only turns it into a
  channel delivery if the program subscribes with `signal.Notify`. Without that subscription there
  is no drain, no `Close()`, no worker shutdown - which is the failure from the opening paragraph.
- **Teardown in Kubernetes is a race.** On pod deletion the endpoint is removed from the
  EndpointSlice and `SIGTERM` is delivered roughly in parallel, so requests can still arrive after
  the signal. Graceful shutdown narrows the window; a `preStop` hook and a grace period longer than
  the drain close it. The user-visible symptom of getting this wrong is `502 Bad Gateway` on a
  handful of requests during *every* rollout.

| Knob | Value here | Why it must be set this way |
| --- | --- | --- |
| `SHUTDOWN_TIMEOUT` | `10s` | Upper bound on the drain |
| `terminationGracePeriodSeconds` | must exceed the above | Otherwise the kubelet `SIGKILL`s mid-drain |
| `docker stop` (Compose `down`) | `10s` default, then `SIGKILL` | Same rule; `SIGTERM` first is why the handler must exist |
| `preStop` hook | optional short sleep | Covers endpoint-propagation lag |
| readiness probe | withdrawn before shutdown | Prevents new traffic during the drain |

A manifest fragment would be out of place in a post about Go: the mechanics above are the subject,
and the YAML is a one-line consequence of them.

Two more mechanisms are worth naming:

- **Both `SIGINT` and `SIGTERM` are registered**, because `docker compose down` and Kubernetes send
  `SIGTERM`, while Ctrl-C in a terminal sends `SIGINT`. Handling one leaves a gap in exactly one of
  the two environments.
- **Errors wrapped with `%w` all the way up and printed once in `main`** mean `docker logs` and
  `kubectl logs` show a single readable line before exit code 1 - which is what turns a
  `CrashLoopBackOff` into a diagnosis instead of a mystery.

This example registers only `/health`. A production shape splits liveness (no dependencies, always
fine) from readiness (checks the database), so a broken database removes the pod from service
instead of restarting it in a loop.

> **Gotcha:** an `http.Server` with no `ReadHeaderTimeout` is trivially exposed to slow-loris
> attacks; a reverse proxy mitigates but does not fix it. Also absent here: `ReadTimeout`,
> `WriteTimeout`, `MaxHeaderBytes` and per-route timeouts. They are left out so the bootstrap stays
> readable - in a real service the struct looks like this:

```go
httpServer := &http.Server{
	Addr:              s.config.ListenAddress,
	Handler:           s.handlerMux,
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       30 * time.Second,
	WriteTimeout:      30 * time.Second,
	IdleTimeout:       60 * time.Second,
	MaxHeaderBytes:    1 << 20,
}
```

---

## 5. Testing: One Layer, One Kind of Test

### 5.1 The map

| Layer | Tool | What is faked | What the test proves | What it cannot catch |
| --- | --- | --- | --- | --- |
| `internal/app/server` | mocks + `httptest` + `testing/synctest` | handlers, storage, worker | the route table, fail-fast config, worker hand-off | handler internals, SQL |
| `<domain>/handler` | mock of the service + `httptest.Recorder` | the service | binding, validation, status codes, response JSON | route registration |
| `<domain>/service` | mock of the repository | the repository | orchestration, error propagation | SQL, HTTP |
| `<domain>/repository` | `go-sqlmock` | the database | the SQL text, the arguments, error wrapping | whether PostgreSQL *accepts* the SQL |
| `internal/helper/*` | plain table tests | nothing | pure logic: cursors, clamping, dates | nothing - and that is the point |

**The layer you fake tells you which layer you are testing.** That is what the interfaces in section 2.3
were for.

### 5.2 Server tests

Three kinds of assertion live at this level. First, that configuration failures fail *fast* - an
invalid duration or port must be rejected before anything is opened:

```go
		{
			name:        "fails fast on an invalid shutdown timeout",
			env:         map[string]string{"SHUTDOWN_TIMEOUT": "soon"},
			wantErrText: "failed to process configuration:",
		},
```

Second, that the route table says what it claims to say. `TestRegisterRoutes` builds the engine with
mocked handlers and drives it through `httptest`, asserting that `/health` answers outside the
version prefix, that `GET /v1/reservations/REF-1` reaches `reservationHandler.Get` with
`c.Param("booking_ref")` set, that an unknown path **and** a known path with an unregistered method
both return `404`, and that `/v1/hotels/` redirects with `301`.

Third, and most interesting, that the worker hand-off is ordered. That test uses
`testing/synctest`:

```go
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		done := make(chan struct{})
		...
		go func() {
			defer close(done)
			server.startWorkers(ctx)
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("startWorkers returned before the worker did")
		default:
		}
	})
```

`testing/synctest` gives a deterministic scheduler and virtual time: `synctest.Wait()` guarantees
the goroutine has parked, so "has not returned yet" is a real assertion rather than a race that
passes on a fast machine. It is made for exactly this class - tickers, background goroutines and
shutdown ordering - and it removes the `time.Sleep(100 * time.Millisecond)` that would otherwise be
load-bearing. Note also `t.Context()`, which cancels when the test ends and removes a whole family
of leaked-goroutine bugs.

### 5.3 Handler tests

The handler is called directly, with no router:

```go
			recorder := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/hotels?"+tt.query, nil).
				WithContext(t.Context())

			New(service).GetAll(c)

			assert.Equal(t, tt.wantStatus, recorder.Code)
```

This isolates binding, validation and status mapping - and the cost is that a route typo is
invisible here. That is precisely why `TestRegisterRoutes` exists: one test owns routing, the other
owns the handler body. Naming that division of labour is more useful than pretending either test is
complete.

Two cases in the table are worth pointing at, because they encode intent:

- The validation cases (`page_size=abc`, `next_page=not_a_token!!`) have **no** mock expectations at
  all. The absence *is* the assertion: the service was never called.
- `null` is an expected body when the service returns a nil page - asserting it verbatim makes any
  accidental nil-handling change visible instead of quietly acceptable.

### 5.4 Service tests

```go
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().
					GetHotelsByName(ctx, model.FindHotelsParams{NamePattern: "grand", PageSize: 20}).
					Return(page, nil).
					Once()
			},
```

Two assertions carry more meaning than the rest. `assert.Same` proves the service returns *the same*
page object, catching an accidental copy or rebuild that `assert.Equal` would accept. And
`require.ErrorIs` proves the repository error survived the layer intact - which is only true if the
wrapping used `%w`:

```go
			got, err := newTestService(t, tt.expectations).FindAllHotels(ctx, tt.params)

			require.NoError(t, err)
			assert.Same(t, tt.want, got)
```

The edge cases make the suite credible: empty input passed through unchanged, an empty page returned
as it is, and errors surfaced without embellishment.

### 5.5 Repository tests, and the SQL contract

The repository is the layer where "did we write the query we think we wrote" actually gets tested:

```go
func newTestRepository(t *testing.T, expectations func(mock sqlmock.Sqlmock)) *hotelRepository {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })

	expectations(mock)

	return New(db, testQueryTimeout)
}
```

That second `t.Cleanup` is the most valuable line in the file: **an unfulfilled expectation fails
the test**, so a query that is never executed - or executed with different arguments - cannot slip
through on the strength of the other assertions.

Queries are pinned by reference to the production constants, never by copying the SQL into the test:

```go
				mock.ExpectQuery(regexp.QuoteMeta(selectHotelsQuery)).
					WithArgs("grand", "", 3).
					WillReturnRows(hotelRows(hotels[0], hotels[1]))
```

`regexp.QuoteMeta` escapes the multi-line query, and reusing `selectHotelsQuery` means a renamed
column or a changed `WHERE` clause fails the test without the test duplicating the query text - one
source of truth, still verified. The arguments are pinned too, and the third one is `3`:
`pageSize+1`, the pagination contract from section 2.6 encoded as an assertion.

The interesting half of the table is the failure paths, because each row corresponds to a real bug
class:

| Case | How it is simulated | Asserted |
| --- | --- | --- |
| Query failure | `WillReturnError(errDefault)` | `"find hotels by name:"` |
| Scan failure | a `nil` for a non-nullable column | `"scan hotel:"` |
| Iteration failure | `hotelRows(...).RowError(0, errDefault)` | `"iterate hotels:"` |
| Exec failure | `WillReturnError(errDefault)` | `"insert hotels:"` / `"delete all hotels:"` |

The iteration case is the one worth copying: `rows.Err()` is the check most codebases forget, and
`RowError` is how it gets a test.

What this buys: the SQL text, the argument order, the pagination arithmetic and four error paths,
with no database, in milliseconds. What it does not buy, stated plainly: `ILIKE`, the
`unnest($1::text[], ...)` insert and the type casts are never parsed by PostgreSQL. A typo in a cast
would pass every test in this file.

### 5.6 Mocks: generated, curated, disposable

`.mockery.yml` is short, and every setting is a decision:

```yaml
dir: '{{.InterfaceDir}}/mock'
filename: 'mocks.go'
structname: 'Mock{{.InterfaceName | firstUpper}}'
pkgname: 'mock'
template: testify
formatter: goimports
formatter-options:
  goimports:
    local-prefix: hotel-booking-api
all: false
```

- `all: false` with an explicit list of packages and interfaces means only the chosen seams are
  mocked - and the file itself documents where the architecture's replaceable points are.
- `dir: '{{.InterfaceDir}}/mock'` puts mocks in a `mock` sub-package next to the interface, so no
  package both is mocked and does the mocking.
- `template: testify` gives the `EXPECT()...Once()` style used above.
- `local-prefix: hotel-booking-api` - the module name from section 1.2 reappears, because import grouping
  of the generated files depends on it. A nice demonstration that a module path is not cosmetic.

`go tool task mocks` deletes every `mock` package and regenerates it, which is what makes a
regenerated mock trustworthy after an interface change.

### 5.7 What this suite is worth

The whole thing runs with no Docker, no PostgreSQL and no network, in parallel
(`t.Parallel()` appears in nearly every test), in well under a second. What it verifies is the set of
*contracts between layers* - the SQL text, the argument lists, the error vocabulary, the route
table, the shutdown ordering. What it does not verify is the system: no real database ever parses
those statements, and no request travels from socket to storage.

The lane that would close that gap is easy to describe: boot a throwaway PostgreSQL - the Compose
service that already exists, or [Testcontainers for Go](https://golang.testcontainers.org/) when the
test should own the lifecycle - apply `specs/postgres.sql`, run the repository methods and assert on
real rows. It buys hermeticity at the price of a Docker dependency in CI and seconds per run.

At the outermost rung sits a smoke test that starts the whole stack and drives one booking from seed
to lookup. It is the only test that can catch a wiring mistake in `init()` - and wiring is exactly
what per-layer mocks are blind to. Neither the integration lane nor the smoke test exists in this
example yet; both are named in section 6 rather than implied to be present.

**Testing each layer separately is what makes the layering real.** If a layer cannot be tested with
exactly one thing faked beneath it, it is not a layer - it is a folder.

---

## 6. What This Example Deliberately Leaves Out

Collected in one place, with the reason and the fix, so the scope is a decision rather than a series
of apologies.

| Omitted | Why it is absent | What the fix looks like |
| --- | --- | --- |
| Authentication / authorization | Out of scope for an architecture post | Middleware plus a per-route policy |
| A real readiness probe (only `/health` exists) | One probe is enough to run locally | Split liveness (no dependencies) from readiness (DB ping) |
| `ReadHeaderTimeout`, `ReadTimeout`, `MaxHeaderBytes` | Kept out so the bootstrap stays readable | The hardened `http.Server` in section 4.5 |
| An error taxonomy (`errors.Is` to status mapping) | Handlers currently return `500` with `err.Error()`; naming the gap beats half-fixing it | Sentinel errors in the service, mapping in the handler |
| Integration tests against a real PostgreSQL | Would add a Docker dependency to every test run | The lane described in section 5.7 |
| An end-to-end smoke test | Same reason - the per-layer suite is the point of section 5 | One seeded booking driven through the real stack |
| Metrics and tracing | Meaningless without a backend | `otelhttp` middleware plus `/metrics` |
| Migrations | Compose applies `specs/postgres.sql` as an init script | A migration tool and versioned files |
| A production gate on `/v1/testing` | It is a demo affordance | A config flag or a build tag |

None of these is a hidden gap. Each is a delimited next step, and the checklist below is the audit
that catches them.

---

## 7. The Checklist

**Structure**

- [ ] One `go.mod` at the example root; the module path is short and stable, not a URL.
- [ ] `cmd/<binary>` contains only `main` - no `init()`, no globals, one `os.Exit`.
- [ ] One can name, in a sentence, what each layer may and may not import.
- [ ] Interfaces are declared where they are consumed, and are as narrow as the consumer needs.
- [ ] `internal/` everywhere something would otherwise be published by accident.

**Local workflow**

- [ ] `docker compose up --build` brings the stack up from a clean clone.
- [ ] Schema changes are documented as requiring `down -v`.
- [ ] Dev tooling is pinned in `go.mod` (`tool`) or equivalent - not installed by hand.
- [ ] Generated mocks are deleted before regeneration; coverage excludes them.

**Lifecycle**

- [ ] `SIGINT` **and** `SIGTERM` are handled; the process exits non-zero with one readable log line.
- [ ] Shutdown order is: stop accepting -> drain -> stop background work -> close the database.
- [ ] Background workers select on `ctx.Done()` and check `ctx.Err()` between items.
- [ ] `terminationGracePeriodSeconds` exceeds the shutdown timeout; the entrypoint is exec-form.
- [ ] `ReadHeaderTimeout` and friends are set, or the omission is deliberate and documented.

**Tests**

- [ ] Every layer has tests that fake exactly one thing beneath it.
- [ ] SQL is asserted against the production constants, not against copies.
- [ ] Argument assertions include the pagination arithmetic.
- [ ] `rows.Err()` has a test.
- [ ] Fake expectations are verified in `t.Cleanup`.

---

## 8. Conclusion

The layering is what makes the four different kinds of test possible, and the bootstrap is what makes
the layer stack deployable. Everything else in this post is in service of two sentences:
**architecture is the set of seams you can replace, and the bootstrap is the promise that the
process will not be killed mid-request.** A `Handler` interface nobody can fake, or a signal handler
nobody installed, undoes both - and neither failure shows up in a code review of a single file.

At the same time, this example is a teaching shape. Everything it lacks fits in one table, one
section up: authentication, metrics, integration tests, a real readiness probe, timeouts on the HTTP
server. Each is a delimited next step rather than a hidden assumption, and two of them are the ones
worth reaching for first: **an error taxonomy across layer boundaries**, and the **room-allocation
rule** that section 2.4 only gestures at.

---

## 9. References & Examples

**Examples**

- [The `hotel-booking-api` example](https://github.com/serhii-chechun/developers-blog/tree/main/programming/go/http-api/examples/hotel-booking-api) - the complete, runnable module
- [`specs/openapi.yaml`](https://github.com/serhii-chechun/developers-blog/blob/main/programming/go/http-api/examples/hotel-booking-api/specs/openapi.yaml) - the API contract
- [`specs/postgres.sql`](https://github.com/serhii-chechun/developers-blog/blob/main/programming/go/http-api/examples/hotel-booking-api/specs/postgres.sql) - the schema, also the Compose init script

**Go documentation**

- [`net/http`](https://pkg.go.dev/net/http) - `Server.Shutdown`, `Server.BaseContext`, `ErrServerClosed`
- [`os/signal`](https://pkg.go.dev/os/signal) - `Notify`, and the default disposition of `SIGTERM`
- [`context`](https://pkg.go.dev/context) - deadlines, cancellation and propagation
- [`database/sql`](https://pkg.go.dev/database/sql) - connection pool settings, `BeginTx`
- [`errors`](https://pkg.go.dev/errors) - `Is`, `As`, `Join`, and `%w` wrapping
- [`testing/synctest`](https://pkg.go.dev/testing/synctest) - deterministic concurrency tests with virtual time
- [`go.mod` `tool` directives](https://go.dev/doc/modules/managing-dependencies) - pinning dev tools as dependencies
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) - the *Interfaces* section is the source of the consumer-side rule quoted in section 2.3; the same page covers `internal` and error wrapping

**Libraries used**

- [Gin](https://github.com/gin-gonic/gin), [`lib/pq`](https://github.com/lib/pq),
  [`go-sqlmock`](https://github.com/DATA-DOG/go-sqlmock), [mockery](https://github.com/vektra/mockery),
  [`go-envconfig`](https://github.com/sethvargo/go-envconfig),
  [testify](https://github.com/stretchr/testify), [Task](https://taskfile.dev/),
  [`google/uuid`](https://github.com/google/uuid)

**Container lifecycle**

- [Kubernetes: Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/) - `preStop`, grace periods
- [Docker: `docker stop`](https://docs.docker.com/reference/cli/docker/container/stop/) - the `SIGTERM`-then-`SIGKILL` sequence
- [Testcontainers for Go](https://golang.testcontainers.org/) - an integration lane, when it is time for one

**Related posts**

- [Optimizations in Go: A Multi-Layer Framework](https://blog.chechun.org/) - same
  "every claim is reproducible" promise
- [Regular Expressions in Go: A Deep Dive](https://blog.chechun.org/) - same framing: a design
  decision, and the trade it implies
