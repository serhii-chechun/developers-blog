# booking-api-demo

Hotel room booking API written in Go (Gin + PostgreSQL).

Hotels have three room types - single, double and deluxe. A booking is made for a
party size rather than a specific room: the server allocates the rooms for the whole
date range, so guests are never asked to change rooms during their stay.

## Requirements

- Go 1.27+ (or Docker)
- PostgreSQL 15+ (or Docker)

## Running the API

### Docker Compose (recommended)

```bash
docker compose up --build
```

Starts PostgreSQL on `localhost:5432` (schema loaded from `specs/postgres.sql`) and
the API on `http://localhost:8080`.

### Locally

Start only the database with Compose, then run the API from the repository root:

```bash
docker compose up -d storage
go run ./cmd/booking-api
```

Configuration is read from the environment:

| Variable | Default | Description |
| --- | --- | --- |
| `LISTEN_ADDRESS` | `:8080` | API listen address |
| `POSTGRES_HOST` | `localhost` | Database host |
| `POSTGRES_PORT` | `5432` | Database port |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `booking_api` | Database credentials |
| `WORKER_INTERVAL` | `10s` | How often the booking worker runs |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |
| `DB_CONNECT_MAX_RETRIES` / `DB_CONNECT_RETRY_DELAY` | `5` / `5s` | Connection retry on startup |
| `DB_QUERY_TIMEOUT` | `5s` | Deadline applied to each repository query |
| `DB_CONN_MAX_IDLE_TIME` / `DB_CONN_MAX_LIFETIME` | `5m` / `30m` | Connection pool connection lifetimes |
| `DB_MAX_IDLE_CONNS` / `DB_MAX_OPEN_CONNS` | `10` / `100` | Connection pool sizes |

> The schema is applied only to a **fresh** database volume. After changing
> `specs/postgres.sql`, run `docker compose down -v` before starting again.

## API overview

Base path: `http://localhost:8080/v1` - `GET /health` sits outside the version prefix.

| Method | Path | Purpose |
| --- | --- | --- |
| `PUT` | `/v1/testing?max_hotels=&max_rooms_per_hotel=` | Seed test data (replaces any existing data) |
| `DELETE` | `/v1/testing` | Remove all data |
| `GET` | `/v1/hotels?name_pattern=` | Find hotels by name (case-insensitive substring) |
| `GET` | `/v1/rooms?guests_count=&check_in_date=&check_out_date=[&hotel_id=]` | Find free rooms whose hotel can host the party for the whole date range (one room or a combination) |
| `POST` | `/v1/reservations` | Book a stay (optionally for specific `room_ids`) |
| `GET` | `/v1/reservations/{booking_ref}` | Find booking details by reference |

The complete contract, including request/response schemas and status codes, is in
[`specs/openapi.yaml`](specs/openapi.yaml). 

List endpoints are paginated with `page_size` (default `20`, maximum `100`) and an
opaque `next_page` token.

## Testing walkthrough

