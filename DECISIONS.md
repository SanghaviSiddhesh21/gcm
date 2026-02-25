# Technical Decisions

## Storage: JSON file inside `.git/`

The store lives at `.git/gcm.json`. This keeps category metadata local and invisible to git (the `.git/` directory is never tracked). No database, no config files in the repo root, no `.gitignore` entries needed. The trade-off is that categories are per-clone and not shareable — this is intentional, since branch organization is personal.

## Atomic writes via temp file + rename

`store.Save` writes to `gcm.json.tmp` then renames to `gcm.json`. This prevents corruption if the process is interrupted mid-write. The rename is atomic on all target platforms (Linux, macOS, Windows NTFS).

## No ORM, no embedded database

The entire state fits in a single small JSON file. Using SQLite or similar would add complexity for no benefit. The store is loaded into memory, mutated, and written back as a whole.

## Sentinel errors over error wrapping in store

The `store` package defines sentinel errors (`ErrNotInitialized`, `ErrCategoryNotFound`, etc.) rather than wrapping dynamic errors. This makes error identity checks in the `cmd` layer simple and predictable.

## Implicit "Uncategorized" category

Branches not present in the `assignments` map are implicitly Uncategorized. The Uncategorized category is always present, marked as immutable, and cannot be deleted or created by the user. This avoids requiring every branch to have an explicit assignment.

## Shell out to `git` binary, no libgit2 bindings

All git operations exec the `git` CLI. This avoids CGo, simplifies cross-compilation (goreleaser builds for 6 targets), and guarantees compatibility with the user's installed git version. The `gosec` G204 exclusion in `.golangci.yml` acknowledges this choice.

## TUI vs static output based on terminal detection

`gcm view` uses `go-isatty` to check if stdout is a terminal. Terminal → Bubbletea interactive TUI (navigate, collapse categories, checkout branches). Piped → static colored tree via `fatih/color`. This makes the tool pipeline-friendly while providing a rich interactive experience in terminals.

## View sorting: current branch's category first, Uncategorized last

The `view` command applies a specific sort order:
1. Current branch appears first within its category
2. The category containing the current branch appears first
3. Other categories sorted by most recent branch commit time
4. Uncategorized always last (unless it contains the current branch)

This is implemented in `cmd/view.go`, not in the UI layer. (inferred) The sorting lives in `cmd` rather than `ui` because it requires git data (commit times) that the UI layer does not own.

## Dirty worktree confirmation before checkout

The TUI checks worktree status before switching branches. If there are uncommitted changes, it shows a confirmation prompt listing staged/unstaged/untracked files. This prevents accidental data loss without being as strict as git's default checkout behavior.

## Category name validation: alphanumeric + hyphens only

Category names must match `^[a-zA-Z0-9-]+$`, max 64 characters. Underscores are explicitly disallowed. (inferred) This may be to avoid ambiguity with git branch naming conventions, but the exact rationale is unclear — the README mentions underscores are allowed but the validation regex rejects them.

**Note:** The README says category names "may contain `-` or `_`" but the actual validation in `store.ValidateCategoryName` rejects underscores. This is a documentation-code mismatch that should be resolved.

## No concurrency

The tool is entirely synchronous and single-threaded. The Bubbletea TUI uses its own event loop but all git operations and store mutations are sequential. There is no shared state, no goroutine pools, no channels beyond what Bubbletea uses internally.

## AI via Cloudflare Worker proxy, not direct Groq API

`gcm commit -g` sends diffs to a Cloudflare Worker rather than calling the Groq API directly. This lets the tool work out-of-the-box without requiring users to supply a Groq key — the Worker uses a shared gcm key with KV-backed rate limiting (3 req/min, 20 req/day per IP). Users who hit rate limits can supply their own key via `gcm config api-key`.

## `~/.gcm/config.json` for user config, no env vars

The API key is stored in `~/.gcm/config.json` (file mode 0600, directory mode 0700) and managed exclusively via `gcm config api-key`. Env var support was explicitly dropped to keep the config path single and consistent. The file lives outside the repo and needs no `.gitignore` entry.

## Rate limiting in Cloudflare Worker with user key bypass

The Worker enforces per-IP rate limits via Cloudflare KV when using the shared gcm key. If the request includes a valid `X-User-Api-Key` header, that key is used directly and no rate limiting applies. The Worker returns HTTP 429 on limit exceeded; the TUI handles this by skipping retries entirely and falling to manual input with a message directing the user to set their own key.

## Lefthook for pre-push hooks

The project uses Lefthook (not Husky or pre-commit) for git hooks. The pre-push pipeline runs fmt → build → test → coverage → lint in sequence. (inferred) Lefthook was likely chosen for its Go-native ecosystem fit and simple YAML configuration.

## golangci-lint exclusions

- `G204` (subprocess with variable): Expected — the tool shells out to git by design.
- `G304` (file path from variable): The path is always `.git/gcm.json`, constructed from `git rev-parse` output, not from arbitrary user input.
- `errcheck` and `gosec` disabled for test files.
- `revive.exported` disabled: No godoc requirement on exported symbols.
