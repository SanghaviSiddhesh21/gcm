# internal/ai

HTTP client for AI-powered commit message generation via a Cloudflare Worker proxy.

## Public API

- **Generator** — interface with a single method: `Generate(ctx context.Context, diff string, gist []string, summaryPrev []string, attempt int) (string, error)`. `gist` holds commit messages from window-phase regenerations. `summaryPrev` is nil during the window phase and becomes a non-nil slice (possibly empty) once the diff is exhausted. `attempt` drives the 6000-char window offset.
- **IsDiffExhausted(diff, attempt)** — returns true when `attempt × 6000 ≥ len(filtered body)`. Used by the TUI to decide when to enter the summary phase.
- **New()** — returns a `Generator` configured to call the production Cloudflare Worker
- **ErrNotConfigured** — returned when the AI is unavailable (e.g. worker returned 401/404)
- **ErrGenerationFailed** — returned on unexpected server error or malformed response
- **ErrRateLimited** — returned on HTTP 429; callers should skip retries

## Internal

- **filterDiff(diff)** — strips context lines, returns `(files []string, body string)`. Shared by `prepareDiff` and `IsDiffExhausted`.
- **prepareDiff(diff, attempt)** — prepends a `Changed files:` summary and returns the attempt-th 6000-char window of the filtered body. Clamps to the last window once exhausted.
- **extractCommitMessage(raw)** — scans output lines for a conventional commit prefix (`feat:`, `fix:`, etc.); falls back to first non-empty line. Strips surrounding backticks.
- **groqGenerator.url** — injectable for tests; set to `workerURL` by `New()`.

## Gotchas

- The Generator does not retry on failure. Retry logic lives in the TUI (up to 3× on transient errors, 0× on `ErrRateLimited`).
- **Window phase** (diff not exhausted): each 'r' press advances to the next 6000-char window. All prior window messages are included as context ("Previous attempts generated…").
- **Summary phase** (diff exhausted, `summaryPrev != nil`): the model receives all window messages as the "gist of the changes" and is asked to synthesise one commit message. Each subsequent 'r' accumulates the prior summary attempts and adds "however, it/they have not captured the complete essence".
- The TUI shows a large-diff warning when the staged diff exceeds 500 changed lines.
- The `X-User-Api-Key` header is sent only when `config.GetAPIKey()` succeeds. If no key is configured, the header is omitted and the Worker uses its shared gcm key (rate-limited per IP).

## Testing

`ai_test.go` — 23 tests using `httptest.NewServer` (no real HTTP calls):
- `extractCommitMessage`: conventional type found, backtick-wrapped, first-line fallback, empty input, multi-line raw output, no conventional type present
- `prepareDiff`: context lines stripped, `Changed files:` list prepended, truncation at window boundary, sliding window (window 0 ≠ window 1 for large diffs), window clamp (attempt beyond diff length returns last window)
- `IsDiffExhausted`: false at attempt 0 for small diff, true at attempt 1; false at attempt 0/1 for large diff, true at attempt 4
- `Generate` (window phase with gist): prompt contains "Previous attempts generated" and the gist messages, not summary-phase language
- `Generate` (summary phase): gist-only prompt; single prior summary ("it has not captured"); multiple prior summaries ("they have not captured")
- `Generate` (HTTP): success path, HTTP 429 → `ErrRateLimited`, HTTP 500 → `ErrGenerationFailed`, empty choices, bad JSON, network error, `X-User-Api-Key` header
