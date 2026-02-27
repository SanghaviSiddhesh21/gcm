package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddhesh/gcm/internal/git"
)

type itemKind int

const (
	itemCategory itemKind = iota
	itemBranch
)

type item struct {
	kind      itemKind
	label     string // category name or branch name
	category  string // parent category (branch items only)
	tag       string // branchTags value (branch items only)
	isCurrent bool
	isLast    bool // last branch in category (for └── vs ├──)
}

type viewMode int

const (
	modeBrowse viewMode = iota
	modeConfirm
)

type model struct {
	gitDir        string
	categories    []string
	branchMap     map[string][]string
	branchTags    map[string]string
	currentBranch string

	items            []item
	cursor           int
	collapsed        map[string]bool
	viewportStartIdx int // First visible item in viewport

	mode          viewMode
	confirmTarget string
	confirmStatus git.WorktreeStatus

	checkedOut string
	errMsg     string
	width      int
	height     int
}

type checkoutResultMsg struct {
	branch string
	err    error
}

type worktreeStatusMsg struct {
	status git.WorktreeStatus
	err    error
}

var (
	styleCategory = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))                                  // cyan
	styleCurrent  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))                                  // green
	styleBranch   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))                                             // white
	styleMeta     = lipgloss.NewStyle().Faint(true)                                                                 // faint
	styleRemote   = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))                                             // magenta
	styleInSync   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))                                             // green
	styleAhead    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))                                             // blue
	styleBehind   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))                                             // yellow
	styleDiverged = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))                                             // red
	styleUnknown  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))                                             // yellow
	styleSelected = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15")) // grey bg
	styleWarning  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))                                             // yellow
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))                                             // red
)

func doCheckout(gitDir, branch string) tea.Cmd {
	return func() tea.Msg {
		err := git.Checkout(gitDir, branch)
		return checkoutResultMsg{branch: branch, err: err}
	}
}

func doWorktreeStatus(gitDir string) tea.Cmd {
	return func() tea.Msg {
		status, err := git.GetWorktreeStatus(gitDir)
		return worktreeStatusMsg{status: status, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {

		case modeBrowse:
			switch msg.String() {
			case "q", "esc":
				return m, tea.Quit

			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
				m.ensureVisible()
				return m, nil

			case "down":
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
				m.ensureVisible()
				return m, nil

			case " ", "right":
				if m.cursor < len(m.items) && m.items[m.cursor].kind == itemCategory {
					cat := m.items[m.cursor].label
					m.collapsed[cat] = !m.collapsed[cat]
					m.items = m.rebuildItems()
					if m.cursor >= len(m.items) {
						m.cursor = len(m.items) - 1
					}
					m.ensureVisible()
				}
				return m, nil

			case "enter":
				if m.cursor >= len(m.items) {
					return m, nil
				}
				it := m.items[m.cursor]
				if it.kind != itemBranch {
					return m, nil
				}

				target := it.label
				if target == m.currentBranch {
					m.errMsg = fmt.Sprintf("Already on '%s'", target)
					return m, nil
				}

				return m, doWorktreeStatus(m.gitDir)

			}

		case modeConfirm:
			switch msg.String() {
			case "y", "Y":
				m.mode = modeBrowse
				return m, doCheckout(m.gitDir, m.confirmTarget)

			case "n", "N", "q", "esc":
				m.mode = modeBrowse
				m.errMsg = "Checkout cancelled"
				return m, nil
			}
		}

	case worktreeStatusMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Could not check worktree status: %v", msg.err)
			return m, nil
		}

		if m.cursor >= len(m.items) {
			return m, nil
		}
		target := m.items[m.cursor].label

		if msg.status.IsDirty() {
			m.mode = modeConfirm
			m.confirmTarget = target
			m.confirmStatus = msg.status
			m.errMsg = ""
		} else {
			return m, doCheckout(m.gitDir, target)
		}
		return m, nil

	case checkoutResultMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Checkout failed: %v", msg.err)
			return m, nil
		}
		m.checkedOut = msg.branch
		return m, tea.Quit
	}

	return m, nil
}

