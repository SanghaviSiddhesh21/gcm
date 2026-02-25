package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddhesh/gcm/internal/ai"
	"github.com/siddhesh/gcm/internal/git"
)

// ErrCommitAborted is returned by RunCommitTUI when the user quits without committing.
var ErrCommitAborted = errors.New("commit aborted")

// commitMode discriminates between TUI states.
type commitMode int

const (
	modeGenerating  commitMode = iota // LLM is running
	modeReview                        // Message ready, awaiting user decision
	modeManualInput                   // Model absent or failed, asking for manual input
)

// manualInputReason controls which warning message is shown above the manual prompt.
type manualInputReason int

const (
	reasonModelNotFound    manualInputReason = iota
	reasonGenerationFailed
	reasonRateLimited
)

const maxGenerationRetries = 3

// Async message types --------------------------------------------------------

type generateResultMsg struct {
	message      string
	err          error
	generationID int // stale results (from cancelled generation) are discarded
}

type editorDoneMsg struct {
	message string
	err     error
}

type spinnerTickMsg struct{}

// Lipgloss styles ------------------------------------------------------------

var (
	styleCommitHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))  // cyan
	styleCommitMessage = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))  // green
	styleCommitWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Faint(true)  // muted yellow
	styleCommitError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // red
	styleCommitMeta    = lipgloss.NewStyle().Faint(true)
	styleCommitPrompt  = lipgloss.NewStyle().Bold(true)
	styleCommitFile    = lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // white
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// commitModel is the Bubbletea model for the commit TUI. -------------------

const largeDiffThreshold = 500

type commitModel struct {
	diff      string
	diffLines int // number of added+removed lines in the staged diff
	status    git.WorktreeStatus
	gen       ai.Generator

	mode         commitMode
	message      string            // current commit message (generated or edited)
	manualInput  string            // buffer for manual text entry
	manualReason manualInputReason // controls the warning shown above manual prompt
	retryCount   int
	generationID int // incremented on each regenerate to discard stale results
	spinnerFrame int

	cursorRow    int  // 0 = Staged row, 1 = Unstaged row (used for dropdown navigation)
	stagedOpen   bool // whether Staged files dropdown is expanded
	unstagedOpen bool // whether Unstaged files dropdown is expanded

	manualErrMsg string // inline validation error on manual input

	result  string // message to commit — set before tea.Quit
	aborted bool   // true when user quits without committing

	width  int
	height int
}

// Async command constructors ------------------------------------------------

func doGenerate(gen ai.Generator, diff string, id int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		msg, err := gen.Generate(ctx, diff)
		return generateResultMsg{message: msg, err: err, generationID: id}
	}
}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// Init starts the first generation attempt and the spinner. -----------------

func (m commitModel) Init() tea.Cmd {
	return tea.Batch(
		doGenerate(m.gen, m.diff, m.generationID),
		spinnerTick(),
	)
}

// Update handles all messages and key events. --------------------------------

