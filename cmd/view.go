package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/siddhesh/gcm/internal/ui"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view [category]",
	Short: "View branches organized by category",
	Long: `View branches organized by category.

Branch status labels:
  [Local]        Branch exists only on your machine — not pushed to remote yet
  [Remote] InSync  Pushed to remote, local and remote are identical
  [Remote] ↑N    Pushed to remote, you have N local commits not yet pushed
  [Remote] ↓N    Pushed to remote, remote has N commits you have not pulled yet
  [Remote] ↑A ↓B Pushed to remote, diverged — A commits ahead and B commits behind
  [Remote] ?     Pushed to remote, but sync status could not be determined

Color scheme:
  [Local] label                 Grey (not pushed)
  [Remote] label                Magenta (pushed to remote)
  InSync status                 Green (in sync)
  ↑N status (ahead)             Blue (needs push)
  ↓N status (behind)            Yellow (needs pull)
  ↑A ↓B status (diverged)       Red (diverged)
  ? status (unknown)            Yellow (status could not be determined)

Note: status labels reflect your last fetch. To get up-to-date information, run:
  git fetch --prune

Note: branches whose remote was deleted will show as [Local] after git fetch --prune.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoInfo, err := git.GetRepoInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		s, err := store.Load(repoInfo.GitDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		allBranches, err := git.ListBranches(repoInfo.GitDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		currentBranch, err := git.CurrentBranch(repoInfo.GitDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		var filterCategory string
		if len(args) > 0 {
			filterCategory = args[0]
			if !s.CategoryExists(filterCategory) {
				ui.PrintError(fmt.Sprintf("category '%s' not found", filterCategory))
				return fmt.Errorf("category not found: %s", filterCategory)
			}
		}

		branchMap := make(map[string][]string)
		var categoryNames []string

		for _, cat := range s.Categories {
			if filterCategory != "" && cat.Name != filterCategory {
				continue
			}

			branches, err := s.BranchesInCategory(cat.Name, allBranches)
			if err != nil {
				ui.PrintError(err.Error())
				return err
			}

			branchMap[cat.Name] = branches
			categoryNames = append(categoryNames, cat.Name)
		}

		remoteBranches, err := git.ListRemoteBranches(repoInfo.GitDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}
		remoteSet := make(map[string]bool, len(remoteBranches))
		for _, b := range remoteBranches {
			remoteSet[b] = true
		}

		branchTags := make(map[string]string)
		for _, branch := range allBranches {
			if !remoteSet[branch] {
				branchTags[branch] = "[Local]"
				continue
			}
			ahead, behind, err := git.SyncStatus(repoInfo.GitDir, branch)
			if err != nil {
				branchTags[branch] = "[Remote] ?"
				continue
			}
			branchTags[branch] = formatSyncTag(ahead, behind)
		}

		branchTimes, err := git.BranchCommitTimes(repoInfo.GitDir)
		if err != nil {
			branchTimes = make(map[string]time.Time) // graceful degradation: preserve existing order
		}
		categoryNames, branchMap = sortView(categoryNames, branchMap, currentBranch, branchTimes)

		if isatty.IsTerminal(os.Stdout.Fd()) {
			checkedOut, err := ui.RunTUI(repoInfo.GitDir, categoryNames, branchMap, currentBranch, branchTags)
			if err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
			if checkedOut != "" {
				ui.PrintSuccess(fmt.Sprintf("Switched to branch '%s'", checkedOut))
			}
			return nil
		}

		ui.PrintTree(categoryNames, branchMap, currentBranch, branchTags)
		return nil
	},
}

func formatSyncTag(ahead, behind int) string {
	switch {
	case ahead == 0 && behind == 0:
		return "[Remote] InSync"
	case ahead > 0 && behind == 0:
		return fmt.Sprintf("[Remote] ↑%d", ahead)
	case ahead == 0 && behind > 0:
		return fmt.Sprintf("[Remote] ↓%d", behind)
	default:
		return fmt.Sprintf("[Remote] ↑%d ↓%d", ahead, behind)
	}
}

// sortView reorders categoryNames and branches within each category per the 5 display rules.
func sortView(
	categoryNames []string,
	branchMap map[string][]string,
	currentBranch string,
	branchTimes map[string]time.Time,
) ([]string, map[string][]string) {
	for cat, branches := range branchMap {
		branchMap[cat] = sortedBranches(branches, currentBranch, branchTimes)
	}
	currentCat := findCurrentCategory(categoryNames, branchMap, currentBranch)
	return sortedCategories(categoryNames, currentCat, branchMap, branchTimes), branchMap
}

// sortedBranches returns branches with currentBranch first (if present),
// then remaining branches sorted newest commit first.
func sortedBranches(branches []string, currentBranch string, branchTimes map[string]time.Time) []string {
	if len(branches) <= 1 {
		return branches
	}
	result := make([]string, 0, len(branches))
	others := make([]string, 0, len(branches))
	for _, b := range branches {
		if b == currentBranch {
			result = append(result, b)
		} else {
			others = append(others, b)
		}
	}
	sort.Slice(others, func(i, j int) bool {
		return branchTimes[others[i]].After(branchTimes[others[j]])
	})
	return append(result, others...)
}

// findCurrentCategory returns the category name containing currentBranch, or "" if not found.
func findCurrentCategory(categoryNames []string, branchMap map[string][]string, currentBranch string) string {
	for _, cat := range categoryNames {
		for _, b := range branchMap[cat] {
			if b == currentBranch {
				return cat
			}
		}
	}
	return ""
}

// sortedCategories returns categoryNames reordered:
//  1. Current branch's category first
//  2. Other named categories sorted by most recent branch commit (newest first)
//  3. Uncategorized last — unless it IS the current branch's category (Problem 1 wins)
func sortedCategories(
	categoryNames []string,
	currentCat string,
	branchMap map[string][]string,
	branchTimes map[string]time.Time,
) []string {
	hasUncategorized := false
	others := make([]string, 0, len(categoryNames))
	for _, cat := range categoryNames {
		if cat == currentCat {
			continue
		}
		if cat == store.UncategorizedName {
			hasUncategorized = true
			continue
		}
		others = append(others, cat)
	}
	sort.Slice(others, func(i, j int) bool {
		return maxCatTime(others[i], branchMap, branchTimes).After(
			maxCatTime(others[j], branchMap, branchTimes))
	})
	result := make([]string, 0, len(categoryNames))
	if currentCat != "" {
		result = append(result, currentCat)
	}
	result = append(result, others...)
	if hasUncategorized && currentCat != store.UncategorizedName {
		result = append(result, store.UncategorizedName)
	}
	return result
}

// maxCatTime returns the most recent commit time among all branches in a category.
func maxCatTime(cat string, branchMap map[string][]string, branchTimes map[string]time.Time) time.Time {
	var t time.Time
	for _, b := range branchMap[cat] {
		if bt := branchTimes[b]; bt.After(t) {
			t = bt
		}
	}
	return t
}

func init() {
	rootCmd.AddCommand(viewCmd)
}
