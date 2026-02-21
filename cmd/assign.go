package cmd

import (
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/spf13/cobra"
)

var assignCmd = &cobra.Command{
	Use:   "assign <branch> <category>",
	Short: "Assign a branch to a category",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing arguments: branch and category are required\n\nUsage: gcm assign <branch> <category>\nExample: gcm assign my-feature feature")
		}
		if len(args) == 1 {
			return fmt.Errorf("missing argument: category is required\n\nUsage: gcm assign <branch> <category>\nExample: gcm assign %s feature", args[0])
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		categoryName := args[1]

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

		exists, err := git.BranchExists(repoInfo.GitDir, branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}
		if !exists {
			fmt.Fprintf(os.Stderr, "Error: branch '%s' not found\n", branch)
			return fmt.Errorf("branch not found: %s", branch)
		}

		if !s.CategoryExists(categoryName) {
			fmt.Fprintf(os.Stderr, "Error: category '%s' not found\n", categoryName)
			return store.ErrCategoryNotFound
		}

		oldCategory := s.GetAssignment(branch)

		if err := s.AssignBranch(branch, categoryName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		if err := store.Save(repoInfo.GitDir, s); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		switch oldCategory {
		case categoryName:
			fmt.Printf("Branch '%s' already in category '%s'\n", branch, categoryName)
		case store.UncategorizedName:
			fmt.Printf("Assigned '%s' to '%s'\n", branch, categoryName)
		default:
			fmt.Printf("Reassigned '%s' from '%s' to '%s'\n", branch, oldCategory, categoryName)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(assignCmd)
}
