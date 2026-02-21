package cmd

import (
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
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
				fmt.Fprintf(os.Stderr, "Error: category '%s' not found\n", filterCategory)
				return fmt.Errorf("category not found: %s", filterCategory)
			}
		}

		for _, cat := range s.Categories {
			if filterCategory != "" && cat.Name != filterCategory {
				continue
			}

			branches, err := s.BranchesInCategory(cat.Name, allBranches)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				return err
			}

			fmt.Println(cat.Name)

			for i, branch := range branches {
				isLast := i == len(branches)-1
				prefix := "├── "
				if isLast {
					prefix = "└── "
				}

				marker := ""
				if branch == currentBranch {
					marker = "● "
				}

				fmt.Printf("%s%s%s\n", prefix, marker, branch)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
}
