.PHONY: lint test test-cover-html docker-build docker-run migrate-up migrate-down migrate-version db-shell clean-cover swagger

# Pin must match //go:generate in cmd/server/main.go and CI swagger workflow.
SWAG_VERSION := v1.16.6

# Pin must match .github/workflows/lint.yml. go run rebuilds with the module toolchain (needed for Go 1.26+).
GOLANGCI_LINT_VERSION := v2.12.2

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

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

# Run migrations via the migrate sidecar (distroless app image has no shell/make).
migrate-up:
	docker compose --profile migrate run --rm migrate up

migrate-down:
	docker compose --profile migrate run --rm migrate down

migrate-version:
	docker compose --profile migrate run --rm migrate version

db-shell:
	docker compose --profile migrate run --rm -it migrate shell

COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html

test-cover-html:
	go test -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "Coverage report: $(COVERAGE_HTML)"

clean-cover:
ifeq ($(OS),Windows_NT)
	-del /Q $(COVERAGE_OUT) $(COVERAGE_HTML)
else
	rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)
endif
