package cmd

import (
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <category>",
	Short: "Delete a category and move its branches to Uncategorized",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		categoryName := args[0]

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

		affectedBranches, err := s.BranchesInCategory(categoryName, allBranches)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		if err := s.RemoveCategory(categoryName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		if err := store.Save(repoInfo.GitDir, s); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		affectedCount := len(affectedBranches)
		if affectedCount > 0 {
			fmt.Printf("Deleted category '%s' and moved %d branch(es) to Uncategorized\n", categoryName, affectedCount)
		} else {
			fmt.Printf("Deleted category '%s'\n", categoryName)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
