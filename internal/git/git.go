package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

// BranchCommitTimes returns a map of local branch name → most recent commit time.
// For each branch, considers both local (refs/heads/<b>) and remote (refs/remotes/origin/<b>),
// returning whichever timestamp is more recent.
// If a branch has no remote ref (local-only or [Remote] ?), only local time is used.
// Uses a single git for-each-ref call for efficiency.
func BranchCommitTimes(gitDir string) (map[string]time.Time, error) {
	wd := workDir(gitDir)
	output, err := runGit(wd, "for-each-ref",
		"--format=%(refname) %(committerdate:unix)",
		"refs/heads", "refs/remotes/origin")
	if err != nil {
		return nil, fmt.Errorf("for-each-ref failed: %w", err)
	}

	localTimes := make(map[string]time.Time)
	remoteTimes := make(map[string]time.Time)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		ref, tsStr := parts[0], parts[1]
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}
		t := time.Unix(ts, 0)
		if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			localTimes[name] = t
		} else if name, ok := strings.CutPrefix(ref, "refs/remotes/origin/"); ok {
			if name != "HEAD" {
				remoteTimes[name] = t
			}
		}
	}

	result := make(map[string]time.Time)
	for name, lt := range localTimes {
		rt := remoteTimes[name]
		if rt.After(lt) {
			result[name] = rt
		} else {
			result[name] = lt
		}
	}
	return result, nil
}

type WorktreeStatus struct {
	Staged    []string
	Unstaged  []string
	Untracked []string
}

func (s WorktreeStatus) IsDirty() bool {
	return len(s.Staged) > 0 || len(s.Unstaged) > 0 || len(s.Untracked) > 0
}

// GetWorktreeStatus returns the dirty state of the working tree at gitDir.
// It runs `git status --porcelain=v1` and parses each line.
func GetWorktreeStatus(gitDir string) (WorktreeStatus, error) {
	wd := workDir(gitDir)
	cmd := exec.Command("git", "status", "--porcelain=v1")
	cmd.Dir = wd
	output, err := cmd.CombinedOutput()
	if err != nil {
		return WorktreeStatus{}, fmt.Errorf("git status failed: %w", err)
	}
	var result WorktreeStatus
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimRight(line, "\r\n") // Only trim line endings, not leading spaces
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		name := strings.TrimSpace(line[3:])
		switch {
		case x == '?' && y == '?':
			result.Untracked = append(result.Untracked, name)
		default:
			if x != ' ' && x != '?' {
				result.Staged = append(result.Staged, name)
			}
			if y != ' ' && y != '?' {
				result.Unstaged = append(result.Unstaged, name)
			}
		}
	}
	return result, nil
}

// Checkout switches the working tree to the given branch.
func Checkout(gitDir, branch string) error {
	_, err := runGit(workDir(gitDir), "checkout", branch)
	if err != nil {
		return fmt.Errorf("git checkout %s: %w", branch, err)
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
