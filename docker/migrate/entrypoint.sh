#!/bin/sh
set -eu

: "${MIGRATIONS_PATH:=/migrations/postgres}"
: "${DATABASE_URL:=postgres://flight:flight@postgres:5432/flight_tracker?sslmode=disable}"

cmd="${1:-up}"

case "$cmd" in
up)
    exec migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" up
    ;;
down)
    exec migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" down 1
    ;;
version)
    exec migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" version
    ;;
force)
    if [ -z "${VERSION:-}" ]; then
        echo "VERSION is required, e.g. docker compose --profile migrate run --rm -e VERSION=1 migrate force" >&2
        exit 1
    fi
    exec migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" force "$VERSION"
    ;;
shell)
    exec psql "$DATABASE_URL"
    ;;
*)
    echo "unknown command: $cmd" >&2
    exit 1
    ;;
esac
