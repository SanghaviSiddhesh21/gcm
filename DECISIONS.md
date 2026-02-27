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

## gcm is a drop-in replacement for git (passthrough architecture)

`gcm` acts as a full `git` wrapper. Any command not natively defined in gcm is forwarded verbatim to `git` via `passthroughGit`, which wires `stdin`/`stdout`/`stderr` directly so interactive flows (pagers, prompts, editors) work correctly. This is implemented by setting `DisableFlagParsing: true` and `Args: cobra.ArbitraryArgs` on `rootCmd`, intercepting `--version`/`-v` and `--help`/`-h` explicitly, and forwarding everything else.

## Security denylist for `-c` config injection

When forwarding to git, `gcm` rejects `-c key=val` pairs where the key could cause git to execute an arbitrary binary (e.g. `core.hookspath`, `core.editor`, `diff.external`, `filter.*.clean`). The denylist is applied at both the `main.go` global-flag parse stage and inside `checkPassthroughArgs`. Keys that are legitimate in CI/agent automation (`core.sshCommand`, `credential.helper`) are deliberately not denied. `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`/`GIT_CONFIG_VALUE_n` env vars (which bypass `-c` checking entirely) are stripped from the subprocess environment.

## Global git flags threaded through both `cmd` and `internal/git`

Git global flags (`-C`, `--git-dir`, `--work-tree`, `-c`) are stripped from `os.Args` in `main.go` before Cobra dispatch. They are distributed to two places: `cmd.SetGlobalGitFlags` (for passthrough subprocesses) and `git.SetGlobalFlags` (for internal `runGit` calls). This ensures flags like `--git-dir=/custom/.git` are honoured both for passthrough commands and for gcm's own git queries.

## `store.LoadOrCreate` instead of requiring explicit `gcm init`

`store.LoadOrCreate` creates a fresh store if none exists, instead of returning `ErrNotInitialized`. All commands (view, assign, create, delete, etc.) use `LoadOrCreate`, so gcm works out-of-the-box in any existing git repo without requiring `gcm init`. `gcm init` still exists but now runs `git init` first, making it useful for new repos.

## `gcm branch` syncs the assignment map on rename and delete

`gcm branch -m old new` updates the branch assignment in `gcm.json` after the git rename succeeds. `gcm branch -d branch` removes the assignment after the git delete succeeds. Failures in store sync do not fail the overall command — the git operation already succeeded and is not reversible.

## Passthrough uses `c.Run()` without `Setpgid`

On Unix, `passthroughGit` uses a plain `c.Run()` with no `Setpgid` or manual signal forwarding. An earlier implementation used `Setpgid: true` to isolate git in its own process group and forwarded signals via a goroutine. This was removed because `Setpgid` places git outside the terminal's foreground process group, causing git's pager (`less`) and any editor children to receive `SIGTTIN` and stop when they try to read from the TTY — freezing `gcm log`, `gcm diff`, `gcm branch`, and `gcm rebase -i`. Without `Setpgid`, git and its children stay in the terminal's foreground process group, Ctrl+C is delivered by the kernel to the entire group automatically, and pagers and editors work correctly.

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

## Inline `//nolint` over global exclusions for lint suppressions

When suppressing a lint warning, prefer an inline `//nolint:linter` comment at the specific line over adding a global exclusion to `.golangci.yml`. This keeps the suppression scoped to exactly the line where it is intentional and documents the reason inline, rather than silencing the rule everywhere in the codebase where it might catch legitimate issues.

Global exclusions in `.golangci.yml` are reserved for rules that are intentionally inapplicable to the entire project (e.g. G204 subprocess variable — expected in a git wrapper, G117 secret field pattern — expected in config storage).

## Anonymous telemetry via Cloudflare Worker → PostHog

