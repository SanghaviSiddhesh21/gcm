package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Get and set GCM and git configuration options",
	Long: `Get and set GCM and git configuration options.

GCM-specific keys are stored in ~/.gcm/config.json.
All other keys are passed directly to git config.

GCM keys:
  gcm config api-key <value>    Set your Groq API key
  gcm config api-key            Get your Groq API key
  gcm config --unset api-key    Remove your Groq API key

Git passthrough (examples):
  gcm config user.name "John"
  gcm config --global user.email "john@example.com"
  gcm config --list`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if checkHelp(args) {
			return cmd.Help()
		}

		if containsAPIKey(args) {
			return runGCMConfig(args)
		}

		return passthroughGitConfig(args)
	},
}

func containsAPIKey(args []string) bool {
	for _, arg := range args {
		if arg == "api-key" {
			return true
		}
	}
	return false
}

func runGCMConfig(args []string) error {
	for i, arg := range args {
		if arg == "--unset" {
			if i+1 < len(args) && args[i+1] == "api-key" {
				if err := config.UnsetAPIKey(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					return err
				}
				fmt.Println("API key removed.")
				return nil
			}
		}
	}

	for i, arg := range args {
		if arg == "api-key" {
			if i+1 < len(args) {
				if err := config.SetAPIKey(args[i+1]); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					return err
				}
				fmt.Println("API key saved.")
			} else {
				val, err := config.GetAPIKey()
				if errors.Is(err, config.ErrNotSet) {
					return fmt.Errorf("api-key is not set")
				}
				if err != nil {
					return err
				}
				fmt.Println(val)
			}
			return nil
		}
	}

	return nil
}

func passthroughGitConfig(args []string) error {
	return passthroughGit(globalGitFlags, append([]string{"config"}, args...))
}

func init() {
	rootCmd.AddCommand(configCmd)
}
