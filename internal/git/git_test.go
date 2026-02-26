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

// setupTestRepoWithRemote creates a bare repo (remote) and a working repo with
// the bare repo configured as "origin". It pushes "main" so the remote tracking
// ref exists. Returns (workingDir, bareDir).
func setupTestRepoWithRemote(t *testing.T) (string, string) {
	t.Helper()
	bareDir := t.TempDir()
	run(t, bareDir, "git", "init", "--bare", "-b", "main")

	workDir := setupTestRepo(t)
	run(t, workDir, "git", "remote", "add", "origin", bareDir)
	run(t, workDir, "git", "push", "-u", "origin", "main")

	return workDir, bareDir
}

// ── TestGetRepoInfo ───────────────────────────────────────────────────────────

func TestGetRepoInfo(t *testing.T) {
	t.Run("SUCCESS_CASE: inside_git_repo", func(t *testing.T) {
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

	t.Run("ERROR_CASE: outside_git_repo", func(t *testing.T) {
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

// ── TestGetRepoInfoAfterChdir ─────────────────────────────────────────────────

func TestGetRepoInfoAfterChdir(t *testing.T) {
	repoA := setupTestRepo(t)
	repoB := setupTestRepo(t)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	// Start in repoA.
	if err := os.Chdir(repoA); err != nil {
		t.Fatalf("Chdir repoA: %v", err)
	}
	infoA, err := GetRepoInfo()
	if err != nil {
		t.Fatalf("GetRepoInfo() in repoA: %v", err)
	}

	// Chdir to repoB — GetRepoInfo must reflect repoB now.
	if err := os.Chdir(repoB); err != nil {
		t.Fatalf("Chdir repoB: %v", err)
	}
	infoB, err := GetRepoInfo()
	if err != nil {
		t.Fatalf("GetRepoInfo() in repoB: %v", err)
	}

	// WorkDir is returned by --show-toplevel which always returns an absolute path.
	// GitDir may return a relative ".git", so we only compare WorkDir.
	if infoA.WorkDir == infoB.WorkDir {
		t.Errorf("WorkDir unchanged after Chdir: both = %q (should differ per repo)", infoA.WorkDir)
	}
}

// ── TestListBranches ──────────────────────────────────────────────────────────

func TestListBranches(t *testing.T) {
	t.Run("SUCCESS_CASE: single_branch", func(t *testing.T) {
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

	t.Run("SUCCESS_CASE: multiple_branches", func(t *testing.T) {
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

	t.Run("SUCCESS_CASE: relative_gitdir", func(t *testing.T) {
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
	t.Run("SUCCESS_CASE: absolute_gitdir", func(t *testing.T) {
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

	t.Run("SUCCESS_CASE: relative_gitdir", func(t *testing.T) {
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

	t.Run("SUCCESS_CASE: existing_branch", func(t *testing.T) {
		exists, err := BranchExists(gitDir, "feature")
		if err != nil {
			t.Fatalf("BranchExists() error = %v", err)
		}
		if !exists {
			t.Error("BranchExists(feature) = false, want true")
		}
	})

	t.Run("SUCCESS_CASE: non_existing_branch", func(t *testing.T) {
		exists, err := BranchExists(gitDir, "non-existent")
		if err != nil {
			t.Fatalf("BranchExists() error = %v", err)
		}
		if exists {
			t.Error("BranchExists(non-existent) = true, want false")
		}
	})
}

// ── TestListRemoteBranches ────────────────────────────────────────────────────

func TestListRemoteBranches(t *testing.T) {
	t.Run("ERROR_CASE: no_remote_configured", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		branches, err := ListRemoteBranches(gitDir)
		if err != nil {
			t.Fatalf("ListRemoteBranches() error = %v", err)
		}
		if len(branches) != 0 {
			t.Errorf("expected empty slice, got %v", branches)
		}
	})

	t.Run("SUCCESS_CASE: pushed_branch_appears", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		branches, err := ListRemoteBranches(gitDir)
		if err != nil {
			t.Fatalf("ListRemoteBranches() error = %v", err)
		}

		found := false
		for _, b := range branches {
			if b == "main" {
				found = true
			}
		}
		if !found {
			t.Errorf("ListRemoteBranches() = %v, want to contain \"main\"", branches)
		}
	})

	t.Run("SUCCESS_CASE: unpushed_branch_absent", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		// Create a local branch but do NOT push it.
		createBranch(t, workDir, "local-only")

		branches, err := ListRemoteBranches(gitDir)
		if err != nil {
			t.Fatalf("ListRemoteBranches() error = %v", err)
		}
		for _, b := range branches {
			if b == "local-only" {
				t.Errorf("ListRemoteBranches() contains local-only branch, want absent")
			}
		}
	})

	t.Run("SUCCESS_CASE: relative_gitdir", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)

		orig, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		defer os.Chdir(orig) //nolint:errcheck
		if err := os.Chdir(workDir); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		branches, err := ListRemoteBranches(".git")
		if err != nil {
			t.Fatalf("ListRemoteBranches(.git) error = %v", err)
		}
		found := false
		for _, b := range branches {
			if b == "main" {
				found = true
			}
		}
		if !found {
			t.Errorf("ListRemoteBranches(.git) = %v, want to contain main", branches)
		}
	})
}

// ── TestSyncStatus ────────────────────────────────────────────────────────────

func TestSyncStatus(t *testing.T) {
	t.Run("SUCCESS_CASE: in_sync", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		ahead, behind, err := SyncStatus(gitDir, "main")
		if err != nil {
			t.Fatalf("SyncStatus() error = %v", err)
		}
		if ahead != 0 || behind != 0 {
			t.Errorf("SyncStatus() = (%d, %d), want (0, 0)", ahead, behind)
		}
	})

	t.Run("SUCCESS_CASE: ahead", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		// Make 2 local commits that have NOT been pushed.
		run(t, workDir, "git", "commit", "--allow-empty", "-m", "local commit 1")
		run(t, workDir, "git", "commit", "--allow-empty", "-m", "local commit 2")

		ahead, behind, err := SyncStatus(gitDir, "main")
		if err != nil {
			t.Fatalf("SyncStatus() error = %v", err)
		}
		if ahead != 2 || behind != 0 {
			t.Errorf("SyncStatus() = (%d, %d), want (2, 0)", ahead, behind)
		}
	})

	t.Run("SUCCESS_CASE: behind", func(t *testing.T) {
		workDir, bareDir := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		// Clone a second working copy to push a commit without involving workDir.
		workDir2 := t.TempDir()
		run(t, workDir2, "git", "clone", bareDir, ".")
		run(t, workDir2, "git", "config", "user.email", "test@gcm.test")
		run(t, workDir2, "git", "config", "user.name", "GCM Test")
		run(t, workDir2, "git", "commit", "--allow-empty", "-m", "remote commit")
		run(t, workDir2, "git", "push", "origin", "main")

		// Now fetch in workDir so tracking ref updates.
		run(t, workDir, "git", "fetch", "origin")

		ahead, behind, err := SyncStatus(gitDir, "main")
		if err != nil {
			t.Fatalf("SyncStatus() error = %v", err)
		}
		if ahead != 0 || behind != 1 {
			t.Errorf("SyncStatus() = (%d, %d), want (0, 1)", ahead, behind)
		}
	})

	t.Run("ERROR_CASE: no_remote_ref", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		// "main" exists locally but no remote configured → SyncStatus should error.
		_, _, err := SyncStatus(gitDir, "main")
		if err == nil {
			t.Error("SyncStatus() expected error when remote ref absent, got nil")
		}
	})

	t.Run("SUCCESS_CASE: multiple_commits_ahead_behind", func(t *testing.T) {
		workDir, bareDir := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		// Create a second clone and push multiple commits
		workDir2 := t.TempDir()
		run(t, workDir2, "git", "clone", bareDir, ".")
		run(t, workDir2, "git", "config", "user.email", "test@gcm.test")
		run(t, workDir2, "git", "config", "user.name", "GCM Test")
		run(t, workDir2, "git", "commit", "--allow-empty", "-m", "remote 1")
		run(t, workDir2, "git", "commit", "--allow-empty", "-m", "remote 2")
		run(t, workDir2, "git", "commit", "--allow-empty", "-m", "remote 3")
		run(t, workDir2, "git", "push", "origin", "main")

		// Make multiple local commits
		run(t, workDir, "git", "commit", "--allow-empty", "-m", "local 1")
		run(t, workDir, "git", "commit", "--allow-empty", "-m", "local 2")

		// Fetch so tracking ref is current
		run(t, workDir, "git", "fetch", "origin")

		ahead, behind, err := SyncStatus(gitDir, "main")
		if err != nil {
			t.Fatalf("SyncStatus() error = %v", err)
		}
		if ahead != 2 || behind != 3 {
			t.Errorf("SyncStatus() = (%d, %d), want (2, 3)", ahead, behind)
		}
	})
}

// ── TestBranchCommitTimes ─────────────────────────────────────────────────────

func TestBranchCommitTimes(t *testing.T) {
	t.Run("SUCCESS_CASE: local_only", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		// Create a local branch but do not push it
		createBranch(t, repoDir, "local-branch")

		branchTimes, err := BranchCommitTimes(gitDir)
		if err != nil {
			t.Fatalf("BranchCommitTimes() error = %v", err)
		}

		mainTime := branchTimes["main"]
		if mainTime.IsZero() {
			t.Error("BranchCommitTimes() main time is zero")
		}

		localTime := branchTimes["local-branch"]
		if localTime.IsZero() {
			t.Error("BranchCommitTimes() local-branch time is zero")
		}

		// Both should have similar times since they're on the same commit
		if mainTime != localTime {
			t.Errorf("BranchCommitTimes() main and local-branch times differ: %v vs %v", mainTime, localTime)
		}
	})

	t.Run("SUCCESS_CASE: remote_newer", func(t *testing.T) {
		workDir, bareDir := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		// Create a second clone and push a commit
		workDir2 := t.TempDir()
		run(t, workDir2, "git", "clone", bareDir, ".")
		run(t, workDir2, "git", "config", "user.email", "test@gcm.test")
		run(t, workDir2, "git", "config", "user.name", "GCM Test")

		run(t, workDir2, "git", "commit", "--allow-empty", "-m", "remote commit")
		run(t, workDir2, "git", "push", "origin", "main")

		// Fetch in workDir so tracking ref updates
		run(t, workDir, "git", "fetch", "origin")

		branchTimes, err := BranchCommitTimes(gitDir)
		if err != nil {
			t.Fatalf("BranchCommitTimes() error = %v", err)
		}

		mainTime := branchTimes["main"]
		if mainTime.IsZero() {
			t.Error("BranchCommitTimes() main time is zero")
		}
	})

	t.Run("SUCCESS_CASE: local_newer", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		// Make a local commit after push
		run(t, workDir, "git", "commit", "--allow-empty", "-m", "local only commit")

		branchTimes, err := BranchCommitTimes(gitDir)
		if err != nil {
			t.Fatalf("BranchCommitTimes() error = %v", err)
		}

		mainTime := branchTimes["main"]
		if mainTime.IsZero() {
			t.Error("BranchCommitTimes() main time is zero")
		}
	})

	t.Run("SUCCESS_CASE: in_sync", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		branchTimes, err := BranchCommitTimes(gitDir)
		if err != nil {
			t.Fatalf("BranchCommitTimes() error = %v", err)
		}

		mainTime := branchTimes["main"]
		if mainTime.IsZero() {
			t.Error("BranchCommitTimes() main time is zero")
		}
	})

	t.Run("SUCCESS_CASE: no_remote_configured", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		branchTimes, err := BranchCommitTimes(gitDir)
		if err != nil {
			t.Fatalf("BranchCommitTimes() error = %v", err)
		}

		if branchTimes == nil {
			t.Error("BranchCommitTimes() returned nil map")
		}

		mainTime := branchTimes["main"]
		if mainTime.IsZero() {
			t.Error("BranchCommitTimes() main time is zero")
		}
	})

	t.Run("SUCCESS_CASE: strips_origin_prefix", func(t *testing.T) {
		workDir, _ := setupTestRepoWithRemote(t)
		gitDir := filepath.Join(workDir, ".git")

		branchTimes, err := BranchCommitTimes(gitDir)
		if err != nil {
			t.Fatalf("BranchCommitTimes() error = %v", err)
		}

		// Verify no keys contain "origin/" prefix
		for key := range branchTimes {
			if len(key) >= len("origin/") && key[:len("origin/")] == "origin/" {
				t.Errorf("BranchCommitTimes() result contains key with origin/ prefix: %q", key)
			}
		}
	})
}

// ── TestWorktreeStatus ─────────────────────────────────────────────────────────

func TestWorktreeStatus(t *testing.T) {
	t.Run("SUCCESS_CASE: clean_worktree", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		status, err := GetWorktreeStatus(gitDir)
		if err != nil {
			t.Fatalf("WorktreeStatus() error = %v", err)
		}
		if status.IsDirty() {
			t.Errorf("WorktreeStatus() on clean repo: IsDirty() = true, want false")
		}
	})

	t.Run("SUCCESS_CASE: untracked_file", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		// Create an untracked file
		if err := os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("hello"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		status, err := GetWorktreeStatus(gitDir)
		if err != nil {
			t.Fatalf("WorktreeStatus() error = %v", err)
		}
		if !status.IsDirty() {
			t.Error("WorktreeStatus() IsDirty() = false, want true")
		}
		if len(status.Untracked) != 1 || status.Untracked[0] != "untracked.txt" {
			t.Errorf("Untracked = %v, want [untracked.txt]", status.Untracked)
		}
		if len(status.Staged) != 0 {
			t.Errorf("Staged = %v, want []", status.Staged)
		}
		if len(status.Unstaged) != 0 {
			t.Errorf("Unstaged = %v, want []", status.Unstaged)
		}
	})

	t.Run("SUCCESS_CASE: staged_file", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		// Create and stage a new file
		if err := os.WriteFile(filepath.Join(repoDir, "staged.txt"), []byte("hello"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		run(t, repoDir, "git", "add", "staged.txt")

		status, err := GetWorktreeStatus(gitDir)
		if err != nil {
			t.Fatalf("WorktreeStatus() error = %v", err)
		}
		if !status.IsDirty() {
			t.Error("WorktreeStatus() IsDirty() = false, want true")
		}
		if len(status.Staged) != 1 || status.Staged[0] != "staged.txt" {
			t.Errorf("Staged = %v, want [staged.txt]", status.Staged)
		}
		if len(status.Unstaged) != 0 {
			t.Errorf("Unstaged = %v, want []", status.Unstaged)
		}
		if len(status.Untracked) != 0 {
			t.Errorf("Untracked = %v, want []", status.Untracked)
		}
	})

	t.Run("SUCCESS_CASE: unstaged_modification", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		// Create and commit a file
		if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("v1"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		run(t, repoDir, "git", "add", "tracked.txt")
		run(t, repoDir, "git", "commit", "-m", "add tracked.txt")

		// Modify without staging
		if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("v2"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		status, err := GetWorktreeStatus(gitDir)
		if err != nil {
			t.Fatalf("WorktreeStatus() error = %v", err)
		}
		if !status.IsDirty() {
			t.Error("WorktreeStatus() IsDirty() = false, want true")
		}
		if len(status.Unstaged) != 1 || status.Unstaged[0] != "tracked.txt" {
			t.Errorf("Unstaged = %v, want [tracked.txt]", status.Unstaged)
		}
		if len(status.Staged) != 0 {
			t.Errorf("Staged = %v, want []", status.Staged)
		}
	})
}

// ── TestCheckout ──────────────────────────────────────────────────────────────

func TestCheckout(t *testing.T) {
	t.Run("SUCCESS_CASE: switch_to_existing_branch", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		createBranch(t, repoDir, "feature")

		if err := Checkout(gitDir, "feature"); err != nil {
			t.Fatalf("Checkout() error = %v", err)
		}

		current, err := CurrentBranch(gitDir)
		if err != nil {
			t.Fatalf("CurrentBranch() error = %v", err)
		}
		if current != "feature" {
			t.Errorf("CurrentBranch() = %q, want %q", current, "feature")
		}
	})

	t.Run("SUCCESS_CASE: switch_back_to_main", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		createBranch(t, repoDir, "feature")
		run(t, repoDir, "git", "checkout", "feature")

		if err := Checkout(gitDir, "main"); err != nil {
			t.Fatalf("Checkout() error = %v", err)
		}

		current, err := CurrentBranch(gitDir)
		if err != nil {
			t.Fatalf("CurrentBranch() error = %v", err)
		}
		if current != "main" {
			t.Errorf("CurrentBranch() = %q, want %q", current, "main")
		}
	})

	t.Run("ERROR_CASE: nonexistent_branch", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")

		err := Checkout(gitDir, "does-not-exist")
		if err == nil {
			t.Error("Checkout() expected error for nonexistent branch, got nil")
		}
	})

	t.Run("SUCCESS_CASE: checkout_with_clean_worktree", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		gitDir := filepath.Join(repoDir, ".git")
		createBranch(t, repoDir, "dev")

		if err := Checkout(gitDir, "dev"); err != nil {
			t.Fatalf("Checkout() error = %v", err)
		}

		// Do it again to same branch — git does not error on this
		if err := Checkout(gitDir, "dev"); err != nil {
			t.Fatalf("Checkout() second call error = %v", err)
		}
	})
}
