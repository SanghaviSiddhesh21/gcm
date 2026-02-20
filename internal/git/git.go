package git

import (
	"os/exec"
	"path/filepath"
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

func ListBranches(gitDir string) ([]string, error) {
	workDir := filepath.Dir(gitDir)
	if gitDir == ".git" {
		workDir = "."
	}

	output, err := runGit(workDir, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

func CurrentBranch(gitDir string) (string, error) {
	workDir := filepath.Dir(gitDir)
	if gitDir == ".git" {
		workDir = "."
	}

	branch, err := runGit(workDir, "branch", "--show-current")
	if err != nil {
		return "", err
	}

	return branch, nil
}

func BranchExists(gitDir, branch string) (bool, error) {
	branches, err := ListBranches(gitDir)
	if err != nil {
		return false, err
	}

	for _, b := range branches {
		if b == branch {
			return true, nil
		}
	}
	return false, nil
}

func SwitchBranch(workDir, branch string) error {
	cmd := exec.Command("git", "switch", branch)
	cmd.Dir = workDir
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("git", "checkout", branch)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
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
