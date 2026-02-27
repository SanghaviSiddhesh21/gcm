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

var ErrCommitAborted = errors.New("commit aborted")

// CommitOutcome is the typed result of a commit-g session.
type CommitOutcome string

const (
	OutcomeAccepted CommitOutcome = "accepted"
	OutcomeEdited   CommitOutcome = "edited"
	OutcomeAborted  CommitOutcome = "aborted"
	OutcomeManual   CommitOutcome = "manual"
)

// CommitResult carries the outcome of a RunCommitTUI session.
type CommitResult struct {
	Message       string
	Outcome       CommitOutcome
	Regenerations int // count of user 'r' presses only; internal retries-on-failure excluded
}

type commitMode int

const (
	modeGenerating commitMode = iota
	modeReview
	modeManualInput
)

type manualInputReason int

const (
	reasonModelNotFound manualInputReason = iota
	reasonGenerationFailed
	reasonRateLimited
)

const maxGenerationRetries = 3

type generateResultMsg struct {
	message      string
	err          error
	generationID int // stale results from a superseded generation are discarded
}

type editorDoneMsg struct {
	message string
	err     error
}

type spinnerTickMsg struct{}

var (
	styleCommitHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))  // cyan
	styleCommitMessage = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))  // green
	styleCommitWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Faint(true) // muted yellow
	styleCommitError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // red
	styleCommitMeta    = lipgloss.NewStyle().Faint(true)
	styleCommitPrompt  = lipgloss.NewStyle().Bold(true)
	styleCommitFile    = lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // white
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const largeDiffThreshold = 500

type commitModel struct {
	diff      string
	diffLines int
	status    git.WorktreeStatus
	gen       ai.Generator

	mode         commitMode
	message      string
	manualInput  string
	manualReason manualInputReason
	retryCount   int
	generationID int
	spinnerFrame int

	cursorRow    int
	stagedOpen   bool
	unstagedOpen bool

	manualErrMsg string

	result         string
	aborted        bool
	regenerations  int  // user 'r' presses only (not internal retry-on-failure)
	usedEditor     bool // set when editorDoneMsg arrives with a non-empty message
	exitedManually bool // set when modeManualInput produces a result via Enter

	width  int
	height int
}

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

func (m commitModel) Init() tea.Cmd {
	return tea.Batch(
		doGenerate(m.gen, m.diff, m.generationID),
		spinnerTick(),
	)
}

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
		if msg.generationID != m.generationID {
			return m, nil
		}
		if msg.err == nil {
			m.mode = modeReview
			m.message = msg.message
			return m, nil
		}
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
			return m, nil
		}
		m.message = msg.message
		m.usedEditor = true
		m.mode = modeReview
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m commitModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.aborted = true
		return m, tea.Quit
	}

	switch m.mode {

	case modeGenerating:
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
			m.generationID++
			m.retryCount = 0
			m.mode = modeGenerating
			m.regenerations++ // user-initiated regeneration only (not internal retry-on-failure)
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
			m.exitedManually = true
			return m, tea.Quit

		case "backspace":
			if len(m.manualInput) > 0 {
				m.manualInput = m.manualInput[:len(m.manualInput)-1]
				m.manualErrMsg = ""
			}
			return m, nil

		default:
			if len(msg.String()) == 1 {
				m.manualInput += msg.String()
				m.manualErrMsg = ""
			}
			return m, nil
		}
	}

	return m, nil
}

