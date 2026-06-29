# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev make sqlite3 \
    && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1
RUN go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

ENV PATH="/go/bin:${PATH}"

WORKDIR /app

COPY migrations /migrations
COPY docker/migrate/Makefile ./Makefile

ENV MIGRATIONS_PATH=/migrations/sqlite
ENV DATABASE_URL=sqlite3:///data/flight-tracker.db

ENTRYPOINT ["make"]
CMD ["up"]
