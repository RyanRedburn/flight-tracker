.PHONY: lint test test-all test-cover-html docker-build docker-run migrate-up migrate-down migrate-version db-seed db-shell clean-cover

# SQLite driver requires CGO; export works on Unix and Windows (GNU Make).
export CGO_ENABLED := 1

# Packages that do not import the SQLite driver (no C compiler required).
TEST_UNIT_PKGS := ./internal/config/... ./internal/operator/... ./internal/api/... ./internal/ingest/... ./internal/store/mem/...

lint:
	golangci-lint run

# Cross-platform unit tests (no C compiler required).
test:
	go test $(TEST_UNIT_PKGS)

# Full suite including SQLite integration tests (requires a C compiler).
test-all:
	go test ./...

docker-build:
	docker compose build

docker-run:
	docker compose up

# Run migrations via the migrate sidecar (distroless app image has no shell/make).
migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down

migrate-version:
	docker compose run --rm migrate version

db-seed:
	docker compose run --rm migrate seed-flights

db-shell:
	docker compose run --rm -it migrate shell

COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html

# Full test suite (requires CGO for SQLite) with HTML coverage report.
test-cover-html:
	go test -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "Coverage report: $(COVERAGE_HTML)"

clean-cover:
	rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)
