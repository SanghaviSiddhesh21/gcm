package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupBareRepo creates a bare git repo with one commit, suitable as a clone source.
func setupBareRepo(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	bare := t.TempDir()

	// Create a working repo, commit, then clone as bare.
	for _, args := range [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@gcm.test"},
		{"git", "config", "user.name", "GCM Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}

	cloneCmd := exec.Command("git", "clone", "--bare", src, bare)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --bare: %v\n%s", err, out)
	}
	return bare
}

// ── TestCloneSuccess ──────────────────────────────────────────────────────────

func TestCloneSuccess(t *testing.T) {
	src := setupBareRepo(t)
	parent := t.TempDir()

	dest := filepath.Join(parent, "cloned")
	_, err := runGCM(t, parent, "clone", src, dest)
	if err != nil {
		t.Fatalf("gcm clone: %v", err)
	}

	gcmJSON := filepath.Join(dest, ".git", "gcm.json")
	if _, statErr := os.Stat(gcmJSON); statErr != nil {
		t.Errorf("gcm.json not created after clone: %v", statErr)
	}
}

// ── TestCloneWithExplicitDir ──────────────────────────────────────────────────

func TestCloneWithExplicitDir(t *testing.T) {
	src := setupBareRepo(t)
	parent := t.TempDir()
	dest := filepath.Join(parent, "mydir")

	_, err := runGCM(t, parent, "clone", src, dest)
	if err != nil {
		t.Fatalf("gcm clone <url> mydir: %v", err)
	}

	gcmJSON := filepath.Join(dest, ".git", "gcm.json")
	if _, statErr := os.Stat(gcmJSON); statErr != nil {
		t.Errorf("gcm.json not at mydir/.git/gcm.json: %v", statErr)
	}
}

// ── TestCloneURLInference ─────────────────────────────────────────────────────

func TestCloneURLInference(t *testing.T) {
	src := setupBareRepo(t)
	parent := t.TempDir()

	// No explicit dest — gcm should infer from URL basename.
	_, err := runGCM(t, parent, "clone", src)
	if err != nil {
		t.Fatalf("gcm clone (no dest): %v", err)
	}

	// Find the created directory — should be a child of parent.
	entries, _ := os.ReadDir(parent)
	if len(entries) == 0 {
		t.Fatal("no directory created after gcm clone with inferred name")
	}
	clonedDir := filepath.Join(parent, entries[0].Name())
	gcmJSON := filepath.Join(clonedDir, ".git", "gcm.json")
	if _, statErr := os.Stat(gcmJSON); statErr != nil {
		t.Errorf("gcm.json not created in inferred directory %q: %v", clonedDir, statErr)
	}
}

// ── TestCloneGitFails ─────────────────────────────────────────────────────────

func TestCloneGitFails(t *testing.T) {
	parent := t.TempDir()

	// Invalid URL — git clone will fail.
	_, err := runGCM(t, parent, "clone", "/nonexistent/repo/that/does/not/exist")
	if err == nil {
		t.Error("gcm clone invalid URL: expected error, got nil")
	}
}

// ── TestCloneSecurityRejectExtURL ─────────────────────────────────────────────

func TestCloneSecurityRejectExtURL(t *testing.T) {
	parent := t.TempDir()

	out, err := runGCM(t, parent, "clone", "ext::bash -c 'id'")
	if err == nil {
		t.Errorf("gcm clone ext:: URL: expected error, got nil (output: %s)", out)
	}
}