func (m commitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinnerTickMsg:
		if m.mode == modeGenerating {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, spinnerTick()
		}
		return m, nil

	case generateResultMsg:
		// Discard results from a superseded generation (user pressed 'r')
		if msg.generationID != m.generationID {
			return m, nil
		}
		if msg.err == nil {
			m.mode = modeReview
			m.message = msg.message
			return m, nil
		}
		// Generation failed — retry or fall back to manual input
		if errors.Is(msg.err, ai.ErrNotConfigured) {
			m.mode = modeManualInput
			m.manualReason = reasonModelNotFound
			return m, nil
		}
		if errors.Is(msg.err, ai.ErrRateLimited) {
			m.mode = modeManualInput
			m.manualReason = reasonRateLimited
			return m, nil
		}
		m.retryCount++
		if m.retryCount < maxGenerationRetries {
			m.generationID++
			return m, doGenerate(m.gen, m.diff, m.generationID)
		}
		m.mode = modeManualInput
		m.manualReason = reasonGenerationFailed
		return m, nil

	case editorDoneMsg:
		if msg.err != nil || strings.TrimSpace(msg.message) == "" {
			// Editor produced empty or errored — stay in review with original message
			return m, nil
		}
		m.message = msg.message
		m.mode = modeReview
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey routes key events based on current mode. ------------------------

func (m commitModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C works in all modes
	if msg.String() == "ctrl+c" {
		m.aborted = true
		return m, tea.Quit
	}

	switch m.mode {

	case modeGenerating:
		// No keys active during generation except Ctrl+C (handled above)
		return m, nil

	case modeReview:
		switch msg.String() {
		case "y", "Y":
			m.result = m.message
			return m, tea.Quit

		case "q", "Q":
			m.aborted = true
			return m, tea.Quit

		case "r", "R":
			// Regenerate: increment ID so the previous result is discarded, restart
			m.generationID++
			m.retryCount = 0
			m.mode = modeGenerating
			return m, tea.Batch(
				doGenerate(m.gen, m.diff, m.generationID),
				spinnerTick(),
			)

		case "e", "E":
			return m.openEditor()

		case "up":
			if m.cursorRow > 0 {
				m.cursorRow--
			}
			return m, nil

		case "down":
			if m.cursorRow < 1 {
				m.cursorRow++
			}
			return m, nil

		case " ", "enter":
			switch m.cursorRow {
			case 0:
				m.stagedOpen = !m.stagedOpen
			case 1:
				m.unstagedOpen = !m.unstagedOpen
			}
			return m, nil
		}

	case modeManualInput:
		switch msg.String() {
		case "enter":
			trimmed := strings.TrimSpace(m.manualInput)
			if trimmed == "" {
				m.manualErrMsg = "Commit message cannot be empty"
				return m, nil
			}
			m.result = trimmed
			return m, tea.Quit

		case "backspace":
			if len(m.manualInput) > 0 {
				m.manualInput = m.manualInput[:len(m.manualInput)-1]
				m.manualErrMsg = ""
			}
			return m, nil

		default:
			// Accept printable characters
			if len(msg.String()) == 1 {
				m.manualInput += msg.String()
				m.manualErrMsg = ""
			}
			return m, nil
		}
	}

	return m, nil
}

// openEditor suspends the TUI, opens $EDITOR with the current message,
// then resumes and returns the edited content.
func (m commitModel) openEditor() (tea.Model, tea.Cmd) {
	tmpFile, err := os.CreateTemp("", "gcm-commit-*.txt")
	if err != nil {
		return m, nil
	}
	if _, err := tmpFile.WriteString(m.message); err != nil {
		tmpFile.Close()
		return m, nil
	}
	tmpFile.Close()

	name := tmpFile.Name()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	return m, tea.ExecProcess(exec.Command(editor, name), func(err error) tea.Msg {
		defer os.Remove(name)
		if err != nil {
			return editorDoneMsg{err: err}
		}
		content, readErr := os.ReadFile(name)
		if readErr != nil {
			return editorDoneMsg{err: readErr}
		}
		return editorDoneMsg{message: strings.TrimSpace(string(content))}
	})
}

// View renders the current TUI state. ----------------------------------------

func (m commitModel) View() string {
	var sb strings.Builder

	switch m.mode {

	case modeGenerating:
		frame := spinnerFrames[m.spinnerFrame]
		sb.WriteString(styleCommitMeta.Render(fmt.Sprintf("%s Generating commit message...\n", frame)))

	case modeReview:
		// Staged files dropdown
		stagedArrow := "▼"
		if m.stagedOpen {
			stagedArrow = "▲"
		}
		stagedHeader := fmt.Sprintf("Staged files (%d)   %s", len(m.status.Staged), stagedArrow)
		if m.cursorRow == 0 {
			sb.WriteString(styleCommitHeader.Render("> "+stagedHeader) + "\n")
		} else {
			sb.WriteString(styleCommitMeta.Render("  "+stagedHeader) + "\n")
		}
		if m.stagedOpen {
			for _, f := range m.status.Staged {
				sb.WriteString(styleCommitFile.Render("    " + f) + "\n")
			}
		}

		// Unstaged files dropdown
		unstagedArrow := "▼"
		if m.unstagedOpen {
			unstagedArrow = "▲"
		}
		unstagedHeader := fmt.Sprintf("Unstaged files (%d) %s", len(m.status.Unstaged), unstagedArrow)
		if m.cursorRow == 1 {
			sb.WriteString(styleCommitHeader.Render("> "+unstagedHeader) + "\n")
		} else {
			sb.WriteString(styleCommitMeta.Render("  "+unstagedHeader) + "\n")
		}
		if m.unstagedOpen {
			for _, f := range m.status.Unstaged {
				sb.WriteString(styleCommitFile.Render("    " + f) + "\n")
			}
		}

		// Large diff warning
		if m.diffLines > largeDiffThreshold {
			sb.WriteString(styleCommitWarning.Render(fmt.Sprintf("⚠ Large changeset (~%d lines) — generated message may not capture all changes. Press 'e' to refine or 'r' to retry.", m.diffLines)) + "\n")
		}

		// Generated message
		sb.WriteString("\n")
		sb.WriteString(styleCommitMeta.Render("Generated message:") + "\n")
		sb.WriteString(styleCommitMessage.Render(m.message) + "\n")
		sb.WriteString("\n")
		sb.WriteString(styleCommitPrompt.Render("Accept? (y/e/r/q): "))

		sb.WriteString("\n\n")
		sb.WriteString(styleCommitMeta.Render("↑/↓ navigate dropdowns  space/enter expand  y accept  e edit  r regenerate  q quit"))

	case modeManualInput:
		// Warning message varies based on reason
		switch m.manualReason {
		case reasonModelNotFound:
			sb.WriteString(styleCommitError.Render("✗ Error: \"AI not configured\"") + "\n")
			sb.WriteString(styleCommitWarning.Render("⚠  GCM_API_KEY not set. Run: gcm config set api-key <your-groq-key>") + "\n")
		case reasonGenerationFailed:
			sb.WriteString(styleCommitError.Render("✗ Error: \"Failed to generate commit message\"") + "\n")
			sb.WriteString(styleCommitWarning.Render("⚠  Message generation failed. Please enter the commit message manually:") + "\n")
		case reasonRateLimited:
			sb.WriteString(styleCommitError.Render("✗ Error: \"Rate limit exceeded\"") + "\n")
			sb.WriteString(styleCommitWarning.Render("⚠  Too many requests on the shared key. Set your own: gcm config set api-key <your-groq-key>") + "\n")
		}
		sb.WriteString("\n")
		sb.WriteString(styleCommitPrompt.Render("Enter commit message manually: "))
		sb.WriteString(m.manualInput)
		sb.WriteString("█\n") // block cursor

		if m.manualErrMsg != "" {
			sb.WriteString(styleCommitError.Render(m.manualErrMsg) + "\n")
		}
	}

	return sb.String()
}

// RunCommitTUI launches the interactive commit TUI. --------------------------
// Returns the final commit message, or ErrCommitAborted if the user quits.
// countDiffLines counts added and removed lines in a diff.
func countDiffLines(diff string) int {
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		if len(line) > 0 && (line[0] == '+' || line[0] == '-') &&
			!strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
			count++
		}
	}
	return count
}

func RunCommitTUI(_ string, diff string, status git.WorktreeStatus, gen ai.Generator) (string, error) {
	m := commitModel{
		diff:      diff,
		diffLines: countDiffLines(diff),
		status:    status,
		gen:       gen,
		mode:      modeGenerating,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	fm, ok := finalModel.(commitModel)
	if !ok {
		return "", ErrCommitAborted
	}
	if fm.aborted || fm.result == "" {
		return "", ErrCommitAborted
	}
	return fm.result, nil
}
