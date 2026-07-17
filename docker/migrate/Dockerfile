# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends make postgresql-client \
    && rm -rf /var/lib/apt/lists/*

RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

ENV PATH="/go/bin:${PATH}"

WORKDIR /app

COPY migrations /migrations
COPY docker/migrate/Makefile ./Makefile

ENV MIGRATIONS_PATH=/migrations/postgres
ENV DATABASE_URL=postgres://flight:flight@postgres:5432/flight_tracker?sslmode=disable

ENTRYPOINT ["make"]
CMD ["up"]
