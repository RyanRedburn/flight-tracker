# Style

`.golangci.yml` is authoritative. `make lint` uses golangci-lint v2.12.2 via `go run` (same pin as CI).

## Imports (`gci`)

Three groups, blank lines between them, project module before third-party:

```go
import (
	"context"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
)
```

## Whitespace (`wsl_v5`)

Cuddle `if err != nil` with the assignment it checks. Keep an empty line between independent statements. Match the file you are editing; if lint disagrees, follow lint.

## Errors

- Wrap with `fmt.Errorf("verb: %w", err)` at package boundaries.
- HTTP **500** bodies use stable, client-safe strings (see `internal/api/handlers/errors.go`). Do not leak driver/internal details.
- Sentinel errors live next to the package that owns them (`store.ErrNotFound`, `ingest.ErrRangeTooLarge`, …). Handlers use `errors.Is`.

## Naming

- JSON and query params are `snake_case`.
- Store SQL constants are `Query*` in `internal/store/queries.go`.
- Job types are `import_*` strings on `internal/model`.
- Avoid magic strings that `goconst` will flag; lift repeats into package-level consts (see `handlers/errors.go`, ingest column names).

## Functions

Do not create one-line wrapper functions with no real utility. Call the underlying function, or inline the expression, instead of adding `func foo(...) { return bar(...) }` (or equivalent) that only exists to rename, forward, or "layer" a call.

Shims are allowed: adapters that satisfy an interface, inject a test seam (`CSVOpener`, `storetest.Stub` methods), or translate a third-party/stdlib type at a package boundary.

## Control flow

Prefer early returns over nested `if`. `nestif` is enabled. Switches on enums should be exhaustive; `default-signifies-exhaustive` is on.

## Other enabled linters to respect

- `errcheck`, `govet`, `staticcheck`, `unused`, `gosec`
- `exhaustive`, `gocritic`, `decorder`, `grouper`, `iface`, `misspell`, `predeclared`
- `perfsprint` — prefer `strconv` over `fmt` for simple conversions
- Formatters: `gofmt`, `goimports`, `gci`

Do not add testify. Tests use the standard library (`testifylint` is enabled only so testify, if ever added, is used correctly).
