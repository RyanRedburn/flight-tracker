---
name: mcp-tools
description: Add or change flight-tracker MCP tools served at /mcp, their input schemas, structured results, and tool errors. Use when working on internal/mcp, adding a tool, or changing what an assistant sees from the MCP server.
---

# MCP tools

Also follow [development-practices](../development-practices/SKILL.md).

## Layout

- `internal/mcp/server.go` builds the server and the `/mcp` handler. Register each tool in `newServer` with an `addX(server, s, logger)` call.
- One file per tool (`find_airport.go`): input struct, schema, `addX`, and a plain `x(ctx, s, logger, in)` function holding the logic.
- `internal/api/server.go` mounts `/mcp` behind the API key check and the `mcp` rate-limit surface (`ExternalRoles`). Tools do no auth of their own.
- Tools call existing `store.Store` methods. Never put SQL or a second copy of query logic in `internal/mcp`; if a tool needs new data, add a store method (see [store-and-migrations](../store-and-migrations/SKILL.md)).

## Results

Every tool returns `Result[T]` from `result.go` through `successResult` or `failureResult`.

- `summary` is one short line the assistant can act on. State on-time odds and how many flights they come from when that is the answer, or say when the data is missing or thin. Don't repeat the confidence value, and don't append a caveat.
- `confidence` uses the `model.Confidence*` values.
- Failures carry a stable `Code*` constant and a message that tells the assistant what to do next. Add close `matches` instead of making it guess. Every result carries the same JSON in text and structuredContent.

Bad input, no match, and store errors are tool errors (`IsError` result, nil Go error) so the model can recover. Log store errors and return a generic `CodeUnavailable` message. Only return a Go error for a real protocol failure.

## Input schema

Generate the schema from the input struct, then set `Minimum`, `Maximum`, and `MaxLength` on it (see `findAirportInputSchema`) so clients see the limits. Keep the same checks in the tool function too. The SDK rejects out-of-range values first with a generic message; the tool's checks cover direct calls and give the friendly text. Share limit constants with the store (`store.AirportSearch*`).

## Annotations

Set `ReadOnlyHint: true` and `OpenWorldHint: false` (closed world: only our database). Give the tool a `Title` and a description that says when to use it.

## Transport

The handler is stateless streamable HTTP with JSON responses. No sessions and no in-process state between calls, so any replica can serve any request. It's wrapped in `http.NewCrossOriginProtection`, which returns 403 to foreign browser origins and lets non-browser clients through. `/mcp` is not a Swagger route.

## Tests

Use `storetest.Stub` and the in-package helpers in `find_airport_test.go` (`connectServer`, `callFindAirport`, `decodeResult`), which run through the in-memory transport. Cover each failure code, the happy path, the schema limits, and annotations. Never test against a real database; integration tests are prohibited.
