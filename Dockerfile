# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
RUN go build -o /server ./cmd/server

# Distroless nonroot is UID/GID 65532. The server reads /server and /migrations
# and listens on 8080; startup migrations write only to Postgres. /tmp is 1777
# for ingest temp files.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=build --chown=65532:65532 /server /server
COPY --chown=65532:65532 migrations /migrations

EXPOSE 8080

ENV DATABASE_DRIVER=postgres
ENV DATABASE_URL=postgres://flight:flight@postgres:5432/flight_tracker?sslmode=disable
ENV MIGRATIONS_PATH=/migrations/postgres

USER 65532:65532

ENTRYPOINT ["/server"]
