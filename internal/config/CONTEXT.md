# internal/config

Reads and writes user configuration to `~/.gcm/config.json`.

## Public API

- **GetAPIKey() (string, error)** — returns the stored API key. Returns `ErrNotSet` if absent or empty.
- **SetAPIKey(key string) error** — writes the key to disk. Creates `~/.gcm/` (mode 0700) if it does not exist.
- **UnsetAPIKey() error** — clears the key (writes empty string; `omitempty` removes it from the JSON file).
- **ErrNotSet** — sentinel error returned by `GetAPIKey` when no key has been configured.

## Internal

- Config schema: `{ "api_key": "<value>" }` — the field is `omitempty`, so absent and empty are equivalent.
- File permissions: 0600 (user read/write only). Directory: 0700.
- `load()` returns an empty `file{}` with no error when `config.json` does not exist — a missing file is a valid "nothing configured" state.

## Gotchas

- Config lives at `~/.gcm/config.json`, outside the repo. It is never tracked by git and needs no `.gitignore` entry.
- No environment variable fallback. The only supported mechanism is `gcm config api-key`.

## Testing

`config_test.go` — 7 tests, each using `t.Setenv("HOME", t.TempDir())` for full isolation:
- `GetAPIKey` returns `ErrNotSet` when key is absent
- `GetAPIKey` returns `ErrNotSet` when config file does not exist
- `SetAPIKey` + `GetAPIKey` round-trip
- `SetAPIKey` overwrites an existing key
- `UnsetAPIKey` clears the key (subsequent `GetAPIKey` returns `ErrNotSet`)
- `UnsetAPIKey` on a non-existent config file does not error
- Written file has 0600 permissions
