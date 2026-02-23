# internal/ui

Terminal rendering layer with two output modes: a static colored tree and an interactive TUI.

## Static renderer (`ui.go`)

- **PrintTree** — Renders a category→branches tree with box-drawing characters (`├──`, `└──`), current branch markers (`●`), and colored sync tags. Uses `fatih/color`.
- **PrintCategoryList** — Simple list of category names.
- **PrintSuccess / PrintWarning / PrintError** — One-line colored messages.
- **renderBranchTag** — Parses `[Local]` / `[Remote]` tags and applies distinct colors to the label and status portions.

## Interactive TUI (`tui.go`)

A Bubbletea program that renders the same tree as the static renderer but adds:
- Keyboard navigation (up/down to move, enter to checkout, space/right to collapse/expand categories)
- Category collapsing (▼/▶ markers)
- Branch checkout with dirty-worktree confirmation
- Viewport scrolling for large branch lists

The TUI depends on `internal/git` for checkout and worktree status checks. All git operations are dispatched as async Bubbletea commands.

## Exposed to `cmd`

- `RunTUI` — Entry point for the interactive view. Returns the name of the checked-out branch (empty string if none).
- `PrintTree`, `PrintCategoryList`, `PrintSuccess`, `PrintWarning`, `PrintError` — Static output functions.

## Gotchas

- The TUI and static renderer have parallel but separate tag-rendering logic (`renderBranchTag` in `ui.go` vs `renderTagLipgloss` in `tui.go`). Changes to tag formatting must be made in both places.
- The TUI's `buildItems` flattens the category/branch tree into a single navigable list. Collapsing a category removes its branch items from this list, which can shift the cursor.
- Color definitions exist in two parallel sets: `fatih/color` vars in `ui.go` and `lipgloss` styles in `tui.go`. These should stay visually consistent.