`gcm` collects anonymous usage telemetry (command invocations, outcomes, regeneration counts) to inform prioritization. Events are routed through a Cloudflare Worker so the PostHog project write key never appears in the binary. This matches the architecture already used for AI commit messages. Telemetry is suppressed automatically in CI environments (`CI`, `GITHUB_ACTIONS`) — the recorder degrades to a no-op with zero overhead. There is no user-facing opt-out.

## `Recorder` interface with `noop{}` default

The `telemetry.Recorder` interface (`Record`, `Flush`) is always non-nil. When telemetry is disabled (env vars, empty install ID, or empty worker URL), `telemetry.New()` returns `noop{}` rather than `nil`. This eliminates nil checks throughout `cmd`. The package-level `cmdTel` variable in `cmd/root.go` is initialized to `noop{}` so commands never panic even if `Execute(tel)` is somehow called before assignment.

## `run(tel) int` pattern for guaranteed Flush

`main.go` calls `tel.Flush()` after `run(tel)` returns and before `os.Exit(code)`. This guarantees flush executes even when commands return errors. The alternative — deferring Flush in `main` — would work, but the explicit `run()` wrapper makes the flow visible and lets `main` stay minimal.

## Non-blocking Flush with 500ms hard cap

`Flush` dispatches the HTTP POST in a goroutine with a 400ms context deadline and waits at most 500ms via `time.After`. The binary must not hang on shutdown even if the Worker is unreachable. Events are silently dropped on timeout — acceptable for analytics data.

## Anonymous install ID stored in `~/.gcm/config.json`

The install ID is a UUIDv4 stored alongside the API key in `~/.gcm/config.json`. A UUID provides the cardinality needed to count unique users without linking to any identity. Storing it in the existing config file avoids creating a new file, keeps all user-local state in one place, and reuses the existing atomic-write path. The `google/uuid` package was chosen for its RFC 4122 compliance and zero CGo.

## CI environment suppresses telemetry automatically

Checking `CI` and `GITHUB_ACTIONS` means automated pipelines never generate telemetry or install IDs. The same check is duplicated in both `internal/config` (`isDisabledEnv`) and `internal/telemetry` (`isDisabled`) to avoid a circular import. There is no user-facing opt-out — telemetry is always on for end users. Integration tests set `CI=1` in `TestMain` to suppress telemetry during test runs.

## HTTP transport isolated in `telemetry/http.go`

The actual HTTP POST logic lives in a separate `http.go` file. This allows `.testcoverage.yml` to exempt only that file rather than the entire `telemetry` package. The `poster` function type enables white-box unit tests to inject a mock without exposing any test-only exports.

## `sync.Once` cache in `internal/config`

`config.load()` now caches the parsed `file{}` in a `sync.Once` to avoid repeated disk reads within a single `gcm` invocation. The cache is write-through: `SetAPIKey`, `UnsetAPIKey`, and `GetOrCreateInstallID` update `cachedFile` in-memory after writing to disk. Tests reset `configOnce = sync.Once{}` in `withTempHome` to ensure isolation.

## Coverage threshold lowered after telemetry feature

The `make coverage-check` threshold was lowered from 67% to 58% after the telemetry feature (v0.1.10). The feature added new command-runner functions (`runCategories`, per-command telemetry wrappers, etc.) that are correctly exempt from unit tests per the testing contract — they orchestrate I/O and git calls and are covered exclusively by binary integration tests (`cmd/*_test.go`). Binary integration tests run the compiled gcm binary as a subprocess, so their coverage is not captured in `coverage.out`. Adding the exempt code without adding proportional measured coverage lowered the overall percentage below the 67% ratchet. The threshold was adjusted to reflect the true state of measured (non-exempt) logic coverage, and will ratchet upward as new testable logic is added.

## golangci-lint exclusions

- `G204` (subprocess with variable): Expected — the tool shells out to git by design.
- `G304` (file path from variable): The path is always `.git/gcm.json`, constructed from `git rev-parse` output, not from arbitrary user input.
- `errcheck` and `gosec` disabled for test files.
- `revive.exported` disabled: No godoc requirement on exported symbols.
