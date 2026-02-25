# internal/ai

HTTP client for AI-powered commit message generation via a Cloudflare Worker proxy.

## Public API

- **Generator** — interface with a single method: `Generate(ctx context.Context, diff string) (string, error)`
- **New()** — returns a `Generator` configured to call the production Cloudflare Worker
- **ErrNotConfigured** — returned when the AI is unavailable (e.g. worker returned 401/404)
- **ErrGenerationFailed** — returned on unexpected server error or malformed response
- **ErrRateLimited** — returned on HTTP 429; callers should skip retries

## Internal

- **prepareDiff(diff)** — strips context lines, prepends a `Changed files:` summary list, and truncates at 6000 chars. Reduces token usage and focuses the model on signal lines only.
- **extractCommitMessage(raw)** — scans output lines for a conventional commit prefix (`feat:`, `fix:`, etc.); falls back to first non-empty line. Strips surrounding backticks.
- **groqGenerator.url** — injectable for tests; set to `workerURL` by `New()`.

## Gotchas

- The Generator does not retry on failure. Retry logic lives in the TUI (up to 3× on transient errors, 0× on `ErrRateLimited`).
- `prepareDiff` truncates at 6000 chars — for very large diffs the model only sees the beginning. The TUI shows a warning when the staged diff exceeds 500 changed lines.
- The `X-User-Api-Key` header is sent only when `config.GetAPIKey()` succeeds. If no key is configured, the header is omitted and the Worker uses its shared gcm key (rate-limited per IP).

## Testing

`ai_test.go` — 13 tests using `httptest.NewServer` (no real HTTP calls):
- `extractCommitMessage`: conventional type found, backtick-wrapped, first-line fallback, empty input, multi-line raw output, no conventional type present
- `prepareDiff`: empty diff, context lines stripped, `Changed files:` list prepended, truncation at `maxDiffChars`
- `Generate`: success path, HTTP 429 → `ErrRateLimited`, HTTP 500 → `ErrGenerationFailed`, empty choices → `ErrGenerationFailed`, `X-User-Api-Key` header sent when config key exists
