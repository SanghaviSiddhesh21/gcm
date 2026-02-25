# Architecture

## System Diagram

```
                         ┌──────────┐
                         │  main.go │
                         └────┬─────┘
                              │ cmd.Execute()
                              ▼
                         ┌──────────┐
                         │   cmd/   │  Cobra commands
                         │          │  (init, create, assign, view, delete,
                         │          │   categories, commit, config)
                         └─┬─┬─┬─┬─┘
                           │ │ │ │
            ┌──────────────┘ │ │ └──────────────────┐
            ▼                ▼ ▼                     ▼
     ┌─────────────┐  ┌────────────┐  ┌──────────┐  ┌──────────────┐
     │ internal/git│  │ internal/  │  │ internal │  │ internal/    │
     │             │  │   store    │  │   /ui    │  │   config     │
     └──────┬──────┘  └─────┬──────┘  └─────┬────┘  └──────────────┘
            │               │               │               ▲
            ▼               ▼               │               │
       git binary     .git/gcm.json         │        ┌──────────────┐
                                            │        │ internal/ai  │
                                            └───────►│              │
                                                     └──────┬───────┘
                                                            │
                                                            ▼
                                                   Cloudflare Worker
                                                      → Groq API
```

## Data Flow

### Write operations (init, create, assign, delete)

1. `cmd` calls `git.GetRepoInfo()` to find the `.git` directory
2. `cmd` calls `store.Load()` to read the current state from disk
3. `cmd` mutates the store (add category, assign branch, etc.)
4. `cmd` calls `store.Save()` to write the updated state atomically

### Read operations (view, categories)

1. `cmd` calls `git.GetRepoInfo()` + `store.Load()` as above
2. `cmd` calls `git.ListBranches()`, `git.CurrentBranch()`, and optionally `git.ListRemoteBranches()` + `git.SyncStatus()` + `git.BranchCommitTimes()`
3. `cmd/view.go` builds a sorted category-to-branches map with sync tags
4. If stdout is a terminal → `ui.RunTUI()` launches the Bubbletea interactive view
5. If stdout is piped → `ui.PrintTree()` renders a static colored tree

### Commit flow (`gcm commit -g`)

1. `cmd/commit.go` calls `git.GetStagedChanges()` — errors if nothing staged
2. `cmd/commit.go` calls `git.GetWorktreeStatus()` for the staged/unstaged file lists shown in the TUI
3. `ui.RunCommitTUI()` launches a Bubbletea program, receives `ai.New()` as the generator
4. TUI calls `gen.Generate(ctx, diff)` — `prepareDiff` filters and truncates the diff, then sends it to the Cloudflare Worker with optional `X-User-Api-Key` header
5. User accepts / edits / regenerates the message in the TUI
6. TUI returns the final message string → `cmd` calls `git.Commit(repoInfo.GitDir, message)`

### Branch checkout (TUI only)

1. User presses Enter on a branch in the TUI
2. TUI dispatches an async command to check `git.GetWorktreeStatus()`
3. If worktree is dirty → confirmation prompt; if clean → proceed
4. TUI dispatches `git.Checkout()` asynchronously
5. On success, TUI quits and returns the checked-out branch name to `cmd`

## Package Dependencies

```
cmd     ──→  internal/git     (repo info, branch operations, staged diff, commit)
cmd     ──→  internal/store   (load/save/mutate state)
cmd     ──→  internal/ui      (rendering)
cmd     ──→  internal/ai      (commit message generation)
cmd     ──→  internal/config  (API key management)
ui      ──→  internal/git     (checkout, worktree status — TUI only)
ai      ──→  internal/config  (reads API key for X-User-Api-Key header)
```

`internal/store` and `internal/git` have no dependencies on each other. `internal/ui` depends on `internal/git` only for the TUI's checkout and dirty-worktree check — the static renderer has no git dependency. `internal/ai` depends on `internal/config` to read the optional user API key.

## Boundaries

- **git boundary:** All git interactions go through `internal/git`. No other package runs `exec.Command("git", ...)`.
- **persistence boundary:** All file I/O for `gcm.json` goes through `internal/store`. The `cmd` layer never reads or writes the file directly.
- **display boundary:** All terminal output formatting lives in `internal/ui`. The `cmd` layer only calls `fmt.Printf` for simple success messages and `fmt.Fprintf(os.Stderr, ...)` for errors.
- **config boundary:** All reads and writes of `~/.gcm/config.json` go through `internal/config`. No other package touches that file.
- **AI boundary:** All HTTP calls to the Cloudflare Worker go through `internal/ai`. The TUI interacts only via the `Generator` interface — it never makes HTTP calls directly.

## Store Schema

The `.git/gcm.json` file:

```
version: "1.0"
categories: [{ name, immutable }]
assignments: { branchName → categoryName }
```

Branches not in `assignments` belong to "Uncategorized" implicitly. Assignments can become stale (reference deleted branches) — `BranchesInCategory` filters against the live branch list from git at read time.
