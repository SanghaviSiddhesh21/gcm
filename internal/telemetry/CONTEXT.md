# internal/telemetry

Anonymous usage telemetry — in-memory event queue with a non-blocking flush to a Cloudflare Worker that forwards events to PostHog.

## Public API

- **Recorder** — interface with two methods:
  - `Record(event string, props map[string]any)` — enqueue an event; never blocks.
  - `Flush()` — send all queued events and wait at most 500ms; safe to call on every exit.
- **New(installID, workerURL, version string) Recorder** — returns a real client if telemetry is enabled and both `installID` and `workerURL` are non-empty; otherwise returns `Noop()`.
- **Noop() Recorder** — returns a no-op recorder (`noop{}`). Zero overhead, never panics.

## Telemetry suppression

`New()` returns `Noop()` when `CI` or `GITHUB_ACTIONS` env vars are set (automated pipeline environments should never generate telemetry). An empty `installID` or `workerURL` also yields `Noop()`. This check is in `isDisabled()` — duplicated from `internal/config.isDisabledEnv()` to avoid a circular import.

## Standard properties merged on every event

`Record` merges these into every event's props map:
- `distinct_id` — the anonymous install ID (UUIDv4)
- `os` — `runtime.GOOS`
- `arch` — `runtime.GOARCH`
- `gcm_version` — the version string passed to `New()`

Callers only need to supply event-specific props.

## Flush behavior

- Pending events are drained under a mutex into a local slice.
- A goroutine posts the slice via `defaultPoster` with a 400ms context deadline.
- The outer `Flush` call waits at most 500ms via `time.After`.
- Events are silently dropped on timeout — acceptable for analytics data.

## File layout

| File | Contents |
|---|---|
| `telemetry.go` | `Recorder` interface, `noop{}`, `client`, `New()`, `Noop()`, `isDisabled()`, `Record()`, `Flush()` |
| `http.go` | `event` struct, `poster` function type, `defaultPoster` (HTTP POST to worker URL) |
| `telemetry_test.go` | 11 white-box tests (package `telemetry`, not `telemetry_test`) |

`http.go` is excluded from `.testcoverage.yml` — it is a thin HTTP wrapper with no testable logic beyond what `defaultPoster` tests via `httptest` would duplicate.

## Testing

`telemetry_test.go` — 9 tests using mock `poster` injection:
- `CI`/`GITHUB_ACTIONS` → `noop{}`
- Empty `installID` or `workerURL` → `noop{}`
- `Noop()` — `Record` and `Flush` are no-ops, no panic
- `Record` enqueues events with standard properties merged
- `Flush` with empty queue → no HTTP call
- `Flush` delivers events to mock poster
- `Flush` hard cap: poster sleeping 2s returns in under 750ms
- `Flush` clears the pending queue

## Gotchas

- `internal/telemetry` must not import `internal/config` (circular dep via `main.go`). The CI suppression check is therefore duplicated.
- `Recorder` is always non-nil — callers never need a nil guard.
- `Record` is safe to call concurrently; `Flush` is intended to be called once on exit.
