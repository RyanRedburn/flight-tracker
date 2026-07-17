# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build

# CGO remains until the SQLite store package is removed (later phase).
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
RUN go build -o /server ./cmd/server

FROM gcr.io/distroless/base-debian12

WORKDIR /

COPY --from=build /server /server
COPY migrations /migrations

EXPOSE 8080

ENV DATABASE_DRIVER=postgres
ENV DATABASE_URL=postgres://flight:flight@postgres:5432/flight_tracker?sslmode=disable
ENV MIGRATIONS_PATH=/migrations/postgres

ENTRYPOINT ["/server"]
