# Interactive TUI Implementation for `gcm view`

This document explains all the code written for the interactive Terminal User Interface (TUI) feature in the `gcm view` command.

## Table of Contents
1. [Git Operations Layer](#1-git-operations-layer)
2. [TUI Implementation](#2-tui-implementation)
3. [Integration with Static View](#3-integration-with-static-view)
4. [Architecture Overview](#architecture-overview)
5. [Key Design Decisions](#key-design-decisions)

---

## 1. Git Operations Layer (`internal/git/git.go`)

### WorktreeStatus Struct
```go
type WorktreeStatus struct {
	Staged    []string  // Files with staged changes
	Unstaged  []string  // Tracked files with unstaged changes
	Untracked []string  // Files not tracked by git
}
```

**Purpose:** Holds the current state of uncommitted changes in three categories.

### IsDirty() Method
```go
func (s WorktreeStatus) IsDirty() bool {
	return len(s.Staged) > 0 || len(s.Unstaged) > 0 || len(s.Untracked) > 0
}
```

**Purpose:** Quick check if there are ANY uncommitted changes (returns true if any category has files).

### GetWorktreeStatus() Function
```go
func GetWorktreeStatus(gitDir string) (WorktreeStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain=v1")
	cmd.Dir = wd
	output, err := cmd.CombinedOutput()
	// Parse output...
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimRight(line, "\r\n")  // Only trim line endings, preserve leading spaces
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]  // First char = staged status, Second char = unstaged status
		name := strings.TrimSpace(line[3:])  // Filename starts at position 3

		switch {
		case x == '?' && y == '?':
			result.Untracked = append(result.Untracked, name)
		default:
			if x != ' ' && x != '?' {
				result.Staged = append(result.Staged, name)  // x != ' ' means file is staged
			}
			if y != ' ' && y != '?' {
				result.Unstaged = append(result.Unstaged, name)  // y != ' ' means file has unstaged changes
			}
		}
	}
}
```

**Purpose:** Gets the git status and categorizes files. Uses `git status --porcelain=v1` which outputs lines like:
- ` M file.txt` → Space M = staged changes: none, unstaged changes: Modified
- `A  file.txt` → A space = staged changes: Added, unstaged: none
- `?? file.txt` → ? ? = untracked

**Key Implementation Detail:** Uses `TrimRight` instead of `TrimSpace` to preserve leading spaces that are part of git's porcelain format.

### Checkout() Function
```go
func Checkout(gitDir, branch string) error {
	_, err := runGit(workDir(gitDir), "checkout", branch)
	if err != nil {
		return fmt.Errorf("git checkout %s: %w", branch, err)
	}
	return nil
}
```

**Purpose:** Simple wrapper around `git checkout <branch>`. Wraps any git error with context.

---

## 2. TUI Implementation (`internal/ui/tui.go`)

### Data Structures

#### itemKind (discriminator)
```go
type itemKind int
const (
	itemCategory itemKind = iota  // 0 = category header row
	itemBranch                    // 1 = branch row
)
```

**Purpose:** Tells us whether an item in the flat list is a category header or a branch name.

#### item (single row in TUI)
```go
type item struct {
	kind      itemKind  // Category or Branch?
	label     string    // Either category name or branch name
	category  string    // Parent category (only for branches)
	tag       string    // e.g., "[Local]", "[Remote] ↑2"
	isCurrent bool      // Is this the checked-out branch?
	isLast    bool      // Is this the last branch in its category? (for └── vs ├──)
}
```

**Purpose:** Represents one navigable row in the TUI. The flat list of items allows us to use a simple cursor position instead of managing nested structures.

#### viewMode (state machine)
```go
type viewMode int
const (
	modeBrowse  viewMode = iota  // Normal navigation
	modeConfirm                  // Showing dirty worktree confirmation
)
```

**Purpose:** Tracks which "screen" we're on - browsing branches or confirming a checkout.

#### model (main Bubbletea model)
```go
type model struct {
	// Immutable data from caller
	gitDir        string
	categories    []string
	branchMap     map[string][]string
	branchTags    map[string]string
	currentBranch string

	// Navigation state
	items            []item   // Flat list of all visible items
	cursor           int      // Index into items[] of currently selected item
	viewportStartIdx int      // Index of first item shown on screen (for scrolling)

	// Expand/collapse state
	collapsed map[string]bool  // Which categories are collapsed?

	// UI state machine
	mode          viewMode      // Browse or Confirm?
	confirmTarget string        // Which branch is being checked out?
	confirmStatus git.WorktreeStatus  // Files to show in confirmation prompt

	// Results
	checkedOut string  // Non-empty after successful checkout
	errMsg     string  // Error to display at bottom
	width      int
	height     int
}
```

**Purpose:** Holds all the state for the TUI application. This is the core of Bubbletea's Model interface.

### Async Messages
```go
type checkoutResultMsg struct {
	branch string
	err    error
}

type worktreeStatusMsg struct {
	branch string
	status git.WorktreeStatus
	err    error
}
```

**Purpose:** Messages sent back from background operations. Bubbletea lets us run I/O in background and handle results asynchronously.

### Async Commands
```go
func doWorktreeStatus(gitDir string) tea.Cmd {
	return func() tea.Msg {
		status, err := git.GetWorktreeStatus(gitDir)
		return worktreeStatusMsg{status: status, err: err}
	}
}

func doCheckout(gitDir, branch string) tea.Cmd {
	return func() tea.Msg {
		err := git.Checkout(gitDir, branch)
		return checkoutResultMsg{branch: branch, err: err}
	}
}
```

**Purpose:** Returns Bubbletea commands that run git operations in the background. When they complete, they send back a message with the result. This keeps the UI responsive.

### Init() - Bubbletea Lifecycle
```go
func (m model) Init() tea.Cmd {
	return nil  // No initial I/O needed
}
```

**Purpose:** Called when TUI starts. We don't need any initial commands since all data was pre-loaded by `cmd/view.go`.

### Update() - State Machine (Core Logic)
```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:  // Terminal was resized
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.mode {
		case modeBrowse:
			switch msg.String() {
			case "up":
				if m.cursor > 0 { m.cursor-- }  // Move up
				m.ensureVisible()  // Keep cursor in viewport
			case "down":
				if m.cursor < len(m.items)-1 { m.cursor++ }  // Move down
				m.ensureVisible()  // Keep cursor in viewport
			case " ", "right":  // Expand/collapse
				if m.items[m.cursor].kind == itemCategory {
					cat := m.items[m.cursor].label
					m.collapsed[cat] = !m.collapsed[cat]  // Toggle
					m.items = m.rebuildItems()  // Rebuild list with new visibility
					m.ensureVisible()  // Adjust viewport after list changes
				}
			case "enter":
				if m.items[m.cursor].kind != itemBranch { return m, nil }
				target := m.items[m.cursor].label

				// Already on this branch?
				if target == m.currentBranch {
					m.errMsg = fmt.Sprintf("Already on '%s'", target)
					return m, nil
				}

				// Check for dirty files asynchronously
				return m, doWorktreeStatus(m.gitDir)
			case "q", "esc":
				return m, tea.Quit

		case modeConfirm:  // In confirmation prompt
			switch msg.String() {
			case "y", "Y":
				m.mode = modeBrowse
				return m, doCheckout(m.gitDir, m.confirmTarget)  // Do the checkout
			case "n", "N", "q", "esc":
				m.mode = modeBrowse
				m.errMsg = "Checkout cancelled"
			}

	case worktreeStatusMsg:  // Async result: worktree status received
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Could not check worktree status: %v", msg.err)
			return m, nil
		}

		target := m.items[m.cursor].label

		if msg.status.IsDirty() {
			// Show confirmation prompt
			m.mode = modeConfirm
			m.confirmTarget = target
			m.confirmStatus = msg.status
			m.errMsg = ""
		} else {
			// Clean worktree, proceed directly
			return m, doCheckout(m.gitDir, target)
		}

	case checkoutResultMsg:  // Async result: checkout complete
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Checkout failed: %v", msg.err)
			return m, nil
		}
		m.checkedOut = msg.branch
		return m, tea.Quit  // Exit TUI after successful checkout
	}

	return m, nil
}
```

**Purpose:** The heart of the TUI. Processes all user input and messages, updates state, and returns commands to execute. Implements a state machine with two modes:
- **modeBrowse:** Navigation mode - user selects branches
- **modeConfirm:** Confirmation mode - user confirms checkout with dirty files

**Flow:**
1. User presses Enter on a branch
2. System checks if working tree is dirty (async)
3. If dirty: show confirmation prompt
4. If clean OR user confirms: checkout branch (async)
5. On success: exit TUI and return branch name

### View() - Rendering
```go
func (m model) View() string {
	var sb strings.Builder

	// ─── Confirmation Mode ───────────────────────────
	if m.mode == modeConfirm {
		sb.WriteString(styleWarning.Render("Uncommitted changes detected:\n"))

		// List staged, unstaged, untracked files
		if len(m.confirmStatus.Staged) > 0 {
			sb.WriteString("  Staged:\n")
			for _, f := range m.confirmStatus.Staged {
				sb.WriteString("    " + f + "\n")
			}
		}
		// ... etc for Unstaged and Untracked ...

		prompt := fmt.Sprintf("\nSwitch to '%s' anyway? [y/n] ", m.confirmTarget)
		sb.WriteString(styleWarning.Render(prompt))
		return sb.String()
	}

	// ─── Browse Mode ─────────────────────────────────
	for i, it := range m.items {
		isSelected := i == m.cursor

		switch it.kind {
		case itemCategory:
			collapseMarker := "▼ "
			if m.collapsed[it.label] {
				collapseMarker = "▶ "  // Collapsed = right arrow
			}
			catText := styleCategory.Render(collapseMarker + it.label)
			if isSelected {
				catText = styleSelected.Render(collapseMarker + it.label)
			}
			sb.WriteString("- " + catText + "\n")

		case itemBranch:
			prefix := "  ├── "
			if it.isLast {
				prefix = "  └── "  // Last branch gets └
			}

			marker := ""
			if it.isCurrent {
				marker = "● "  // Dot for current branch
			}

			var branchText string
			if it.isCurrent {
				branchText = styleCurrent.Render(marker + it.label)  // Green
			} else {
				branchText = styleBranch.Render(marker + it.label)   // White
			}

			if isSelected {
				branchText = styleSelected.Render(marker + it.label)  // Highlight selected
			}

			tagText := renderTagLipgloss(it.tag)  // Colored status tag

			row := prefix + branchText
			if tagText != "" {
				row += "  " + tagText
			}
			sb.WriteString(row + "\n")
		}
	}

	// ─── Footer ──────────────────────────────────────
	sb.WriteString("\n")
	if m.errMsg != "" {
		sb.WriteString(styleError.Render(m.errMsg) + "\n")
	}
	hint := styleMeta.Render("↑/↓ navigate  enter checkout  space/→ expand  q quit")
	sb.WriteString(hint + "\n")

	return sb.String()
}
```

**Purpose:** Renders the entire TUI. Handles two modes:
- **modeBrowse:** Shows the tree of categories and branches with the cursor highlighted. Only renders items visible in the viewport (`viewportStartIdx` to `viewportStartIdx + visibleHeight`), enabling scrolling for tall trees.
- **modeConfirm:** Shows warning about dirty files and asks for confirmation

**Visual Example:**
```
- features (3 branches)
  ├── ● main  [Remote] InSync      ← Current branch (●), selected (highlighted)
  ├── feature/login  [Remote] ↑2
  └── bugfix  [Local]

- hotfix (2 branches)

↑/↓ navigate  enter checkout  space/→ expand  q quit
```

### ensureVisible() - Viewport Scrolling

```go
func (m *model) ensureVisible() {
	footerHeight := 2  // Error line + hint line
	visibleHeight := m.height - footerHeight

	if visibleHeight <= 0 {
		m.viewportStartIdx = 0
		return
	}

	// If cursor is above viewport, scroll up
	if m.cursor < m.viewportStartIdx {
		m.viewportStartIdx = m.cursor
	}

	// If cursor is below viewport, scroll down
	if m.cursor >= m.viewportStartIdx+visibleHeight {
		m.viewportStartIdx = m.cursor - visibleHeight + 1
	}
}
```

**Purpose:** Manages vertical scrolling when the terminal is too short to display all items. Ensures the cursor stays within the visible viewport by adjusting `viewportStartIdx`.

**How it works:**
1. Calculates available height: `terminal height - footer (2 lines)`
2. If cursor is above the current viewport, scroll up to show it
3. If cursor is below the current viewport, scroll down to show it
4. This is called after cursor movement and category collapse/expand

**Example:**
- Terminal height: 20 lines
- Available height: 18 lines (20 - 2 footer)
- If you have 50 branches and cursor is at position 30
- `ensureVisible()` will set `viewportStartIdx = 13` so cursor appears in the middle of the viewport

### Helper Functions

#### buildItems()
```go
func buildItems(categories []string, branchMap map[string][]string,
                currentBranch string, branchTags map[string]string) []item {
	var items []item
	for _, cat := range categories {
		items = append(items, item{
			kind:  itemCategory,
			label: cat,
		})
		branches := branchMap[cat]
		for i, branch := range branches {
			items.append(items, item{
				kind:      itemBranch,
				label:     branch,
				category:  cat,
				tag:       branchTags[branch],
				isCurrent: branch == currentBranch,
				isLast:    i == len(branches)-1,  // Last one gets └──
			})
		}
	}
	return items
}
```

**Purpose:** Converts the hierarchical `(categories, branchMap)` into a flat list of items. This makes navigation with a simple cursor position possible.

**Example:**
```
Input:
categories = ["features", "hotfix"]
branchMap = {
	"features": ["main", "login", "bugfix"],
	"hotfix": ["v0.1.7", "v0.1.8"]
}

Output (flat list):
[
	{kind: itemCategory, label: "features"},
	{kind: itemBranch, label: "main", isLast: false},
	{kind: itemBranch, label: "login", isLast: false},
	{kind: itemBranch, label: "bugfix", isLast: true},   ← isLast=true, gets └──
	{kind: itemCategory, label: "hotfix"},
	{kind: itemBranch, label: "v0.1.7", isLast: false},
	{kind: itemBranch, label: "v0.1.8", isLast: true},
]
```

#### rebuildItems()
```go
func (m *model) rebuildItems() []item {
	// Same as buildItems but checks m.collapsed
	// to skip branches under collapsed categories
}
```

**Purpose:** Rebuilds the flat items list after a category is collapsed/expanded. Rebuilding ensures the cursor doesn't point to hidden items.

#### renderTagLipgloss()
```go
func renderTagLipgloss(tag string) string {
	// Parse "[Local]" or "[Remote]" prefix
	// Parse status suffix like " InSync", " ↑2", etc.
	// Apply appropriate colors using lipgloss styles
	// Return styled string
}
```

**Purpose:** Converts tag strings like `"[Remote] ↑2"` into colored text. Mirrors the logic from the static `PrintTree()` view.

**Example:**
- `"[Local]"` → Grey text
- `"[Remote] InSync"` → Magenta `[Remote]` + Green ` InSync`
- `"[Remote] ↑2"` → Magenta `[Remote]` + Blue ` ↑2`

#### RunTUI()
```go
func RunTUI(gitDir string, categories []string, branchMap map[string][]string,
            currentBranch string, branchTags map[string]string) (checkedOut string, err error) {

	// Build initial flat item list
	items := buildItems(categories, branchMap, currentBranch, branchTags)

	// Find cursor position (point to current branch)
	cursor := 0
	for i, it := range items {
		if it.kind == itemBranch && it.isCurrent {
			cursor = i
			break
		}
	}

	// Create and run Bubbletea program
	m := model{
		gitDir:        gitDir,
		categories:    categories,
		branchMap:     branchMap,
		branchTags:    branchTags,
		currentBranch: currentBranch,
		items:         items,
		cursor:        cursor,
		collapsed:     make(map[string]bool),  // All expanded initially
		mode:          modeBrowse,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	// Return which branch was checked out (empty if none)
	if fm, ok := finalModel.(model); ok {
		return fm.checkedOut, nil
	}
	return "", nil
}
```

**Purpose:** Entry point to the TUI. Creates the initial model state, runs the Bubbletea program, and returns the result to the caller.

---

## 3. Integration with Static View (`cmd/view.go`)

### TTY Detection and Branching
```go
categoryNames, branchMap = sortView(categoryNames, branchMap, currentBranch, branchTimes)

// ← New code: Check if stdout is a TTY (terminal)
if isatty.IsTerminal(os.Stdout.Fd()) {
	// We're in an interactive terminal, launch TUI
	checkedOut, err := ui.RunTUI(repoInfo.GitDir, categoryNames, branchMap, currentBranch, branchTags)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Print success message after checkout
	if checkedOut != "" {
		ui.PrintSuccess(fmt.Sprintf("Switched to branch '%s'", checkedOut))
	}
	return nil
}

// Not a TTY (piped, redirected, CI), use static view
ui.PrintTree(categoryNames, branchMap, currentBranch, branchTags)
return nil
```

**Purpose:** Detects if the user is in an interactive terminal:
- **Is TTY:** Launch interactive TUI
- **Not TTY (piped/redirected):** Show static text output

**Examples:**
```bash
# TTY - shows interactive TUI
$ gcm view

# Not TTY - shows static output
$ gcm view | cat
$ gcm view > output.txt
$ git log | gcm view
```

This allows the tool to work in all contexts: interactive terminals, scripts, CI/CD pipelines, and pipes.

---

## Architecture Overview

```
User input (keyboard)
    ↓
View.go (TTY detection)
    ↓
RunTUI() ← Calls this if TTY
    ↓
Bubbletea Program (Update/View loop)
    ├─ Update: Processes keyboard input
    │  ├─ Navigation: Up/Down move cursor
    │  ├─ Collapse/Expand: Space/Right toggle category
    │  ├─ Checkout: Enter → calls GetWorktreeStatus (async)
    │  │            → If dirty, show confirmation
    │  │            → If clean, call Checkout (async)
    │  └─ Quit: q/Esc exits TUI
    │
    ├─ Git operations (run in background)
    │  ├─ GetWorktreeStatus() → checks for uncommitted changes
    │  └─ Checkout() → switches branches
    │
    └─ View: Renders the TUI
       ├─ Browse mode: Shows tree with cursor
       └─ Confirm mode: Shows dirty files + y/n prompt
```

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate between branches and categories |
| `Space` / `→` | Expand/collapse category |
| `Enter` | Checkout selected branch |
| `q` / `Esc` | Quit TUI |
| `y` / `Y` | Confirm checkout with dirty files |
| `n` / `N` | Cancel checkout |

---

## Key Design Decisions

### 1. Flat Items List
Converting the hierarchical category/branch structure into a flat list makes cursor navigation simple (just increment/decrement index). No need to manage nested state.

**Benefit:** Simple, efficient navigation without complex tree-walking logic.

### 2. Async Operations
Git calls happen in the background so the UI stays responsive while we check status or checkout.

**Benefit:** User can't accidentally freeze the UI. Long git operations don't block interaction.

### 3. State Machine
Two modes (`modeBrowse` / `modeConfirm`) keep the code organized and clear. Each mode has different keyboard handlers.

**Benefit:** Clear separation of concerns. Easy to reason about what happens in each state.

### 4. Graceful Fallback
Always checks if we're in a TTY with `isatty.IsTerminal()`. Non-TTY contexts get static output.

**Benefit:** Single `gcm view` command works everywhere - interactive terminals, scripts, pipes, CI/CD.

### 5. Immutable Display Sort
The sort order (current category first, commit-time ordering) is done once in `view.go` before entering TUI.

**Benefit:** TUI doesn't need to re-sort. Data structure is ready for navigation.

### 6. No Auto-Stash or Auto-Discard
Unlike some tools, we don't automatically stash or discard changes. We show what would happen and let the user decide.

**Benefit:** Safe. User retains full control. No data loss surprises.

### 7. Always Show Hints
Footer shows keyboard hints even when displaying errors.

**Benefit:** User always knows what keys are available, even after hitting an error.

---

## Error Handling

### Safe Checkout
```
User presses Enter on branch
  ↓
Check worktree status
  ├─ On error: Show error message, stay in browse mode
  ├─ If dirty: Show confirmation prompt with file lists
  └─ If clean: Proceed directly to checkout
      ↓
    Attempt checkout
      ├─ On error: Show git error, stay in TUI
      └─ On success: Exit TUI, print success message
```

### Specific Error Scenarios

| Scenario | Behavior |
|----------|----------|
| Checking out current branch | Show "Already on 'X'" message, don't run git |
| Worktree status check fails | Show error, stay in TUI |
| Checkout fails (git error) | Show git error message, stay in TUI |
| Merge conflict during checkout | Show git error, TUI stays open |
| User cancels with 'n' | Show "Checkout cancelled", return to browse |
| Dirty files but user confirms | Run checkout, show success/error |

---

## Testing

### Unit Tests
- `TestWorktreeStatus` - Tests file categorization (staged/unstaged/untracked)
- `TestCheckout` - Tests branch switching, error handling, idempotency

### Manual Testing Scenarios
1. Navigate with arrows and collapse/expand categories
2. Checkout clean branch (no confirmation)
3. Checkout with dirty files (show confirmation, test y/n)
4. Try to checkout current branch (show "Already on" message)
5. Pipe output: `gcm view | cat` (should show static text)
6. Cancel checkout with 'n' (should stay in TUI)

---

## Summary

The interactive TUI feature transforms `gcm view` from a static text command into a full terminal application:

- **User Experience:** Interactive navigation, immediate feedback, confirmation prompts
- **Safety:** Detects dirty files, prevents accidental destructive operations
- **Compatibility:** Works in TTY, scripts, pipes, and CI/CD
- **Reliability:** Async operations, proper error handling, always responsive

All while maintaining the existing static view as a fallback and preserving the smart branch sorting implemented previously.
