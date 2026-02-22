package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
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

func workDir(gitDir string) string {
	if gitDir == ".git" {
		return "."
	}
	return filepath.Dir(gitDir)
}

func ListBranches(gitDir string) ([]string, error) {
	wd := workDir(gitDir)

	output, err := runGit(wd, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

func CurrentBranch(gitDir string) (string, error) {
	wd := workDir(gitDir)

	branch, err := runGit(wd, "branch", "--show-current")
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

func ListRemoteBranches(gitDir string) ([]string, error) {
	output, err := runGit(workDir(gitDir), "branch", "-r", "--format=%(refname:short)")
	if err != nil { //nolint:nilerr // No remote configured; not an error condition
		return []string{}, nil
	}

	if output == "" {
		return []string{}, nil
	}

	raw := strings.Split(output, "\n")
	result := make([]string, 0, len(raw))
	for _, ref := range raw {
		ref = strings.TrimSpace(ref)
		if strings.HasSuffix(ref, "/HEAD") {
			continue
		}
		if after, ok := strings.CutPrefix(ref, "origin/"); ok {
			result = append(result, after)
		}
	}
	return result, nil
}

func SyncStatus(gitDir, branch string) (ahead, behind int, err error) {
	wd := workDir(gitDir)
	remote := "origin/" + branch

	aheadStr, err := runGit(wd, "rev-list", "--count", remote+".."+branch)
	if err != nil { //nolint:nilerr // Remote ref doesn't exist; defensive error
		return 0, 0, fmt.Errorf("remote ref %s not found: %w", remote, err)
	}

	behindStr, err := runGit(wd, "rev-list", "--count", branch+".."+remote)
	if err != nil { //nolint:nilerr // Defensive error; git rev-list should not fail with valid refs
		return 0, 0, fmt.Errorf("rev-list behind failed for %s: %w", branch, err)
	}

	ahead, err = strconv.Atoi(aheadStr)
	if err != nil { //nolint:errcheck // Defensive: git rev-list always returns a valid number
		return 0, 0, fmt.Errorf("parsing ahead count %q: %w", aheadStr, err)
	}

	behind, err = strconv.Atoi(behindStr)
	if err != nil { //nolint:errcheck // Defensive: git rev-list always returns a valid number
		return 0, 0, fmt.Errorf("parsing behind count %q: %w", behindStr, err)
	}

	return ahead, behind, nil
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
