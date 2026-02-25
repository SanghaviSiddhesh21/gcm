package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/mattn/go-isatty"
	"github.com/siddhesh/gcm/internal/ai"
	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/ui"
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Create a commit, optionally with AI-generated message (-g)",
	Long: `Create a git commit.

Without -g, all arguments are passed directly to git commit:
  gcm commit -m "My message"
  gcm commit --amend
  gcm commit -a -m "My message"

With -g, generate a commit message using local AI (offline):
  gcm commit -g

Note: -g cannot be combined with other flags.`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, arg := range args {
			if arg == "--help" || arg == "-h" {
				return cmd.Help()
			}
		}

		if !containsGenerateFlag(args) {
			return passthroughGitCommit(args)
		}

		// -g must be the only argument
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "error: '-g' cannot be combined with other flags\n\n")
			return cmd.Help()
		}

		return runGenerateCommit()
	},
}

// containsGenerateFlag reports whether -g is present in args.
func containsGenerateFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-g" {
			return true
		}
	}
	return false
}

// passthroughGitCommit shells directly to git commit with the provided args,
// forwarding stdin/stdout/stderr so interactive git flows work correctly.
func passthroughGitCommit(args []string) error {
	c := exec.Command("git", append([]string{"commit"}, args...)...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// runGenerateCommit runs the AI generation flow: validate → diff → TUI → commit.
func runGenerateCommit() error {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		fmt.Fprintln(os.Stderr, "Error: gcm commit requires a terminal")
		return fmt.Errorf("not a terminal")
	}

	repoInfo, err := git.GetRepoInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}

	diff, err := git.GetStagedChanges(repoInfo.GitDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}
	if diff == "" {
		fmt.Fprintln(os.Stderr, "Error: No staged changes")
		return fmt.Errorf("no staged changes")
	}

	status, err := git.GetWorktreeStatus(repoInfo.GitDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}

	message, err := ui.RunCommitTUI(repoInfo.GitDir, diff, status, ai.New())
	if err != nil {
		if errors.Is(err, ui.ErrCommitAborted) {
			return nil // user quit — not an error condition
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}

	if err := git.Commit(repoInfo.GitDir, message); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}

	ui.PrintSuccess("Commit created successfully")
	return nil
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
