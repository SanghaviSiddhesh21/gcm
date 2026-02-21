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

func PrintSuccess(msg string) {
	colorSuccess.Println(msg)
}

func PrintWarning(msg string) {
	colorWarning.Println(msg)
}

func PrintError(msg string) {
	colorError.Println(msg)
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

		colorMeta.Print("- ")
		if count == 0 {
			colorCategory.Printf("%s (empty)\n", category)
		} else {
			colorCategory.Printf("%s (%d %s)\n", category, count, countText)
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
				colorCurrentBranch.Printf("%s%s\n", marker, branch)
			} else {
				fmt.Printf("%s", prefix)
				colorBranch.Printf("%s%s\n", marker, branch)
			}
		}
	}
}

func PrintCategoryList(categories []string) {
	for _, category := range categories {
		colorCategory.Println(category)
	}
}
