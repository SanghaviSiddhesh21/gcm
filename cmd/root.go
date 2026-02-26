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

gcm is a drop-in replacement for git. Any command not natively defined in
gcm is forwarded to git with all arguments, flags, stdin/stdout/stderr, and
exit codes passed through unchanged.

Native commands:
  gcm init                        Initialize gcm (also runs git init)
  gcm create <category>           Create a new category
  gcm assign <branch> <category>  Assign a branch to a category
  gcm view [category]             View branches organized by category
  gcm categories                  List all categories
  gcm delete <category>           Delete a category (branches move to Uncategorized)
  gcm commit                      Create a commit (wraps git commit)
  gcm commit -g                   Generate a commit message using local AI
  gcm clone <url> [dir]           Clone a repository and initialize gcm
  gcm branch                      Manage branches (wraps git branch)

Git passthrough (examples):
  gcm push origin main
  gcm status
  gcm log --oneline
  gcm fetch --all
  gcm add -p
  gcm rebase -i HEAD~3`,
	Version:            version,
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Intercept --version / -v before passthrough (DisableFlagParsing swallows it).
		if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
			cmd.Println("gcm version " + version)
			return nil
		}

		// Intercept --help / -h (stops at -- separator).
		// Routes through runHelpAll so `gcm --help` == `gcm help`.
		if checkHelp(args) {
			return runHelpAll(cmd)
		}

		if len(args) == 0 {
			return cmd.Help()
		}

		return passthroughGit(globalGitFlags, args)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
