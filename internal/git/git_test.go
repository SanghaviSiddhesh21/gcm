package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// run executes a command in dir and fails the test immediately if it errors.
// t.Helper() makes failures point to the calling line, not inside this function.
func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command %v failed: %v\n%s", args, err, out)
	}
}

// setupTestRepo creates a real git repo in a temp directory with one empty
// commit on "main", so HEAD exists and branch operations work correctly.
// The directory is automatically deleted when the test finishes.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run(t, dir, "git", "init", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@gcm.test")
	run(t, dir, "git", "config", "user.name", "GCM Test")
	run(t, dir, "git", "commit", "--allow-empty", "-m", "initial commit")

	return dir
}

// createBranch creates a new branch in the given repo without switching to it.
func createBranch(t *testing.T, repoDir, branchName string) {
	t.Helper()
	run(t, repoDir, "git", "branch", branchName)
}

// ── TestGetRepoInfo ───────────────────────────────────────────────────────────

func TestGetRepoInfo(t *testing.T) {
	t.Run("inside-git-repo", func(t *testing.T) {
		repoDir := setupTestRepo(t)

		orig, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		defer os.Chdir(orig) //nolint:errcheck
		if err := os.Chdir(repoDir); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		info, err := GetRepoInfo()
		if err != nil {
			t.Fatalf("GetRepoInfo() error = %v", err)
		}
		if info.WorkDir == "" {
			t.Error("WorkDir is empty")
		}
		if info.GitDir == "" {
			t.Error("GitDir is empty")
		}
	})

	t.Run("outside-git-repo", func(t *testing.T) {
		dir := t.TempDir() // plain directory — not a git repo

		orig, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		defer os.Chdir(orig) //nolint:errcheck
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		_, err = GetRepoInfo()
		if err == nil {
			t.Error("GetRepoInfo() expected error outside git repo, got nil")
		}
	})
}

// ── TestListBranches ──────────────────────────────────────────────────────────

func TestListBranches(t *testing.T) {
	t.Run("single-branch", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		branches, err := ListBranches(gitDir)
		if err != nil {
			t.Fatalf("ListBranches() error = %v", err)
		}
		if len(branches) != 1 {
			t.Fatalf("len(branches) = %d, want 1", len(branches))
		}
		if branches[0] != "main" {
			t.Errorf("branches[0] = %q, want %q", branches[0], "main")
		}
	})

	t.Run("multiple-branches", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		createBranch(t, repoDir, "feature")
		createBranch(t, repoDir, "hotfix")

		branches, err := ListBranches(gitDir)
		if err != nil {
			t.Fatalf("ListBranches() error = %v", err)
		}

		sort.Strings(branches)
		want := []string{"feature", "hotfix", "main"}

		if len(branches) != len(want) {
			t.Fatalf("len(branches) = %d, want %d: got %v", len(branches), len(want), branches)
		}
		for i := range want {
			if branches[i] != want[i] {
				t.Errorf("branches[%d] = %q, want %q", i, branches[i], want[i])
			}
		}
	})

	t.Run("relative-gitdir", func(t *testing.T) {
		// Covers the gitDir == ".git" special case in ListBranches.
		// When gitDir is the literal string ".git", workDir is set to "."
		// so we must be chdir-ed into the repo first.
		repoDir := setupTestRepo(t)

		orig, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		defer os.Chdir(orig) //nolint:errcheck
		if err := os.Chdir(repoDir); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		branches, err := ListBranches(".git")
		if err != nil {
			t.Fatalf("ListBranches(.git) error = %v", err)
		}
		if len(branches) != 1 || branches[0] != "main" {
			t.Errorf("branches = %v, want [main]", branches)
		}
	})
}

// ── TestCurrentBranch ─────────────────────────────────────────────────────────

func TestCurrentBranch(t *testing.T) {
	t.Run("absolute-gitdir", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		branch, err := CurrentBranch(gitDir)
		if err != nil {
			t.Fatalf("CurrentBranch() error = %v", err)
		}
		if branch != "main" {
			t.Errorf("CurrentBranch() = %q, want %q", branch, "main")
		}
	})

	t.Run("relative-gitdir", func(t *testing.T) {
		// Covers the gitDir == ".git" special case in CurrentBranch.
		repoDir := setupTestRepo(t)

		orig, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		defer os.Chdir(orig) //nolint:errcheck
		if err := os.Chdir(repoDir); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		branch, err := CurrentBranch(".git")
		if err != nil {
			t.Fatalf("CurrentBranch(.git) error = %v", err)
		}
		if branch != "main" {
			t.Errorf("CurrentBranch(.git) = %q, want %q", branch, "main")
		}
	})
}

// ── TestBranchExists ──────────────────────────────────────────────────────────

func TestBranchExists(t *testing.T) {
	repoDir := setupTestRepo(t)
	gitDir := filepath.Join(repoDir, ".git")
	createBranch(t, repoDir, "feature")

	t.Run("existing-branch", func(t *testing.T) {
		exists, err := BranchExists(gitDir, "feature")
		if err != nil {
			t.Fatalf("BranchExists() error = %v", err)
		}
		if !exists {
			t.Error("BranchExists(feature) = false, want true")
		}
	})

	t.Run("non-existing-branch", func(t *testing.T) {
		exists, err := BranchExists(gitDir, "non-existent")
		if err != nil {
			t.Fatalf("BranchExists() error = %v", err)
		}
		if exists {
			t.Error("BranchExists(non-existent) = true, want false")
		}
	})
}