// ensureVisible adjusts viewportStartIdx so cursor stays within visible area
func (m *model) ensureVisible() {
	footerHeight := 2 // error line + hint line
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

func (m model) View() string {
	var sb strings.Builder

	if m.mode == modeConfirm {
		sb.WriteString(styleWarning.Render("Uncommitted changes detected:\n"))
		if len(m.confirmStatus.Staged) > 0 {
			sb.WriteString(styleWarning.Render("  Staged:\n"))
			for _, f := range m.confirmStatus.Staged {
				sb.WriteString(styleWarning.Render("    " + f + "\n"))
			}
		}
		if len(m.confirmStatus.Unstaged) > 0 {
			sb.WriteString(styleWarning.Render("  Unstaged:\n"))
			for _, f := range m.confirmStatus.Unstaged {
				sb.WriteString(styleWarning.Render("    " + f + "\n"))
			}
		}
		if len(m.confirmStatus.Untracked) > 0 {
			sb.WriteString(styleWarning.Render("  Untracked:\n"))
			for _, f := range m.confirmStatus.Untracked {
				sb.WriteString(styleWarning.Render("    " + f + "\n"))
			}
		}
		prompt := fmt.Sprintf("\nSwitch to '%s' anyway? [y/n] ", m.confirmTarget)
		sb.WriteString(styleWarning.Render(prompt))
		return sb.String()
	}

	footerHeight := 2
	visibleHeight := m.height - footerHeight
	endIdx := m.viewportStartIdx + visibleHeight
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}

	for i := m.viewportStartIdx; i < endIdx; i++ {
		it := m.items[i]
		isSelected := i == m.cursor

		switch it.kind {
		case itemCategory:
			collapseMarker := "▼ "
			if m.collapsed[it.label] {
				collapseMarker = "▶ "
			}
			catText := styleCategory.Render(collapseMarker + it.label)
			if isSelected {
				catText = styleSelected.Render(collapseMarker + it.label)
			}
			sb.WriteString("- " + catText + "\n")

		case itemBranch:
			prefix := "  ├── "
			if it.isLast {
				prefix = "  └── "
			}
			marker := ""
			if it.isCurrent {
				marker = "● "
			}

			var branchText string
			if it.isCurrent {
				branchText = styleCurrent.Render(marker + it.label)
			} else {
				branchText = styleBranch.Render(marker + it.label)
			}

			if isSelected {
				branchText = styleSelected.Render(marker + it.label)
			}

			tagText := renderTagLipgloss(it.tag)

			row := prefix + branchText
			if tagText != "" {
				row += "  " + tagText
			}
			sb.WriteString(row + "\n")
		}
	}

	sb.WriteString("\n")
	if m.errMsg != "" {
		sb.WriteString(styleError.Render(m.errMsg) + "\n")
	}
	hint := styleMeta.Render("↑/↓ navigate  enter checkout  space/→ expand  q quit")
	sb.WriteString(hint + "\n")

	return sb.String()
}

func renderTagLipgloss(tag string) string {
	if tag == "" {
		return ""
	}

	var label, status string
	if strings.HasPrefix(tag, "[Local]") {
		label = "[Local]"
		status = tag[7:]
	} else if strings.HasPrefix(tag, "[Remote]") {
		label = "[Remote]"
		status = tag[8:]
	} else {
		return styleMeta.Render(tag)
	}

	var labelStr string
	if label == "[Local]" {
		labelStr = styleMeta.Render(label)
	} else {
		labelStr = styleRemote.Render(label)
	}

	if status == "" {
		return labelStr
	}

	var statusStr string
	if label == "[Remote]" {
		switch status {
		case " InSync":
			statusStr = styleInSync.Render(status)
		case " ?", "?":
			statusStr = styleUnknown.Render(status)
		default:
			hasAhead := strings.Contains(status, "↑")
			hasBehind := strings.Contains(status, "↓")
			switch {
			case hasAhead && hasBehind:
				statusStr = styleDiverged.Render(status)
			case hasAhead:
				statusStr = styleAhead.Render(status)
			case hasBehind:
				statusStr = styleBehind.Render(status)
			default:
				statusStr = styleMeta.Render(status)
			}
		}
	} else {
		statusStr = styleMeta.Render(status)
	}

	return labelStr + statusStr
}

// buildItems constructs the flat navigation list
func buildItems(
	categories []string,
	branchMap map[string][]string,
	currentBranch string,
	branchTags map[string]string,
) []item {
	var items []item
	for _, cat := range categories {
		items = append(items, item{
			kind:  itemCategory,
			label: cat,
		})
		branches := branchMap[cat]
		for i, branch := range branches {
			items = append(items, item{
				kind:      itemBranch,
				label:     branch,
				category:  cat,
				tag:       branchTags[branch],
				isCurrent: branch == currentBranch,
				isLast:    i == len(branches)-1,
			})
		}
	}
	return items
}

// rebuildItems rebuilds the item list when categories are collapsed/expanded
func (m *model) rebuildItems() []item {
	var items []item
	for _, cat := range m.categories {
		items = append(items, item{
			kind:  itemCategory,
			label: cat,
		})
		if m.collapsed[cat] {
			continue
		}
		branches := m.branchMap[cat]
		for i, branch := range branches {
			items = append(items, item{
				kind:      itemBranch,
				label:     branch,
				category:  cat,
				tag:       m.branchTags[branch],
				isCurrent: branch == m.currentBranch,
				isLast:    i == len(branches)-1,
			})
		}
	}
	return items
}

// RunTUI starts the interactive TUI
func RunTUI(
	gitDir string,
	categories []string,
	branchMap map[string][]string,
	currentBranch string,
	branchTags map[string]string,
) (checkedOut string, err error) {
	items := buildItems(categories, branchMap, currentBranch, branchTags)

	// Position cursor on current branch
	cursor := 0
	for i, it := range items {
		if it.kind == itemBranch && it.isCurrent {
			cursor = i
			break
		}
	}

	m := model{
		gitDir:           gitDir,
		categories:       categories,
		branchMap:        branchMap,
		branchTags:       branchTags,
		currentBranch:    currentBranch,
		items:            items,
		cursor:           cursor,
		collapsed:        make(map[string]bool),
		mode:             modeBrowse,
		viewportStartIdx: 0,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	if fm, ok := finalModel.(model); ok {
		return fm.checkedOut, nil
	}
	return "", nil
}
