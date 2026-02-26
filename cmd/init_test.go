package cmd_test

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gcmBin is built once in TestMain and shared across all cmd_test tests.
var gcmBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gcm-test-bin-*")
	if err != nil {
		log.Fatalf("MkdirTemp: %v", err)
	}
	gcmBin = filepath.Join(dir, "gcm")

	// Build from module root (parent of cmd/).
	cmd := exec.Command("go", "build", "-o", gcmBin, "..")
	cmd.Dir = "."
	if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
		log.Fatalf("build gcm: %v\n%s", buildErr, out)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// runGCM runs the gcm binary with the given args in dir.
func runGCM(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(gcmBin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// setupGitConfig sets minimal git config so commits work in temp repos.
func setupGitConfig(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"git", "config", "user.email", "test@gcm.test"},
		{"git", "config", "user.name", "GCM Test"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config: %v\n%s", err, out)
		}
	}
}

// ── TestInitNewRepo ───────────────────────────────────────────────────────────

func TestInitNewRepo(t *testing.T) {
	dir := t.TempDir()

	if _, err := runGCM(t, dir, "init"); err != nil {
		t.Fatalf("gcm init: %v", err)
	}

	gcmJSON := filepath.Join(dir, ".git", "gcm.json")
	if _, err := os.Stat(gcmJSON); err != nil {
		t.Errorf("gcm.json not created: %v", err)
	}
}

// ── TestInitExistingGitRepo ───────────────────────────────────────────────────

func TestInitExistingGitRepo(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	if _, err := runGCM(t, dir, "init"); err != nil {
		t.Fatalf("gcm init on existing git repo: %v", err)
	}

	gcmJSON := filepath.Join(dir, ".git", "gcm.json")
	if _, err := os.Stat(gcmJSON); err != nil {
		t.Errorf("gcm.json not created: %v", err)
	}
}

// ── TestInitAlreadyInitialized ────────────────────────────────────────────────

func TestInitAlreadyInitialized(t *testing.T) {
	dir := t.TempDir()

	if _, err := runGCM(t, dir, "init"); err != nil {
		t.Fatalf("first gcm init: %v", err)
	}
	if _, err := runGCM(t, dir, "init"); err != nil {
		t.Fatalf("second gcm init (idempotent): %v", err)
	}
}

// ── TestInitWithDirectory ─────────────────────────────────────────────────────

func TestInitWithDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "myrepo")

	if _, err := runGCM(t, parent, "init", target); err != nil {
		t.Fatalf("gcm init myrepo: %v", err)
	}

	gcmJSON := filepath.Join(target, ".git", "gcm.json")
	if _, err := os.Stat(gcmJSON); err != nil {
		t.Errorf("gcm.json not at myrepo/.git/gcm.json: %v", err)
	}
}

// ── TestInitGitFails ──────────────────────────────────────────────────────────

func TestInitGitFails(t *testing.T) {
	dir := t.TempDir()
	_, err := runGCM(t, dir, "init", "--invalid-flag-that-git-does-not-know")
	if err == nil {
		t.Error("gcm init with invalid flag: expected error, got nil")
	}
	gcmJSON := filepath.Join(dir, ".git", "gcm.json")
	if _, statErr := os.Stat(gcmJSON); statErr == nil {
		t.Error("gcm.json created even though git init failed")
	}
}
