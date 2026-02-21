package ui

import (
	"fmt"

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
)

// cprint, cprintf, cprintln are thin wrappers that discard the error returned
// by fatih/color's print methods. Writing to a terminal never meaningfully fails
// in a CLI context, so checking these errors everywhere would be pure noise.
func cprint(c *color.Color, msg string)                  { _, _ = c.Print(msg) }
func cprintf(c *color.Color, f string, a ...any)         { _, _ = c.Printf(f, a...) }
func cprintln(c *color.Color, msg string)                { _, _ = c.Println(msg) }

func PrintSuccess(msg string) {
	cprintln(colorSuccess, msg)
}

func PrintWarning(msg string) {
	cprintln(colorWarning, msg)
}

func PrintError(msg string) {
	cprintln(colorError, msg)
}

func PrintTree(categories []string, branches map[string][]string, currentBranch string) {
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

			if branch == currentBranch {
				fmt.Printf("%s", prefix)
				cprintf(colorCurrentBranch, "%s%s\n", marker, branch)
			} else {
				fmt.Printf("%s", prefix)
				cprintf(colorBranch, "%s%s\n", marker, branch)
			}
		}
	}
}

func PrintCategoryList(categories []string) {
	for _, category := range categories {
		cprintln(colorCategory, category)
	}
}
