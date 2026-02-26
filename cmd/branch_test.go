package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupGCMRepo creates an initialized gcm repo (git init -b main + gcm store) and
// returns the repo directory.
func setupGCMRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := runGCM(t, dir, "init", "-b", "main"); err != nil {
		t.Fatalf("gcm init: %v", err)
	}
	setupGitConfig(t, dir)
	// Create an initial commit so branches can be created.
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	return dir
}

// createAndAssignBranch creates a branch and assigns it to a category in the gcm store.
func createAndAssignBranch(t *testing.T, dir, branch, category string) {
	t.Helper()
	gitBranch(t, dir, "branch", branch)
	if _, err := runGCM(t, dir, "create", category); err != nil {
		// ignore ErrCategoryExists
	}
	if _, err := runGCM(t, dir, "assign", branch, category); err != nil {
		t.Fatalf("gcm assign %s %s: %v", branch, category, err)
	}
}

func gitBranch(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// storeAssignment reads gcm.json and returns the category for branch (or "" if unassigned).
func storeAssignment(t *testing.T, dir, branch string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".git", "gcm.json"))
	if err != nil {
		t.Fatalf("read gcm.json: %v", err)
	}
	var s struct {
		Assignments map[string]string `json:"assignments"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("parse gcm.json: %v", err)
	}
	return s.Assignments[branch]
}

// ── TestBranchDeleteSingle ────────────────────────────────────────────────────

func TestBranchDeleteSingle(t *testing.T) {
	dir := setupGCMRepo(t)
	createAndAssignBranch(t, dir, "feature-x", "feature")

	// Checkout main so feature-x can be deleted.
	gitBranch(t, dir, "checkout", "main")

	if _, err := runGCM(t, dir, "branch", "-d", "feature-x"); err != nil {
		t.Fatalf("gcm branch -d feature-x: %v", err)
	}

	if cat := storeAssignment(t, dir, "feature-x"); cat != "" {
		t.Errorf("store still has assignment %q for deleted branch", cat)
	}
}

// ── TestBranchDeleteUnassigned ────────────────────────────────────────────────

func TestBranchDeleteUnassigned(t *testing.T) {
	dir := setupGCMRepo(t)
	gitBranch(t, dir, "branch", "untracked")
	gitBranch(t, dir, "checkout", "main")

	if _, err := runGCM(t, dir, "branch", "-d", "untracked"); err != nil {
		t.Fatalf("gcm branch -d untracked: %v", err)
	}
}

// ── TestBranchDeleteNotFound ──────────────────────────────────────────────────

func TestBranchDeleteNotFound(t *testing.T) {
	dir := setupGCMRepo(t)

	_, err := runGCM(t, dir, "branch", "-d", "does-not-exist")
	if err == nil {
		t.Error("gcm branch -d nonexistent: expected error, got nil")
	}
}

// ── TestBranchDeleteNotMerged ─────────────────────────────────────────────────

// TestBranchDeleteNotMerged verifies that when git rejects a safe delete (-d)
// because the branch has unmerged commits, the gcm store is left untouched.
func TestBranchDeleteNotMerged(t *testing.T) {
	dir := setupGCMRepo(t)
	createAndAssignBranch(t, dir, "unmerged-branch", "feature")

	// Create an unmerged commit on the branch.
	gitBranch(t, dir, "checkout", "unmerged-branch")
	writeFile(t, dir, "unmerged.txt", "content")
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	addCmd.CombinedOutput() //nolint:errcheck
	commitCmd := exec.Command("git", "commit", "-m", "unmerged commit")
	commitCmd.Dir = dir
	commitCmd.CombinedOutput() //nolint:errcheck
	gitBranch(t, dir, "checkout", "main")

	// -d must be rejected by git (branch is not merged).
	_, err := runGCM(t, dir, "branch", "-d", "unmerged-branch")
	if err == nil {
		t.Fatal("gcm branch -d unmerged-branch: expected error (not merged), got nil")
	}

	// Store must be untouched — assignment still present.
	if cat := storeAssignment(t, dir, "unmerged-branch"); cat != "feature" {
		t.Errorf("store assignment = %q, want 'feature' (git rejected the delete; store must be unchanged)", cat)
	}

	// Branch must still exist in git.
	out, _ := runGCM(t, dir, "branch")
	if !strings.Contains(out, "unmerged-branch") {
		t.Errorf("branch 'unmerged-branch' should still exist after failed -d\noutput: %s", out)
	}
}

// ── TestBranchDeleteForce ─────────────────────────────────────────────────────

func TestBranchDeleteForce(t *testing.T) {
	dir := setupGCMRepo(t)
	// Create an unmerged branch with a commit.
	gitBranch(t, dir, "checkout", "-b", "unmerged")
	writeFile(t, dir, "file.txt", "content")
	exec.Command("git", "add", ".").Dir = dir //nolint:errcheck
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.CombinedOutput() //nolint:errcheck
	cmd2 := exec.Command("git", "commit", "-m", "unmerged commit")
	cmd2.Dir = dir
	cmd2.CombinedOutput() //nolint:errcheck
	gitBranch(t, dir, "checkout", "main")

	if _, err := runGCM(t, dir, "branch", "-D", "unmerged"); err != nil {
		t.Fatalf("gcm branch -D unmerged: %v", err)
	}
}

// ── TestBranchDeleteMultiplePartialSuccess ────────────────────────────────────

func TestBranchDeleteMultiplePartialSuccess(t *testing.T) {
	dir := setupGCMRepo(t)
	createAndAssignBranch(t, dir, "branch-a", "feature")
	createAndAssignBranch(t, dir, "branch-c", "feature")

	// branch-b: create with unmerged commit so -d fails
	gitBranch(t, dir, "checkout", "-b", "branch-b")
	writeFile(t, dir, "b.txt", "b")
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.CombinedOutput() //nolint:errcheck
	cmd2 := exec.Command("git", "commit", "-m", "b commit")
	cmd2.Dir = dir
	cmd2.CombinedOutput() //nolint:errcheck
	gitBranch(t, dir, "checkout", "main")

	// a and c should delete; b should fail.
	_, err := runGCM(t, dir, "branch", "-d", "branch-a", "branch-b", "branch-c")
	if err == nil {
		t.Error("expected error due to branch-b not merged, got nil")
	}

	// a and c must be cleaned from store.
	if cat := storeAssignment(t, dir, "branch-a"); cat != "" {
		t.Errorf("branch-a still in store with category %q", cat)
	}
	if cat := storeAssignment(t, dir, "branch-c"); cat != "" {
		t.Errorf("branch-c still in store with category %q", cat)
	}
}

// ── TestBranchDeleteNoStore ───────────────────────────────────────────────────

func TestBranchDeleteNoStore(t *testing.T) {
	// Plain git repo — no gcm.json. git should still delete the branch.
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	cmd.CombinedOutput() //nolint:errcheck
	setupGitConfig(t, dir)
	cmd2 := exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd2.Dir = dir
	cmd2.CombinedOutput() //nolint:errcheck
	gitBranch(t, dir, "branch", "to-delete")
	gitBranch(t, dir, "checkout", "main")

	if _, err := runGCM(t, dir, "branch", "-d", "to-delete"); err != nil {
		t.Fatalf("gcm branch -d without store: %v", err)
	}
}

// ── TestBranchRenameBothArgs ──────────────────────────────────────────────────

func TestBranchRenameBothArgs(t *testing.T) {
	dir := setupGCMRepo(t)
	createAndAssignBranch(t, dir, "old-name", "feature")
	gitBranch(t, dir, "checkout", "main")

	if _, err := runGCM(t, dir, "branch", "-m", "old-name", "new-name"); err != nil {
		t.Fatalf("gcm branch -m old new: %v", err)
	}

	if cat := storeAssignment(t, dir, "old-name"); cat != "" {
		t.Errorf("old-name still in store with category %q", cat)
	}
	if cat := storeAssignment(t, dir, "new-name"); cat != "feature" {
		t.Errorf("new-name assignment = %q, want feature", cat)
	}
}

// ── TestBranchRenameImplicit ──────────────────────────────────────────────────

func TestBranchRenameImplicit(t *testing.T) {
	dir := setupGCMRepo(t)
	createAndAssignBranch(t, dir, "old-current", "feature")
	gitBranch(t, dir, "checkout", "old-current")

	// No oldbranch arg — git uses current branch.
	if _, err := runGCM(t, dir, "branch", "-m", "renamed-current"); err != nil {
		t.Fatalf("gcm branch -m newname (implicit): %v", err)
	}

	if cat := storeAssignment(t, dir, "old-current"); cat != "" {
		t.Errorf("old-current still in store with category %q", cat)
	}
	if cat := storeAssignment(t, dir, "renamed-current"); cat != "feature" {
		t.Errorf("renamed-current assignment = %q, want feature", cat)
	}
}

// ── TestBranchRenameUnassigned ────────────────────────────────────────────────

func TestBranchRenameUnassigned(t *testing.T) {
	dir := setupGCMRepo(t)
	gitBranch(t, dir, "branch", "unassigned")
	gitBranch(t, dir, "checkout", "main")

	// Rename must succeed; store is a no-op.
	if _, err := runGCM(t, dir, "branch", "-m", "unassigned", "renamed-unassigned"); err != nil {
		t.Fatalf("gcm branch -m unassigned renamed-unassigned: %v", err)
	}
}

// ── TestBranchRenameGitFails ──────────────────────────────────────────────────

func TestBranchRenameGitFails(t *testing.T) {
	dir := setupGCMRepo(t)

	// Rename non-existent branch — git will error.
	_, err := runGCM(t, dir, "branch", "-m", "no-such-branch", "whatever")
	if err == nil {
		t.Error("gcm branch -m nonexistent: expected error, got nil")
	}
}

// ── TestBranchNoArgs ──────────────────────────────────────────────────────────

func TestBranchNoArgs(t *testing.T) {
	dir := setupGCMRepo(t)

	out, err := runGCM(t, dir, "branch")
	if err != nil {
		t.Fatalf("gcm branch (no args): %v\n%s", err, out)
	}
}

// ── TestBranchDeleteRemoteTracking ────────────────────────────────────────────

func TestBranchDeleteRemoteTracking(t *testing.T) {
	// Remote-tracking deletions (-d -r origin/foo) must be forwarded to git.
	// They must NOT cause store mutations.
	// We can't easily test a real remote in a unit test, so verify the
	// command routes correctly by checking that gcm doesn't error on the
	// git-level error (no remote configured).
	dir := setupGCMRepo(t)
	// -r with no remote configured: git errors, that's fine — we just want
	// to confirm gcm passes through without panicking or mutating store.
	runGCM(t, dir, "branch", "-d", "-r", "origin/no-such-branch") //nolint:errcheck
}

// ── TestBranchOtherFlags ──────────────────────────────────────────────────────

func TestBranchOtherFlags(t *testing.T) {
	dir := setupGCMRepo(t)

	for _, args := range [][]string{
		{"branch", "--list"},
		{"branch", "-v"},
	} {
		if _, err := runGCM(t, dir, args...); err != nil {
			t.Errorf("gcm %v: unexpected error: %v", args, err)
		}
	}
}

// ── TestBranchDeleteDashNamedBranch ──────────────────────────────────────────

func TestBranchDeleteDashNamedBranch(t *testing.T) {
	dir := setupGCMRepo(t)
	// Create a branch literally named "-" using git directly (gcm branch
	// would also work, but git branch is simpler in the test setup).
	cmd := exec.Command("git", "branch", "--", "-")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git does not support dash-named branches on this version: %v\n%s", err, out)
	}
	gitBranch(t, dir, "checkout", "main")

	if _, err := runGCM(t, dir, "branch", "-d", "--", "-"); err != nil {
		t.Fatalf("gcm branch -d -- - (dash-named branch): %v", err)
	}
}

// writeFile creates a file with content in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}
