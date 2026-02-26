# internal/git

Thin wrapper around the `git` CLI binary. All git operations in the project go through this package.

## Public API

- **RepoInfo** — Holds `WorkDir` (repo root) and `GitDir` (`.git` path). Obtained via `GetRepoInfo()`, which must be called from within a git repository.
- **GetGitDirAt(dir)** — Returns the canonical git dir for the repo at `dir` (absolute when `dir` is absolute). Used by `gcm init` and `gcm clone` after running the underlying git command.
- **SetGlobalFlags(flags)** — Stores git global flags (`--git-dir`, `--work-tree`, `-c key=val`) to be prepended to every `runGit` call. Called once at startup by `main.go` alongside `cmd.SetGlobalGitFlags`.
- **Branch operations** — `ListBranches`, `CurrentBranch`, `BranchExists` query local branch state. `ListRemoteBranches` returns `origin/*` branches with the prefix stripped.
- **SyncStatus** — Computes ahead/behind counts for a branch against its `origin/` counterpart using `rev-list --count`.
- **BranchCommitTimes** — Returns a map of branch → most recent commit timestamp, taking the max of local and remote ref times. Uses a single `for-each-ref` call for efficiency.
- **WorktreeStatus** — Parses `git status --porcelain=v1` into staged/unstaged/untracked file lists. Used by the view TUI (dirty-worktree confirmation) and the commit TUI (file list display).
- **GetStagedChanges** — Returns `git diff --cached` output. Used by `cmd/commit.go` to obtain the diff for AI generation. Returns empty string if nothing is staged.
- **Checkout** — Switches the working tree to a given branch.
- **Commit** — Creates a commit with `git commit -m <message>`.

## Internal

- `runGit` is the single point where `exec.Command("git", ...)` is called. All public functions delegate to it. It prepends `globalGitFlags` to every invocation and trims trailing `\r\n` from output (not `TrimSpace` — leading spaces in `git status --porcelain=v1` output are meaningful).
- `workDir` resolves the working directory from a gitDir path, handling the special case of relative `.git`.

## Gotchas

- `ListRemoteBranches` silently returns an empty slice if no remote is configured (error is intentionally swallowed).
- `SyncStatus` assumes the remote is always `origin`. Multi-remote setups are not supported.
- `GetRepoInfo` depends on the current working directory — it calls `git rev-parse --show-toplevel` without an explicit path.
- `BranchCommitTimes` filters out `origin/HEAD` to avoid polluting the results.

## Testing

Tests create real temporary git repositories (with commits and remotes) using `setupTestRepo` and `setupTestRepoWithRemote` helpers. Tests cover:
- Repo detection (inside vs outside git repo)
- Branch listing with absolute and relative gitDir paths
- Remote branch listing (with remote, without remote, unpushed branches)
- Sync status (in-sync, ahead, behind, diverged, no remote ref)
- Commit time retrieval (local-only, remote-newer, local-newer, in-sync)
- Worktree status (clean, untracked, staged, unstaged modifications)
- Checkout (success, back-and-forth, nonexistent branch, idempotent)
- Staged changes (clean worktree returns empty string, with staged file returns diff)
- Commit (creates commit with correct message)
