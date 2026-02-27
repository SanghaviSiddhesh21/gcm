package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/siddhesh/gcm/internal/ai"
	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/telemetry"
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
		if checkHelp(args) {
			return cmd.Help()
		}

		if !containsGenerateFlag(args) {
			return passthroughGitCommit(args)
		}

		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "error: '-g' cannot be combined with other flags\n\n")
			return cmd.Help()
		}

		return runGenerateCommit(cmdTel)
	},
}

func containsGenerateFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-g" {
			return true
		}
	}
	return false
}

func passthroughGitCommit(args []string) error {
	return passthroughGit(globalGitFlags, append([]string{"commit"}, args...))
}

func runGenerateCommit(tel telemetry.Recorder) error {
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

	result, err := ui.RunCommitTUI(repoInfo.GitDir, diff, status, ai.New())
	if err != nil {
		if errors.Is(err, ui.ErrCommitAborted) {
			tel.Record("cmd_commit_ai", map[string]any{
				"outcome":       string(ui.OutcomeAborted),
				"regenerations": result.Regenerations,
				"success":       true,
			})
			return nil // user quit — not an error condition
		}
		tel.Record("cmd_commit_ai", map[string]any{
			"outcome": string(ui.OutcomeManual),
			"success": false,
		})
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}

	tel.Record("cmd_commit_ai", map[string]any{
		"outcome":       string(result.Outcome),
		"regenerations": result.Regenerations,
		"success":       true,
	})

	if err := git.Commit(repoInfo.GitDir, result.Message); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return err
	}

	ui.PrintSuccess("Commit created successfully")
	return nil
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
