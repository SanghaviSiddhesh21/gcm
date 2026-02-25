# cmd

Cobra command definitions. One file per subcommand plus `root.go` for the root command.

## Commands

| File | Command | Notes |
|---|---|---|
| `root.go` | `gcm` | Root command. Version injected via ldflags. Disables default completion command. |
| `init.go` | `gcm init` | Creates `.git/gcm.json` with a fresh store. Fails if already initialized. |
| `create.go` | `gcm create <category>` | Validates name, adds category to store. |
| `assign.go` | `gcm assign <branch> <category>` | Verifies branch exists via git, verifies category exists in store, then assigns. Reports reassignment if branch was previously in a different category. |
| `view.go` | `gcm view [category]` | The most complex command. Gathers branches, sync status, commit times, sorts everything, then delegates to TUI or static renderer. |
| `delete.go` | `gcm delete <category>` | Removes category, reports how many branches were moved to Uncategorized. |
| `categories.go` | `gcm categories` | Lists category names. |
| `commit.go` | `gcm commit` | Passthrough to `git commit`. `-g` flag triggers AI commit TUI (requires a terminal). |
| `config.go` | `gcm config` | Passthrough to `git config`. Intercepts `api-key` args for GCM-managed config in `~/.gcm/config.json`. |

## View sorting logic (`view.go`)

The `sortView` function and its helpers implement a specific display order:
1. Within each category: current branch first, then remaining branches sorted by most recent commit (newest first)
2. Categories: current branch's category first, then other categories by most recent branch commit, Uncategorized always last

This sorting logic lives in `cmd` rather than `ui` because it requires commit-time data from the git layer.

## Pattern

Every command follows the same structure:
1. Call `git.GetRepoInfo()` to locate the repository
2. Call `store.Load()` to read current state (except `init`, which creates a new store)
3. Perform the operation
4. Call `store.Save()` to persist changes (for mutating commands)
5. Print a success message

Error handling: errors are both printed to stderr and returned. Cobra's `SilenceUsage` and `SilenceErrors` are set on the root command, so error display is handled by `main.go`.

## Passthrough pattern

`commit.go` and `config.go` use `DisableFlagParsing: true` and inspect `args` manually. Both commands intercept a specific arg (`-g` for commit, `api-key` for config) and delegate everything else verbatim to the underlying `git` command via `exec.Command`, forwarding stdin/stdout/stderr. This mirrors how `git` itself handles subcommand extensions.

## Gotchas

- The `version` variable in `root.go` defaults to `"dev"` and is overwritten by goreleaser at build time.
- `view.go` imports `mattn/go-isatty` directly rather than through the `ui` package to decide between TUI and static output.
- `commit.go` rejects `-g` when combined with other flags and requires a terminal (`isatty` check) — the commit TUI cannot run in a pipe.
