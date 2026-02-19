package git

import (
	"os/exec"
	"strings"
)

type RepoInfo struct {
	WorkDir string
	GitDir  string
}

func GetRepoInfo() (*RepoInfo, error) {
	workDir, err := runGit(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}

	gitDir, err := runGit(workDir, "rev-parse", "--git-dir")
	if err != nil {
		return nil, err
	}

	return &RepoInfo{
		WorkDir: workDir,
		GitDir:  gitDir,
	}, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
