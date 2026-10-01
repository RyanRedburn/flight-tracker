# Pin must match //go:generate in cmd/server/main.go and CI swagger workflow.
swag_version := "v1.16.6"

# Pin must match .github/workflows/lint.yml. go run rebuilds with the module toolchain (needed for Go 1.26+).
golangci_lint_version := "v2.12.2"

coverage_out := "coverage.out"
coverage_html := "coverage.html"

lint:
    go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{ golangci_lint_version }} run

# Regenerate external + full (internal) OpenAPI docs via go:generate.
swagger:
    go generate ./cmd/server/...

test:
    go test ./...

docker-build:
    docker compose build

# postgres + app only. The migrate sidecar is behind the "migrate" profile so
# default `up` does not race the app's on-open migrations.
docker-run:
    docker compose up

# Run migrations via the migrate sidecar (distroless app image has no shell).
migrate-up:
    docker compose --profile migrate run --rm migrate up

migrate-down:
    docker compose --profile migrate run --rm migrate down

migrate-version:
    docker compose --profile migrate run --rm migrate version

db-shell:
    docker compose --profile migrate run --rm -it migrate shell

test-cover-html:
    go test -coverprofile={{ coverage_out }} ./...
    go tool cover -html={{ coverage_out }} -o {{ coverage_html }}
    @echo "Coverage report: {{ coverage_html }}"

clean-cover:
    rm -f {{ coverage_out }} {{ coverage_html }}
