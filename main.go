package main

import (
	"os"

	"github.com/siddhesh/gcm/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
