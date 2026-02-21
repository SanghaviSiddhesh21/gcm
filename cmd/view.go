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
	Args:  cobra.MaximumNArgs(1),
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

		ui.PrintTree(categoryNames, branchMap, currentBranch)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
}
