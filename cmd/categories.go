package cmd

import (
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/siddhesh/gcm/internal/telemetry"
	"github.com/siddhesh/gcm/internal/ui"
	"github.com/spf13/cobra"
)

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List all available categories",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCategories(cmdTel, args)
	},
}

func runCategories(tel telemetry.Recorder, args []string) (err error) {
	defer func() { tel.Record("cmd_categories", map[string]any{"success": err == nil}) }()

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

	var categoryNames []string
	for _, cat := range s.Categories {
		categoryNames = append(categoryNames, cat.Name)
	}

	ui.PrintCategoryList(categoryNames)
	return nil
}

func init() {
	rootCmd.AddCommand(categoriesCmd)
}
