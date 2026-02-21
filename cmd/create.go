package cmd

import (
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <category>",
	Short: "Create a new category",
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

		if err := s.AddCategory(categoryName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		if err := store.Save(repoInfo.GitDir, s); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return err
		}

		fmt.Printf("Created category '%s'\n", categoryName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
