# Architecture

## System Diagram

```
                         ┌──────────┐
                         │  main.go │ strips global git flags (-C, --git-dir, -c ...)
                         └────┬─────┘ inits telemetry, calls run(tel), flushes, os.Exit
                              │ cmd.Execute(tel)
                              ▼
                         ┌──────────┐
                         │   cmd/   │  Cobra commands
                         │          │  (init, clone, branch, create, assign, view,
                         │          │   delete, categories, commit, config)
                         │          │  + passthrough.go (security, env filtering)
                         └─┬─┬─┬─┬─┘
                              │ unknown subcommand
                              ▼
                         git binary (passthrough)
                           │ │ │ │
            ┌──────────────┘ │ │ └──────────────────┐
            ▼                ▼ ▼                     ▼
     ┌─────────────┐  ┌────────────┐  ┌──────────┐  ┌──────────────┐
     │ internal/git│  │ internal/  │  │ internal │  │ internal/    │
     │             │  │   store    │  │   /ui    │  │   config     │
     └──────┬──────┘  └─────┬──────┘  └─────┬────┘  └──────┬───────┘
            │               │               │               │
            ▼               ▼               │               ▼
       git binary     .git/gcm.json         │        ┌──────────────┐
                                            │        │ internal/ai  │
                                            └───────►│              │
                                                     └──────┬───────┘
                                                            │
                                                            ▼
                                                   Cloudflare Worker
                                                   → Groq API / PostHog

  main.go ──► internal/config (GetOrCreateInstallID)
          ──► internal/telemetry (New, Record, Flush)
  cmd/    ──► internal/telemetry (cmdTel.Record per command)
```

## Data Flow

### Passthrough (unknown subcommand or explicit forwarding)

1. `main.go` strips and validates global flags (`-C`, `--git-dir`, `--work-tree`, `-c`)
2. `cmd.SetGlobalGitFlags` + `git.SetGlobalFlags` receive the flags
3. `rootCmd.RunE` calls `checkPassthroughArgs` (security denylist) then `passthroughGit`
4. `passthroughGit` builds a filtered env (`buildPassthroughEnv`), runs git via `c.Run()` (same process group as gcm so pagers and editors can access the TTY), and returns the exit code

### Write operations (init, clone, create, assign, delete, branch)

1. `cmd` runs any necessary git operation (e.g. `git init`, `git clone`) via `passthroughGit`
2. `cmd` calls `git.GetRepoInfo()` to find the `.git` directory
3. `cmd` calls `store.LoadOrCreate()` to read the current state from disk (creates a fresh store if none exists)
4. `cmd` mutates the store (add category, assign branch, rename branch, etc.)
5. `cmd` calls `store.Save()` to write the updated state atomically

### Read operations (view, categories)

1. `cmd` calls `git.GetRepoInfo()` + `store.LoadOrCreate()` as above
2. `cmd` calls `git.ListBranches()`, `git.CurrentBranch()`, and optionally `git.ListRemoteBranches()` + `git.SyncStatus()` + `git.BranchCommitTimes()`
3. `cmd/view.go` builds a sorted category-to-branches map with sync tags
4. If stdout is a terminal → `ui.RunTUI()` launches the Bubbletea interactive view
5. If stdout is piped → `ui.PrintTree()` renders a static colored tree

### Commit flow (`gcm commit -g`)

1. `cmd/commit.go` calls `git.GetStagedChanges()` — errors if nothing staged
2. `cmd/commit.go` calls `git.GetWorktreeStatus()` for the staged/unstaged file lists shown in the TUI
3. `ui.RunCommitTUI()` launches a Bubbletea program, receives `ai.New()` as the generator
4. TUI calls `gen.Generate(ctx, diff, gist, summaryPrev, attempt)` — `prepareDiff` filters the diff and returns the attempt-th 6000-char window; prior window messages are passed as `gist`; once `ai.IsDiffExhausted` returns true the TUI switches to summary phase (`summaryPrev != nil`)
5. User accepts / edits / regenerates the message in the TUI; each 'r' press advances the window or accumulates summary context
6. TUI returns a `CommitResult` (message, outcome, regeneration count) → `cmd` calls `git.Commit(repoInfo.GitDir, message)`
7. `cmd` calls `cmdTel.Record("cmd_commit_ai", ...)` with outcome and regenerations count

### Branch checkout (TUI only)

1. User presses Enter on a branch in the TUI
2. TUI dispatches an async command to check `git.GetWorktreeStatus()`
3. If worktree is dirty → confirmation prompt; if clean → proceed
4. TUI dispatches `git.Checkout()` asynchronously
5. On success, TUI quits and returns the checked-out branch name to `cmd`

## Package Dependencies

```
main          ──→  internal/config    (GetOrCreateInstallID)
main          ──→  internal/telemetry (New, Flush)
cmd           ──→  internal/git       (repo info, branch operations, staged diff, commit)
cmd           ──→  internal/store     (load/save/mutate state)
cmd           ──→  internal/ui        (rendering)
cmd           ──→  internal/ai        (commit message generation)
cmd           ──→  internal/config    (API key management)
cmd           ──→  internal/telemetry (Record per command via cmdTel)
ui            ──→  internal/git       (checkout, worktree status — TUI only)
ai            ──→  internal/config    (reads API key for X-User-Api-Key header)
```

`internal/store` and `internal/git` have no dependencies on each other. `internal/ui` depends on `internal/git` only for the TUI's checkout and dirty-worktree check — the static renderer has no git dependency. `internal/ai` depends on `internal/config` to read the optional user API key. `internal/telemetry` has no dependencies on other internal packages (it duplicates the `isDisabledEnv()` check from `internal/config` to avoid a circular import).

## Boundaries

- **git boundary:** All internal git queries go through `internal/git`. Passthrough to git for user-facing commands goes through `cmd/passthrough_unix.go` / `cmd/passthrough_windows.go`, which is the only place `exec.Command("git", ...)` is called outside `internal/git`.
- **persistence boundary:** All file I/O for `gcm.json` goes through `internal/store`. The `cmd` layer never reads or writes the file directly.
- **display boundary:** All terminal output formatting lives in `internal/ui`. The `cmd` layer only calls `fmt.Printf` for simple success messages and `fmt.Fprintf(os.Stderr, ...)` for errors.
- **config boundary:** All reads and writes of `~/.gcm/config.json` go through `internal/config`. No other package touches that file.
- **AI boundary:** All HTTP calls to the Cloudflare Worker (AI generation) go through `internal/ai`. The TUI interacts only via the `Generator` interface — it never makes HTTP calls directly.
- **telemetry boundary:** All event recording and flushing goes through `internal/telemetry`. The `cmd` layer calls `cmdTel.Record(...)` — it never constructs HTTP requests. `main.go` calls `tel.Flush()` before `os.Exit`. The `Recorder` interface means any caller can hold a `noop{}` if telemetry is disabled.

## Store Schema

The `.git/gcm.json` file:

```
version: "1.0"
categories: [{ name, immutable }]
assignments: { branchName → categoryName }
```

Branches not in `assignments` belong to "Uncategorized" implicitly. Assignments can become stale (reference deleted branches) — `BranchesInCategory` filters against the live branch list from git at read time.
