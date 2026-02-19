package cmd

import (
	"fmt"
	"os"

	"github.com/siddhesh/gcm/internal/git"
	"github.com/siddhesh/gcm/internal/store"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize GCM in the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoInfo, err := git.GetRepoInfo()
		if err != nil {
			return err
		}

		storePath := store.StorePath(repoInfo.GitDir)
		if _, err := os.Stat(storePath); err == nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", store.ErrAlreadyInitialized.Error())
			return store.ErrAlreadyInitialized
		}

		s := store.NewStore()

		if err := store.Save(repoInfo.GitDir, s); err != nil {
			return err
		}

		fmt.Printf("Initialized GCM in %s\n", repoInfo.WorkDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