func (m commitModel) openEditor() (tea.Model, tea.Cmd) {
	tmpFile, err := os.CreateTemp("", "gcm-commit-*.txt")
	if err != nil {
		return m, nil
	}
	if _, err := tmpFile.WriteString(m.message); err != nil {
		tmpFile.Close() //nolint:errcheck,gosec
		return m, nil
	}
	tmpFile.Close() //nolint:errcheck,gosec

	name := tmpFile.Name()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	return m, tea.ExecProcess(exec.Command(editor, name), func(err error) tea.Msg { //nolint:gosec // editor is from $EDITOR env var — launching user's chosen editor is by design
		defer os.Remove(name) //nolint:errcheck
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

func (m commitModel) View() string {
	var sb strings.Builder

	switch m.mode {

	case modeGenerating:
		frame := spinnerFrames[m.spinnerFrame]
		sb.WriteString(styleCommitMeta.Render(fmt.Sprintf("%s Generating commit message...\n", frame)))

	case modeReview:
		stagedArrow := "▶ "
		if m.stagedOpen {
			stagedArrow = "▼ "
		}
		stagedHeader := fmt.Sprintf("Staged files (%d)", len(m.status.Staged))
		if m.cursorRow == 0 {
			sb.WriteString("- " + styleSelected.Render(stagedArrow+stagedHeader) + "\n")
		} else {
			sb.WriteString("- " + styleCommitHeader.Render(stagedArrow+stagedHeader) + "\n")
		}
		if m.stagedOpen {
			for i, f := range m.status.Staged {
				prefix := "  ├── "
				if i == len(m.status.Staged)-1 {
					prefix = "  └── "
				}
				sb.WriteString(styleCommitFile.Render(prefix+f) + "\n")
			}
		}

		unstagedArrow := "▶ "
		if m.unstagedOpen {
			unstagedArrow = "▼ "
		}
		unstagedHeader := fmt.Sprintf("Unstaged files (%d)", len(m.status.Unstaged))
		if m.cursorRow == 1 {
			sb.WriteString("- " + styleSelected.Render(unstagedArrow+unstagedHeader) + "\n")
		} else {
			sb.WriteString("- " + styleCommitHeader.Render(unstagedArrow+unstagedHeader) + "\n")
		}
		if m.unstagedOpen {
			for i, f := range m.status.Unstaged {
				prefix := "  ├── "
				if i == len(m.status.Unstaged)-1 {
					prefix = "  └── "
				}
				sb.WriteString(styleCommitFile.Render(prefix+f) + "\n")
			}
		}

		if m.diffLines > largeDiffThreshold {
			sb.WriteString(styleCommitWarning.Render(fmt.Sprintf("⚠ Large changeset (~%d lines) — generated message may not capture all changes. Press 'e' to refine or 'r' to retry.", m.diffLines)) + "\n")
		}

		sb.WriteString("\n")
		sb.WriteString(styleCommitMeta.Render("Generated message:") + "\n")
		sb.WriteString(styleCommitMessage.Render(m.message) + "\n")
		sb.WriteString("\n")
		sb.WriteString(styleCommitPrompt.Render("Accept? (y/e/r/q): "))

		sb.WriteString("\n\n")
		sb.WriteString(styleCommitMeta.Render("↑/↓ navigate  space/enter expand  y accept  e edit  r regenerate  q quit"))

	case modeManualInput:
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

func RunCommitTUI(_ string, diff string, status git.WorktreeStatus, gen ai.Generator) (CommitResult, error) {
	m := commitModel{
		diff:      diff,
		diffLines: countDiffLines(diff),
		status:    status,
		gen:       gen,
		mode:      modeGenerating,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return CommitResult{Outcome: OutcomeAborted}, err
	}

	fm, ok := finalModel.(commitModel)
	if !ok {
		return CommitResult{Outcome: OutcomeAborted}, ErrCommitAborted
	}
	if fm.aborted || fm.result == "" {
		return CommitResult{Outcome: OutcomeAborted, Regenerations: fm.regenerations}, ErrCommitAborted
	}

	var outcome CommitOutcome
	switch {
	case fm.usedEditor:
		outcome = OutcomeEdited
	case fm.exitedManually:
		outcome = OutcomeManual
	default:
		outcome = OutcomeAccepted
	}
	return CommitResult{
		Message:       fm.result,
		Outcome:       outcome,
		Regenerations: fm.regenerations,
	}, nil
}
