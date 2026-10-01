# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl postgresql-client \
    && rm -rf /var/lib/apt/lists/*

# Same pin as CI (extractions/setup-just just-version).
ARG JUST_VERSION=1.58.0
RUN arch="$(dpkg --print-architecture)" \
    && case "$arch" in \
        amd64) target=x86_64-unknown-linux-musl ;; \
        arm64) target=aarch64-unknown-linux-musl ;; \
        *) echo "unsupported architecture: $arch" >&2; exit 1 ;; \
    esac \
    && curl -fsSL "https://github.com/casey/just/releases/download/${JUST_VERSION}/just-${JUST_VERSION}-${target}.tar.gz" \
        | tar -xz -C /usr/local/bin just \
    && just --version

RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

ENV PATH="/go/bin:${PATH}"

WORKDIR /app

COPY migrations /migrations
COPY docker/migrate/justfile ./justfile

ENV MIGRATIONS_PATH=/migrations/postgres
ENV DATABASE_URL=postgres://flight:flight@postgres:5432/flight_tracker?sslmode=disable

ENTRYPOINT ["just"]
CMD ["up"]
