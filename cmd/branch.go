package cmd

import (
	"fmt"
	"strings"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/spf13/cobra"
)

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Manage branches (wraps git branch)",
	Long: `Manage branches.

All arguments are forwarded verbatim to git branch.

After a rename (-m/-M) or delete (-d/-D), GCM updates the branch
assignment map in .git/gcm.json automatically.`,
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if checkHelp(args) {
			return cmd.Help()
		}

		if _, oldBranch, newBranch, ok := parseRenameArgs(args); ok {
			return runBranchRename(args, oldBranch, newBranch)
		}

		if deletedBranches, ok := parseDeleteArgs(args); ok {
			return runBranchDelete(args, deletedBranches)
		}

		return passthroughGit(globalGitFlags, append([]string{"branch"}, args...))
	},
}

func runBranchRename(args []string, oldBranch, newBranch string) error {
	repoInfo, err := git.GetRepoInfo()
	if err != nil {
		// Not in a gcm repo — pure git passthrough, no store to update.
		return passthroughGit(globalGitFlags, append([]string{"branch"}, args...))
	}

	// Capture current branch BEFORE the rename so implicit rename (-m newname)
	// records the right old name. After git renames, CurrentBranch returns newBranch.
	if oldBranch == "" {
		oldBranch, _ = git.CurrentBranch(repoInfo.GitDir)
	}

	if err := passthroughGit(globalGitFlags, append([]string{"branch"}, args...)); err != nil {
		return err
	}

	s, err := store.LoadOrCreate(repoInfo.GitDir)
	if err != nil {
		return nil // store not accessible — don't fail the rename
	}

	if oldBranch != "" {
		s.RenameBranch(oldBranch, newBranch)
		_ = store.Save(repoInfo.GitDir, s)
	}
	return nil
}

func runBranchDelete(args []string, deletedBranches []string) error {
	deleteFlag := "-d"
	for _, a := range args {
		if a == "-D" {
			deleteFlag = "-D"
			break
		}
	}

	repoInfo, err := git.GetRepoInfo()
	storeAvailable := err == nil

	var s *store.Store
	var gitDir string
	if storeAvailable {
		gitDir = repoInfo.GitDir
		s, err = store.LoadOrCreate(gitDir)
		if err != nil {
			storeAvailable = false
		}
	}

	// Delete branches one at a time so we can track partial success.
	// Use -- before the branch name so dash-named branches aren't treated as flags.
	var successfullyDeleted []string
	hadFailure := false
	for _, branch := range deletedBranches {
		if err := passthroughGit(globalGitFlags, []string{"branch", deleteFlag, "--", branch}); err != nil {
			hadFailure = true
		} else {
			successfullyDeleted = append(successfullyDeleted, branch)
		}
	}

	if storeAvailable && len(successfullyDeleted) > 0 {
		for _, branch := range successfullyDeleted {
			s.UnassignBranch(branch)
		}
		_ = store.Save(gitDir, s)
	}

	if hadFailure {
		return fmt.Errorf("one or more branch deletions failed")
	}
	return nil
}

// parseRenameArgs detects -m/-M/--move and returns (flag, oldBranch, newBranch, true).
// When only one positional arg follows the flag, oldBranch is "" (git uses current branch).
func parseRenameArgs(args []string) (flag, oldBranch, newBranch string, ok bool) {
	for i, arg := range args {
		if arg == "-m" || arg == "-M" || arg == "--move" {
			remaining := args[i+1:]
			switch len(remaining) {
			case 0:
				return arg, "", "", false // git will report the error
			case 1:
				return arg, "", remaining[0], true
			default:
				return arg, remaining[0], remaining[1], true
			}
		}
	}
	return "", "", "", false
}

// parseDeleteArgs extracts branch names after -d/-D/--delete.
// Stops collecting on any new flag.
func parseDeleteArgs(args []string) ([]string, bool) {
	var result []string
	inDelete := false
	for _, arg := range args {
		if arg == "-d" || arg == "-D" || arg == "--delete" {
			inDelete = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			inDelete = false
			continue
		}
		if inDelete {
			result = append(result, arg)
		}
	}
	return result, len(result) > 0
}

func init() {
	rootCmd.AddCommand(branchCmd)
}
