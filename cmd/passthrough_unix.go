//go:build !windows

package cmd

import (
	"os"
	"os/exec"

	"github.com/mattn/go-isatty"
)

// passthroughGit forwards args to git with full TTY and env filtering.
// globalFlags are prepended before args (e.g. --git-dir=, -c key=val).
//
// git runs in the same process group as gcm (no Setpgid). This is required
// so that git's pager (less) and editor children remain in the terminal's
// foreground process group and can read from the TTY. Ctrl+C is delivered by
// the kernel to the entire foreground process group, so git and all its
// children receive the signal without any manual forwarding.
func passthroughGit(globalFlags []string, args []string) error {
	if err := checkPassthroughArgs(args); err != nil {
		return err
	}

	// Handle git help in non-TTY: force plain text output to avoid man pager hangs.
	if len(args) > 0 && args[0] == "help" {
		return passthroughGitHelp(globalFlags, args)
	}

	env := buildPassthroughEnv()
	allArgs := append(globalFlags, args...) //nolint:gocritic
	c := exec.Command("git", allArgs...)    //nolint:gosec
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = env
	return c.Run()
}

// passthroughGitHelp runs git help with --no-man in non-TTY contexts to avoid pager hangs.
func passthroughGitHelp(globalFlags []string, args []string) error {
	env := buildPassthroughEnv()
	finalArgs := args
	// In non-TTY, inject --no-man so git help prints to stdout without opening a pager.
	if !isatty.IsTerminal(os.Stdout.Fd()) && len(args) > 0 {
		helpArgs := make([]string, 0, len(args)+1)
		helpArgs = append(helpArgs, "help", "--no-man")
		if len(args) > 1 {
			helpArgs = append(helpArgs, args[1:]...)
		}
		finalArgs = helpArgs
	}
	allArgs := append(globalFlags, finalArgs...) //nolint:gocritic
	c := exec.Command("git", allArgs...)         //nolint:gosec
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = env
	return c.Run()
}
