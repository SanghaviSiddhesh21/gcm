package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:                "clone <url> [directory]",
	Short:              "Clone a repository and initialize GCM",
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if checkHelp(args) {
			return cmd.Help()
		}

		if len(args) == 0 {
			return fmt.Errorf("usage: gcm clone <url> [directory]")
		}

		if err := passthroughGit(globalGitFlags, append([]string{"clone"}, args...)); err != nil {
			return err
		}

		// Determine target directory and initialize the store.
		targetDir := cloneTargetDir(args)
		gitDir, err := git.GetGitDirAt(targetDir)
		if err != nil {
			return fmt.Errorf("gcm clone: could not locate git dir: %w", err)
		}

		if _, err := store.LoadOrCreate(gitDir); err != nil {
			return fmt.Errorf("gcm clone: failed to initialize store: %w", err)
		}

		return nil
	},
}

// cloneTargetDir infers the clone destination from git clone args.
// It skips known value-bearing flags, collects positional args (URL, optional dir),
// and derives the dir from the URL when no explicit dir is given.
func cloneTargetDir(args []string) string {
	// Known git clone flags that consume the following token as their value.
	knownCloneValueFlags := map[string]bool{
		"-b": true, "--branch": true,
		"-u": true, "--upload-pack": true,
		"--reference": true, "--reference-if-able": true,
		"--origin": true, "-o": true,
		"--depth":         true,
		"--shallow-since": true, "--shallow-exclude": true,
		"--jobs": true, "-j": true,
		"--separate-git-dir": true,
		"--config":           true, "-c": true,
		"--server-option": true,
		"--filter":        true,
		"--bundle-uri":    true,
	}

	nonFlags := make([]string, 0, 2)
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
			if knownCloneValueFlags[arg] {
				skipNext = true
			}
			continue
		}
		if len(arg) == 2 && arg[0] == '-' {
			if knownCloneValueFlags[arg] {
				skipNext = true
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		nonFlags = append(nonFlags, arg)
		if len(nonFlags) == 2 {
			break
		}
	}

	if len(nonFlags) >= 2 {
		return nonFlags[1]
	}
	if len(nonFlags) == 1 {
		return repoNameFromURL(nonFlags[0])
	}
	return "."
}

// repoNameFromURL derives the default clone directory name from a repository URL,
// matching git's own convention: strip trailing slashes, take the last path
// component, and remove any .git suffix.
func repoNameFromURL(url string) string {
	url = strings.TrimRight(url, "/")
	base := filepath.Base(url)
	return strings.TrimSuffix(base, ".git")
}

func init() {
	rootCmd.AddCommand(cloneCmd)
}
