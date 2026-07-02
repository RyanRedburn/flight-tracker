# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build

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
VOLUME ["/data"]

ENV DATABASE_DRIVER=sqlite
ENV DATABASE_URL=file:/data/flight-tracker.db
ENV MIGRATIONS_PATH=/migrations/sqlite

ENTRYPOINT ["/server"]
