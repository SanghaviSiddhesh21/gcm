# cmd

Cobra command definitions. One file per subcommand plus `root.go` for the root command.

## Commands

| File | Command | Notes |
|---|---|---|
| `root.go` | `gcm` | Root command. `DisableFlagParsing: true`; intercepts `--version`/`-v` and `--help`/`-h`, then passes unknown subcommands to git. Version injected via ldflags. Exports `Execute(tel)` and `Version()`. Holds package-level `cmdTel telemetry.Recorder`. |
| `passthrough.go` | — | Shared security infrastructure: `globalGitFlags`, `checkHelp`, `IsDeniedConfigKey`, `checkPassthroughArgs`, `buildPassthroughEnv`, `IsSecurityDenied`. |
| `passthrough_unix.go` | — | Unix `passthroughGit`: plain `c.Run()` (no `Setpgid`; git stays in terminal's foreground process group so pagers and editors work). `passthroughGitHelp` injects `--no-man` in non-TTY. |
| `passthrough_windows.go` | — | Windows `passthroughGit`: plain `c.Run()`. |
| `init.go` | `gcm init` | Runs `git init [args...]`, then calls `store.LoadOrCreate`. Auto-detects target directory. |
| `clone.go` | `gcm clone` | Runs `git clone [args...]`, then calls `store.LoadOrCreate` in the cloned repo. Infers target directory via `cloneTargetDir`. |
| `branch.go` | `gcm branch` | Passthrough to `git branch`. After `-m`/`-M` rename: calls `store.RenameBranch`. After `-d`/`-D` delete: calls `store.UnassignBranch`. |
| `create.go` | `gcm create <category>` | Validates name, adds category to store. |
| `assign.go` | `gcm assign <branch> <category>` | Verifies branch exists via git, verifies category exists in store, then assigns. Reports reassignment if branch was previously in a different category. |
| `view.go` | `gcm view [category]` | The most complex command. Gathers branches, sync status, commit times, sorts everything, then delegates to TUI or static renderer. |
| `delete.go` | `gcm delete <category>` | Removes category, reports how many branches were moved to Uncategorized. |
| `categories.go` | `gcm categories` | Lists category names. |
| `commit.go` | `gcm commit` | Passthrough to `git commit` via shared `passthroughGit`. `-g` flag triggers AI commit TUI (requires a terminal). |
| `config.go` | `gcm config` | Passthrough to `git config` via shared `passthroughGit`. Intercepts `api-key` args for GCM-managed config in `~/.gcm/config.json`. |
| `help.go` | `gcm help` | Custom help routing: gcm-native commands → gcm help; "both" commands (commit, config, init, branch, clone) → gcm section + git section; git-only commands → `git help` passthrough. `runHelpAll` also invoked by `rootCmd.RunE` for `gcm --help`. |

## View sorting logic (`view.go`)

The `sortView` function and its helpers implement a specific display order:
1. Within each category: current branch first, then remaining branches sorted by most recent commit (newest first)
2. Categories: current branch's category first, then other categories by most recent branch commit, Uncategorized always last

This sorting logic lives in `cmd` rather than `ui` because it requires commit-time data from the git layer.

## Pattern

Every command follows the same structure:
1. Call `git.GetRepoInfo()` to locate the repository
2. Call `store.LoadOrCreate()` to read current state (auto-creates store on first use)
3. Perform the operation
4. Call `store.Save()` to persist changes (for mutating commands)
5. Print a success message

Error handling: errors are both printed to stderr and returned. Cobra's `SilenceUsage` and `SilenceErrors` are set on the root command, so error display is handled by `main.go`.

## Telemetry instrumentation pattern

Commands that are native to gcm delegate from `RunE` to a named helper function:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    return runCreate(cmdTel, args)
}

func runCreate(tel telemetry.Recorder, args []string) (err error) {
    defer func() { tel.Record("cmd_create", map[string]any{"success": err == nil}) }()
    // ...existing logic...
}
```

Key details:
- The helper uses a named return `(err error)` so the deferred `Record` closure captures the final error value.
- `cmdTel` is a package-level `telemetry.Recorder` set by `Execute(tel)`. It defaults to `telemetry.Noop()`.
- `rootCmd.RunE` records `cmd_git_passthrough` for unknown subcommands forwarded to git.
- Pure passthrough subcommands (`gcm branch`, `gcm config`, named passthrough in `commitCmd`) are not instrumented — they go through `passthroughGit` directly, not through `rootCmd.RunE`.

## Exported functions from `root.go`

- `Execute(tel telemetry.Recorder) error` — sets `cmdTel = tel` and runs the Cobra command tree.
- `Version() string` — returns the version string (injected at build time via ldflags; defaults to `"dev"`).
- `IsSecurityDenied(err error) bool` — true if `err` wraps `ErrSecurityDenied`.

## Passthrough pattern

`rootCmd` has `DisableFlagParsing: true` and `Args: cobra.ArbitraryArgs`. Unknown subcommands reach `rootCmd.RunE`, which calls `passthroughGit(globalGitFlags, args)`. Individual subcommands that need to forward to git (commit, config, branch, clone, init) also use `DisableFlagParsing: true` and call `passthroughGit` or the shared helpers.

All passthrough goes through `passthroughGit` in `passthrough_unix.go`/`passthrough_windows.go`, which:
- Runs `checkPassthroughArgs` (security denylist for `-c` keys, `--upload-pack`, `--receive-pack`, `--exec`, `ext::` URLs, `-u` in fetch-family)
- Builds a filtered env via `buildPassthroughEnv` (strips `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`/`GIT_CONFIG_VALUE_n`; injects `GIT_TERMINAL_PROMPT=0` in non-TTY)
- On Unix: uses plain `c.Run()` — no `Setpgid`, no signal goroutine; git stays in the terminal's foreground process group so pagers (`less`) and editors work; Ctrl+C reaches git directly via the kernel
- On Unix: special-cases `git help` to inject `--no-man` in non-TTY to avoid pager hangs

`main.go` strips global git flags (`-C`, `--git-dir`, `--work-tree`, `-c`) from `os.Args` before Cobra dispatch and distributes them to both `cmd.SetGlobalGitFlags` and `git.SetGlobalFlags`. The `-c` denylist is enforced at both the `main.go` parse stage and again inside `checkPassthroughArgs`.

## Security denylist

`IsDeniedConfigKey` blocks `-c key=val` where key matches (case-insensitive):
- `core.fsmonitor`, `core.gitproxy`, `core.hookspath`, `core.editor`
- `sequence.editor`, `diff.external`, `diff.tool`, `diff.guitool`
- `merge.tool`, `merge.guitool`, `gpg.program`, `gpg.ssh.defaultkeycommand`
- `protocol.ext.allow`
- `filter.<name>.clean|smudge|process` (wildcard)
- `alias.*` (all aliases — shell execution via `!`)

`core.sshCommand` and `credential.helper` are deliberately **not** denied (legitimate CI/agent use).

## Gotchas

- The `version` variable in `root.go` defaults to `"dev"` and is overwritten by goreleaser at build time.
- `view.go` imports `mattn/go-isatty` directly rather than through the `ui` package to decide between TUI and static output.
- `commit.go` rejects `-g` when combined with other flags and requires a terminal (`isatty` check) — the commit TUI cannot run in a pipe.
