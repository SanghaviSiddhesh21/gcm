package cmd

import (
	"fmt"
	"strings"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a git repository and set up GCM",
	Long: `Initialize a git repository and set up GCM.

Runs 'git init [args...]' then initializes the GCM store.
All arguments are forwarded verbatim to git init.`,
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if checkHelp(args) {
			return cmd.Help()
		}

		if err := passthroughGit(globalGitFlags, append([]string{"init"}, args...)); err != nil {
			return err
		}

		// Determine target directory from positional args.
		// Known init flags that consume the next token as a value.
		knownInitValueFlags := map[string]bool{
			"--template": true, "--separate-git-dir": true,
			"-b": true, "--initial-branch": true,
		}
		targetDir := "."
		skipNext := false
		for _, arg := range args {
			if skipNext {
				skipNext = false
				continue
			}
			if knownInitValueFlags[arg] {
				skipNext = true
				continue
			}
			if !strings.HasPrefix(arg, "-") {
				targetDir = arg
				break
			}
		}

		// Locate the git dir in the target directory.
		gitDir, err := git.GetGitDirAt(targetDir)
		if err != nil {
			return fmt.Errorf("gcm init: could not locate git dir: %w", err)
		}

		// Create the store if not already present.
		if _, err := store.LoadOrCreate(gitDir); err != nil {
			return fmt.Errorf("gcm init: failed to initialize store: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