The commands below use [`jq`](https://jqlang.github.io/jq/) to pretty-print responses;
they work with `curl` alone as well.

### 1. Seed test data

Populates the database with hotels and rooms. Seeding resets first, so it can be run
repeatedly.

```bash
curl -s -X PUT 'http://localhost:8080/v1/testing?max_hotels=1&max_rooms_per_hotel=6' | jq
```

Both query parameters are optional (`max_hotels` defaults to `3`, maximum `100`;
`max_rooms_per_hotel` defaults to `6`, maximum `1000`). Rooms are rotated across the
three room types.

### 2. Find a hotel by name

```bash
curl -s 'http://localhost:8080/v1/hotels?name_pattern=mercury' | jq
```

### 3. Find available rooms

```bash
curl -s 'http://localhost:8080/v1/rooms?guests_count=2&check_in_date=2026-10-01&check_out_date=2026-10-03' | jq
```

Add `&hotel_id=<uuid>` to narrow the search to one hotel. Rooms already booked, or
occupied by another stay that overlaps the requested dates, are excluded.

### 4. Book a room

Pass the hotel and the party size; the server allocates the rooms and holds each of
them for the whole stay. To book the exact rooms returned by the search in step 3,
add a `room_ids` array - the server then books precisely those rooms (they must
belong to the hotel and still be free for the whole date range).

```bash
HOTEL_ID=$(curl -s 'http://localhost:8080/v1/hotels?name_pattern=mercury' | jq -r '.hotels[0].id')

curl -s -X POST http://localhost:8080/v1/reservations \
  -H 'Content-Type: application/json' \
  -d "{
        \"hotel_id\": \"$HOTEL_ID\",
        \"check_in_date\": \"2026-10-01\",
        \"check_out_date\": \"2026-10-03\",
        \"guest_full_name\": \"Ada Lovelace\",
        \"guest_email\": \"ada@example.com\",
        \"guests_count\": 2
      }" | jq
```

Response:

```json
{
  "id": "28ecc8f7-2180-44ee-b129-923564c234b5",
  "check_in_date": "2026-10-01",
  "check_out_date": "2026-10-03",
  "guest_full_name": "Ada Lovelace",
  "guest_email": "ada@example.com",
  "guests_count": 2,
  "reference": "CYYKSX24PR",
  "status": 2000,
  "hotel_id": "84ee89a6-ec93-450d-830f-e4841edf652f",
  "hotel_details": { "name": "1 Mercury Serenity", "address": "Address 1, Test City" },
  "rooms": [
    {
      "room_id": "638629e7-4e3c-4134-95d5-374516f0da5c",
      "room_label": "Room 5",
      "room_type_caption": "double",
      "room_type_capacity": 2,
      "guests_count": 2
    }
  ]
}
```

The request body may also be sent as `application/x-www-form-urlencoded`.

### 5. Look up the booking

```bash
curl -s http://localhost:8080/v1/reservations/CYYKSX24PR | jq
```

Use the `reference` returned when the booking was created.

### 6. Reset the data

```bash
curl -i -X DELETE http://localhost:8080/v1/testing
```

Returns `204 No Content`. The database is now empty and ready to be seeded again.

### Status codes

| Code | Meaning |
| --- | --- |
| `200` | Success |
| `204` | Reset succeeded |
| `400` | Missing or invalid parameters, dates in the past, `check_out_date` not after `check_in_date` |
| `404` | Unknown hotel, unknown requested room, or unknown booking reference |
| `409` | The party cannot be accommodated, a requested room is no longer free, or a booking reference collision |
| `500` | Unexpected server error |

## Asynchronous confirmation

When a booking is created it is stored with the `pending` status (`2000`). A background
worker confirms pending bookings, and after a two-second delay logs a simulated
confirmation email to the console:

```
{"level":"INFO","msg":"successfully confirmed reservation","reservation_id":"..."}
{"level":"INFO","msg":"email sent for reservation","reservation_id":"..."}
```

Run the API in the foreground to watch these messages. The worker interval is
configurable with `WORKER_INTERVAL`.

## Unit tests and tooling

Tests live next to the code (`*_test.go` in every package) and run through the tasks in
[`Taskfile.yaml`](Taskfile.yaml). Task and mockery are pinned as `tool` dependencies in
`go.mod`, so nothing has to be installed globally:

```bash
go tool task            # list the available tasks
go tool task tests      # run the tests, print per package coverage plus the total
go tool task mocks      # delete the generated mock packages, then regenerate them
```

`tests` prints a coverage line per package and finishes with the overall total for the
hand-written packages, excluding the generated `mock` ones. It leaves no artefacts
behind.

| Layer | Test approach |
| --- | --- |
| repositories | `go-sqlmock` against the statements in `queries.go` |
| services | mockery mocks of the repository interfaces |
| handlers | `httptest` requests through gin with mocked services |
| `internal/app/server` | mocked handlers, storage and worker, with `testing/synctest` for the worker hand-off |
| `internal/helper` | direct table-driven tests |

Mocks are generated by [mockery](https://github.com/vektra/mockery) v3 from
[`.mockery.yml`](.mockery.yml), one `mock` sub-package per mocked package. Regenerate
them after changing an interface: the task deletes the existing `mock` packages first,
so stale mocks cannot survive.

## Project layout

```
cmd/booking-api        application entry point
internal/app/server    configuration, routing and wiring
internal/app/storage   PostgreSQL connection
internal/app/testing   seeding and resetting test data
internal/hotel         hotel search
internal/room          room availability
internal/reservation   booking, allocation and lookup
internal/helper        shared date and pagination helpers
specs/                 OpenAPI document and database schema
```

Each package keeps its tests in `*_test.go` files next to the code, and its generated
mocks in a `mock` sub-package (for example `internal/hotel/service/mock`). Repositories
keep their SQL in `queries.go`, apart from the Go code that runs it.
