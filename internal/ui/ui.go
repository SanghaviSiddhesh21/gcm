package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var (
	colorCategory      = color.New(color.FgCyan, color.Bold)
	colorBranch        = color.New(color.FgWhite)
	colorCurrentBranch = color.New(color.FgGreen, color.Bold)
	colorMeta          = color.New(color.Faint)
	colorSuccess       = color.New(color.FgGreen)
	colorWarning       = color.New(color.FgYellow)
	colorError         = color.New(color.FgRed)
	colorRemoteLabel   = color.New(color.FgMagenta)
	colorInSync        = color.New(color.FgGreen)
	colorAhead         = color.New(color.FgBlue)
	colorBehind        = color.New(color.FgYellow)
	colorDiverged      = color.New(color.FgRed)
	colorUnknown       = color.New(color.FgYellow)
)

// cprint, cprintf, cprintln are thin wrappers that discard the error returned
// by fatih/color's print methods. Writing to a terminal never meaningfully fails
// in a CLI context, so checking these errors everywhere would be pure noise.
func cprint(c *color.Color, msg string)          { _, _ = c.Print(msg) }
func cprintf(c *color.Color, f string, a ...any) { _, _ = c.Printf(f, a...) }
func cprintln(c *color.Color, msg string)        { _, _ = c.Println(msg) }

// renderBranchTag prints a branch tag with separate colors for label and status.
func renderBranchTag(tag string) {
	if tag == "" {
		return
	}

	// Parse label and status
	var label, status string
	if len(tag) > 7 && tag[:7] == "[Local]" {
		label = "[Local]"
		status = tag[7:] // everything after [Local]
	} else if len(tag) > 8 && tag[:8] == "[Remote]" {
		label = "[Remote]"
		status = tag[8:] // everything after [Remote]
	} else {
		// Fallback: render entire tag in meta color
		cprint(colorMeta, tag)
		return
	}

	// Render label
	if label == "[Local]" {
		cprint(colorMeta, label)
	} else {
		cprint(colorRemoteLabel, label)
	}

	// Render status with appropriate color
	if status == "" {
		return
	}

	statusColor := colorMeta // default
	if label == "[Remote]" {
		switch status {
		case " InSync":
			statusColor = colorInSync
		case " ?", "?":
			statusColor = colorUnknown
		default:
			hasAhead := strings.Contains(status, "↑")
			hasBehind := strings.Contains(status, "↓")
			if hasAhead && hasBehind {
				statusColor = colorDiverged
			} else if hasAhead {
				statusColor = colorAhead
			} else if hasBehind {
				statusColor = colorBehind
			}
		}
	}

	cprint(statusColor, status)
}

func PrintSuccess(msg string) {
	cprintln(colorSuccess, msg)
}

func PrintWarning(msg string) {
	cprintln(colorWarning, msg)
}

func PrintError(msg string) {
	cprintln(colorError, msg)
}

func PrintTree(categories []string, branches map[string][]string, currentBranch string, branchTags map[string]string) {
	for _, category := range categories {
		catBranches, exists := branches[category]
		if !exists {
			continue
		}

		count := len(catBranches)
		countText := "branch"
		if count != 1 {
			countText = "branches"
		}

		cprint(colorMeta, "- ")
		if count == 0 {
			cprintf(colorCategory, "%s (empty)\n", category)
		} else {
			cprintf(colorCategory, "%s (%d %s)\n", category, count, countText)
		}

		for i, branch := range catBranches {
			isLast := i == len(catBranches)-1
			prefix := "  ├── "
			if isLast {
				prefix = "  └── "
			}

			marker := ""
			if branch == currentBranch {
				marker = "● "
			}

			fmt.Printf("%s", prefix)
			if branch == currentBranch {
				cprintf(colorCurrentBranch, "%s%s", marker, branch)
			} else {
				cprintf(colorBranch, "%s%s", marker, branch)
			}
			if tag := branchTags[branch]; tag != "" {
				fmt.Print("  ")
				renderBranchTag(tag)
			}
			fmt.Println()
		}
	}
}

func PrintCategoryList(categories []string) {
	for _, category := range categories {
		cprintln(colorCategory, category)
	}
}
