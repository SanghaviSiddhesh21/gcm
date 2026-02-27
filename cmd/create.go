package cmd

import (
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/siddhesh/gcm/internal/telemetry"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <category>",
	Short: "Create a new category",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing argument: category name\n\nUsage: gcm create <category>\nExample: gcm create feature")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreate(cmdTel, args)
	},
}

func runCreate(tel telemetry.Recorder, args []string) (err error) {
	defer func() { tel.Record("cmd_create", map[string]any{"success": err == nil}) }()

	categoryName := args[0]

	repoInfo, err := git.GetRepoInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}

	s, err := store.LoadOrCreate(repoInfo.GitDir)
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
}

func init() {
	rootCmd.AddCommand(createCmd)
}
