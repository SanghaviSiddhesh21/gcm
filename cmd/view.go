package cmd

import (
	"fmt"
	"os"

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

func init() {
	rootCmd.AddCommand(viewCmd)
}
