# gcm — Git Category Manager

A CLI tool that organizes local git branches into user-defined categories. Metadata is stored in `.git/gcm.json` — invisible to git and GitHub. The `view` command renders an interactive TUI (Bubbletea) in terminals and falls back to a static colored tree when piped.

**Module:** `github.com/siddhesh/gcm`
**Go version:** 1.24.0

## Directory Map

| Path | Responsibility |
|---|---|
| `main.go` | Entry point — delegates to `cmd.Execute()` |
| `cmd/` | Cobra command definitions (one file per subcommand) and view sorting logic |
| `internal/git/` | All interactions with the `git` binary — branch listing, sync status, checkout, worktree status |
| `internal/store/` | JSON persistence layer — load/save/validate the `.git/gcm.json` store |
| `internal/ui/` | Terminal output — static tree rendering (`ui.go`) and interactive TUI (`tui.go`) |

## Key Dependencies

| Dependency | Why |
|---|---|
| `spf13/cobra` | CLI framework — subcommand routing, argument validation, help generation |
| `fatih/color` | Colored terminal output for the static (non-TUI) tree renderer |
| `charmbracelet/bubbletea` | Interactive TUI for `gcm view` when stdout is a terminal |
| `charmbracelet/lipgloss` | Styling within the Bubbletea TUI |
| `mattn/go-isatty` | Detect whether stdout is a terminal to choose TUI vs static output |

## Build, Run, Test

```bash
go build -o gcm .          # build
./gcm init                  # run (inside a git repo)
go test ./...               # test all
make test-coverage          # coverage report → coverage.html
make lint                   # golangci-lint
```

Version is injected at build time via `-ldflags -X github.com/siddhesh/gcm/cmd.version=...` (see `.goreleaser.yml`).

## Context Files

- [DECISIONS.md](DECISIONS.md) — technical decisions and rationale
- [ARCHITECTURE.md](ARCHITECTURE.md) — data flow, package relationships, system diagram
- [internal/git/CONTEXT.md](internal/git/CONTEXT.md) — git interaction layer
- [internal/store/CONTEXT.md](internal/store/CONTEXT.md) — JSON persistence layer
- [internal/ui/CONTEXT.md](internal/ui/CONTEXT.md) — terminal rendering (static + TUI)
- [cmd/CONTEXT.md](cmd/CONTEXT.md) — CLI commands and view sorting

## Working with this codebase

**For AI agents working on this project:**
These context files are part of the codebase. When you make changes to
source code, you are responsible for keeping these files accurate.
Specifically:
- If you add, remove, or rename a package → update CLAUDE.md directory map
  and any affected CONTEXT.md files
- If you change how packages interact → update ARCHITECTURE.md
- If you make a significant technical decision or change an existing one
  → update DECISIONS.md
- If you change a package's responsibility, expose new interfaces, or
  introduce new invariants → update that package's CONTEXT.md
- If you notice a discrepancy between a context file and the actual source
  code, correct the context file immediately, even if it is unrelated to
  your current task.
- If you are unsure whether a change warrants a doc update, update it anyway.
  Stale docs are silent bugs.
Never treat these files as optional or as a task for later.
