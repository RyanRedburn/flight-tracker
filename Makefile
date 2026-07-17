.PHONY: lint test test-cover-html docker-build docker-run migrate-up migrate-down migrate-version db-shell clean-cover swagger

# Pin must match //go:generate in cmd/server/main.go and CI swagger workflow.
SWAG_VERSION := v1.16.6

lint:
	golangci-lint run

# Regenerate external + full (internal) OpenAPI docs via go:generate.
swagger:
	go generate ./cmd/server/...

test:
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

db-shell:
	docker compose run --rm -it migrate shell

COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html

test-cover-html:
	go test -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "Coverage report: $(COVERAGE_HTML)"

clean-cover:
	rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)
