# Testing

`make test` runs `go test ./...`. There is no live-Postgres integration suite. CI is unit tests only.

## Style

- Standard library `testing` only. No testify, no `require`.
- `t.Fatalf` when the rest of the test cannot proceed; `t.Errorf` when later assertions can still run.
- Table-driven tests for permutations (`t.Run` subtests). Named fields in the table struct.
- Keep tests next to the code (`foo_test.go` in the same package).

## Test the current contract

Do not add unit tests whose purpose is to prove that old functionality no longer exists. Drop or rewrite assertions that encoded the old contract; assert the current shape and behavior instead.

## Store stub

Unit tests that need a database use `internal/store/storetest.Stub`:

- Set only the `*Fn` fields the scenario should call.
- Unset Fns **panic** — that is intentional so missing setup fails loudly.
- Return `store.ErrNotFound` (and other sentinels) from Fns when testing handler mappings.

When adding a `store.Store` method, add the Fn field and forwarding method on `Stub` in the same PR.

## HTTP tests

Use `httptest.NewRequest` / `NewRecorder`. For path params, attach `chi.NewRouteContext()` the same way `internal/api/handlers/handlers_test.go` does.

Assert status code first, then decode JSON into the exported response type when one exists.

## Ingest tests

- Fixtures live in `internal/ingest/<provider>/testdata/`. Small CSVs only; they are not production inputs. They should look like production data to the extent that is reasonably possible (real column names, value shapes, and enough row diversity to exercise parsers) so tests stay robust.
- Inject `WithCSVOpener` (or equivalent) so tests never hit BTS/IEM/OurAirports.
- HTTP download tests should use `httptest.NewServer`, not the public internet.

## Postgres package tests

`internal/store/postgres/*_test.go` tests SQL builders, COPY encoding, and helpers that assemble or execute query logic. Do not introduce tests that require a running database unless the user explicitly asks for integration tests.

Do **not** write unit tests whose only purpose is to assert raw SQL query string constants in `internal/store/queries.go` (or equivalent `Query*` consts). No `queries_test.go`-style checks such as `strings.Contains(QueryFoo, "…")`. Still OK: postgres package tests for SQL builders, COPY encoding, and helpers; `storetest` stubs; handler and operator behavior tests.
