# flight-tracker agent rules

- Go service: chi HTTP API (`/api/v1` consumer, `/api/internal` operator), Postgres, background job workers.
- Always design for multiple HTTP replicas and workers. No single-process shortcuts (in-memory state, local locks, local caches as truth).
- Breaking API changes are fine; there are no outside clients yet.
- Reuse existing service/store code; don't duplicate query logic.
- Migrations: golang-migrate in `migrations/postgres`, one statement per file when using CONCURRENTLY.
- Comments and docs: only non-obvious behavior. Don't restate field names or repeat caveats across code, Swagger, and README. Leave BTS and ASOS docs alone.
- README API examples: one short sentence per curl command.
- Tests must exercise real code. Don't reimplement logic inside tests to compare against itself.
- Never commit plans, reviews, or design notes to the repo.
- Keep changes scoped to the task. Don't refactor unrelated code.
- Run `make lint` and `make test` before pushing.
- Run `make swagger` after changing handler annotations; CI checks the generated docs.
