# internal/config

Reads and writes user configuration to `~/.gcm/config.json`.

## Public API

- **GetAPIKey() (string, error)** — returns the stored API key. Returns `ErrNotSet` if absent or empty.
- **SetAPIKey(key string) error** — writes the key to disk. Creates `~/.gcm/` (mode 0700) if it does not exist.
- **UnsetAPIKey() error** — clears the key (writes empty string; `omitempty` removes it from the JSON file).
- **GetOrCreateInstallID() (string, error)** — returns the existing anonymous install ID, or generates and persists a new UUIDv4 if none is stored. Returns `("", nil)` when telemetry is suppressed via env var.
- **ErrNotSet** — sentinel error returned by `GetAPIKey` when no key has been configured.

## Internal

- Config schema: `{ "api_key": "<value>", "install_id": "<uuid>" }` — both fields are `omitempty`, so absent and empty are equivalent.
- File permissions: 0600 (user read/write only). Directory: 0700.
- `loadFromDisk()` reads the file; `load()` wraps it in a `sync.Once` cache.
- `load()` returns an empty `file{}` with no error when `config.json` does not exist — a missing file is a valid "nothing configured" state.
- `save()` writes atomically via temp-file + rename (`config.json.tmp` → `config.json`), matching the pattern in `internal/store`.
- `isDisabledEnv()` returns true when `CI` or `GITHUB_ACTIONS` is set. Used by `GetOrCreateInstallID` to avoid generating or persisting an ID in automated pipeline environments.

## Caching

`load()` uses a package-level `sync.Once` (`configOnce`) to cache the parsed `file{}` on first access. Write operations (`SetAPIKey`, `UnsetAPIKey`, `GetOrCreateInstallID`) update `cachedFile` in-memory after writing to disk (write-through cache), so subsequent reads within the same process reflect the new value without re-reading the file.

Tests reset `configOnce = sync.Once{}` inside `withTempHome` to ensure isolation between test cases.

## Gotchas

- Config lives at `~/.gcm/config.json`, outside the repo. It is never tracked by git and needs no `.gitignore` entry.
- No environment variable fallback for the API key. The only supported mechanism is `gcm config api-key`.
- `isDisabledEnv()` CI check is duplicated in `internal/telemetry` to avoid a circular import (`main.go` imports both packages).

## Testing

`config_test.go` — 11 tests, each using `withTempHome` (sets `HOME` to a temp dir and resets `configOnce`) for full isolation:
- `GetAPIKey` returns `ErrNotSet` when key is absent
- `GetAPIKey` returns `ErrNotSet` when config file does not exist
- `SetAPIKey` + `GetAPIKey` round-trip
- `SetAPIKey` overwrites an existing key
- `UnsetAPIKey` clears the key (subsequent `GetAPIKey` returns `ErrNotSet`)
- `UnsetAPIKey` on a non-existent config file does not error
- Written file has 0600 permissions
- `GetOrCreateInstallID` returns `""` when `CI` is set (no install ID generated in pipelines)
- `GetOrCreateInstallID` generates a UUID and persists it; second call returns same UUID
- `GetOrCreateInstallID` does not overwrite an existing API key
- `save` writes atomically (temp file then rename)
