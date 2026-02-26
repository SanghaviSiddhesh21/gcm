package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var gcmNativeCmds = map[string]bool{
	"view":       true,
	"create":     true,
	"assign":     true,
	"delete":     true,
	"categories": true,
}

// gcm+git commands: gcm help shown first, git help appended.
var gcmBothCmds = map[string]bool{
	"commit": true,
	"config": true,
	"init":   true,
	"branch": true,
	"clone":  true,
}

var helpCmd = &cobra.Command{
	Use:                "help [command]",
	Short:              "Show help for a gcm or git command",
	SilenceUsage:       true,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		if len(args) == 0 {
			return runHelpAll(root)
		}
		return runHelpTopic(root, args[0])
	},
}

// runHelpAll prints gcm's own help then git's full command listing.
// Accepts root explicitly to avoid an init-cycle with rootCmd.
// Used by both `gcm help` and `gcm --help`.
func runHelpAll(root *cobra.Command) error {
	if err := root.Help(); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(os.Stdout, "\nGit commands (via passthrough):")
	return passthroughGit(globalGitFlags, []string{"help", "--all"})
}

// runHelpTopic routes help for a single topic.
func runHelpTopic(root *cobra.Command, topic string) error {
	if gcmNativeCmds[topic] {
		for _, c := range root.Commands() {
			if c.Name() == topic {
				return c.Help()
			}
		}
		return fmt.Errorf("gcm: unknown command %q", topic)
	}

	if gcmBothCmds[topic] {
		for _, c := range root.Commands() {
			if c.Name() == topic {
				if err := c.Help(); err != nil {
					return err
				}
				break
			}
		}
		_, _ = fmt.Fprintf(os.Stdout, "\n--- git help %s ---\n\n", topic)
		return passthroughGit(globalGitFlags, []string{"help", topic})
	}

	return passthroughGit(globalGitFlags, []string{"help", topic})
}

func init() {
	rootCmd.AddCommand(helpCmd)
}
