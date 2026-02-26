//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

func passthroughGit(globalFlags []string, args []string) error {
	if err := checkPassthroughArgs(args); err != nil {
		return err
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
