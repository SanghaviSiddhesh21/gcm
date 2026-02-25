package cmd

import (
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "gcm",
	Short: "Git Category Manager — organize branches without renaming them",
	Long: `gcm — Git Category Manager

Organize your local git branches into categories without renaming them.
All metadata is stored in .git/gcm.json and never tracked by git.

Commands:
  gcm init                        Initialize gcm in the current repository
  gcm create <category>           Create a new category
  gcm assign <branch> <category>  Assign a branch to a category
  gcm view [category]             View branches organized by category
  gcm categories                  List all categories
  gcm delete <category>           Delete a category (branches move to Uncategorized)
  gcm commit                      Create a commit (wraps git commit)
  gcm commit -g                   Generate a commit message using local AI

Examples:
  gcm init
  gcm create feature
  gcm assign my-branch feature
  gcm view
  gcm view feature
  gcm commit -m "My message"
  gcm commit -g`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
