package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ── TestParseGlobalGitFlags ───────────────────────────────────────────────────

func TestParseGlobalGitFlags_C(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck

	remaining, globalFlags, err := parseGlobalGitFlags([]string{"-C", dir, "status"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(globalFlags) != 0 {
		t.Errorf("globalFlags = %v, want empty (-C is consumed, not stored)", globalFlags)
	}
	if len(remaining) != 1 || remaining[0] != "status" {
		t.Errorf("remaining = %v, want [status]", remaining)
	}
	// CWD must have changed. Use EvalSymlinks to handle macOS /private/var → /var.
	cwd, _ := os.Getwd()
	cwdReal, _ := filepath.EvalSymlinks(cwd)
	dirReal, _ := filepath.EvalSymlinks(dir)
	if cwdReal != dirReal {
		t.Errorf("CWD = %q, want %q", cwdReal, dirReal)
	}
}

func TestParseGlobalGitFlags_c(t *testing.T) {
	remaining, globalFlags, err := parseGlobalGitFlags([]string{"-c", "user.name=Test", "commit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(globalFlags) != 2 || globalFlags[0] != "-c" || globalFlags[1] != "user.name=Test" {
		t.Errorf("globalFlags = %v, want [-c user.name=Test]", globalFlags)
	}
	if len(remaining) != 1 || remaining[0] != "commit" {
		t.Errorf("remaining = %v, want [commit]", remaining)
	}
}

func TestParseGlobalGitFlags_gitDir(t *testing.T) {
	dir := t.TempDir()
	arg := "--git-dir=" + dir
	remaining, globalFlags, err := parseGlobalGitFlags([]string{arg, "log"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(globalFlags) != 1 || globalFlags[0] != arg {
		t.Errorf("globalFlags = %v, want [%s]", globalFlags, arg)
	}
	if len(remaining) != 1 || remaining[0] != "log" {
		t.Errorf("remaining = %v, want [log]", remaining)
	}
}

func TestParseGlobalGitFlags_workTree(t *testing.T) {
	dir := t.TempDir()
	arg := "--work-tree=" + dir
	remaining, globalFlags, err := parseGlobalGitFlags([]string{arg, "status"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(globalFlags) != 1 || globalFlags[0] != arg {
		t.Errorf("globalFlags = %v, want [%s]", globalFlags, arg)
	}
	if len(remaining) != 1 || remaining[0] != "status" {
		t.Errorf("remaining = %v, want [status]", remaining)
	}
}

func TestParseGlobalGitFlags_mixed(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck

	args := []string{"-C", dir, "-c", "user.name=Test", "push", "origin"}
	remaining, globalFlags, err := parseGlobalGitFlags(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remaining) != 2 {
		t.Errorf("remaining = %v, want [push origin]", remaining)
	}
	// -c flag must be in globalFlags, -C must not be (it was consumed via Chdir).
	found := false
	for i, f := range globalFlags {
		if f == "-c" && i+1 < len(globalFlags) && globalFlags[i+1] == "user.name=Test" {
			found = true
		}
	}
	if !found {
		t.Errorf("globalFlags = %v, expected -c user.name=Test", globalFlags)
	}
}

func TestParseGlobalGitFlags_noFlags(t *testing.T) {
	remaining, globalFlags, err := parseGlobalGitFlags([]string{"status", "--short"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(globalFlags) != 0 {
		t.Errorf("globalFlags = %v, want empty", globalFlags)
	}
	if len(remaining) != 2 {
		t.Errorf("remaining = %v, want [status --short]", remaining)
	}
}

// ── TestExitCode ──────────────────────────────────────────────────────────────

func TestExitCode_nil(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Errorf("exitCode(nil) = %d, want 0", got)
	}
}

func TestExitCode_nonExecError(t *testing.T) {
	if got := exitCode(os.ErrPermission); got != 1 {
		t.Errorf("exitCode(non-exec error) = %d, want 1", got)
	}
}

// ── TestSecurityDenylist ──────────────────────────────────────────────────────

func TestParseGlobalGitFlags_deniedConfigKey(t *testing.T) {
	tests := []struct {
		name string
		kv   string
	}{
		{"fsmonitor", "core.fsmonitor=/tmp/evil"},
		{"fsmonitor-upper", "Core.Fsmonitor=/tmp/evil"},
		{"editor", "core.editor=/tmp/evil"},
		{"alias", "alias.foo=!evil"},
		{"filter-clean", "filter.lfs.clean=evil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseGlobalGitFlags([]string{"-c", tt.kv, "status"})
			if err == nil {
				t.Errorf("parseGlobalGitFlags(-c %s) expected error, got nil", tt.kv)
			}
		})
	}
}

func TestParseGlobalGitFlags_allowedConfigKey(t *testing.T) {
	tests := []struct {
		name string
		kv   string
	}{
		{"sshCommand", "core.sshCommand=ssh -i key"},
		{"credentialHelper", "credential.helper=osxkeychain"},
		{"userName", "user.name=Test"},
		{"filterRequired", "filter.lfs.required=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseGlobalGitFlags([]string{"-c", tt.kv})
			if err != nil {
				t.Errorf("parseGlobalGitFlags(-c %s) unexpected error: %v", tt.kv, err)
			}
		})
	}
}

func TestParseGlobalGitFlags_malformedC(t *testing.T) {
	_, _, err := parseGlobalGitFlags([]string{"-c", "nokeyequals"})
	if err == nil {
		t.Error("expected error for -c without '=', got nil")
	}
}

// ── TestExitCodeForwarding ────────────────────────────────────────────────────

func TestExitCodeForwarding(t *testing.T) {
	// git status in a non-repo directory exits 128 — a code that is NOT 0 or 1.
	// This verifies exitCode() forwards the real process exit code rather than
	// hardcoding 1 for all subprocess errors.
	dir := t.TempDir()
	cmd := exec.Command("git", "status")
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected git status to fail in non-repo dir")
	}
	got := exitCode(err)
	if got != 128 {
		t.Errorf("exitCode = %d, want 128 (git status in non-repo exits 128)", got)
	}
}

func TestParseGlobalGitFlags_gitDirSpaceForm(t *testing.T) {
	dir := t.TempDir()
	remaining, globalFlags, err := parseGlobalGitFlags([]string{"--git-dir", dir, "log"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(globalFlags) != 1 || globalFlags[0] != "--git-dir="+dir {
		t.Errorf("globalFlags = %v, want [--git-dir=%s]", globalFlags, dir)
	}
	if len(remaining) != 1 || remaining[0] != "log" {
		t.Errorf("remaining = %v, want [log]", remaining)
	}
}

func TestParseGlobalGitFlags_workTreeSpaceForm(t *testing.T) {
	dir := t.TempDir()
	remaining, globalFlags, err := parseGlobalGitFlags([]string{"--work-tree", dir, "status"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(globalFlags) != 1 || globalFlags[0] != "--work-tree="+dir {
		t.Errorf("globalFlags = %v, want [--work-tree=%s]", globalFlags, dir)
	}
	if len(remaining) != 1 || remaining[0] != "status" {
		t.Errorf("remaining = %v, want [status]", remaining)
	}
}

func TestParseGlobalGitFlags_CMissingArg(t *testing.T) {
	_, _, err := parseGlobalGitFlags([]string{"-C"})
	if err == nil {
		t.Error("expected error for -C without path, got nil")
	}
}

func TestParseGlobalGitFlags_cMissingArg(t *testing.T) {
	_, _, err := parseGlobalGitFlags([]string{"-c"})
	if err == nil {
		t.Error("expected error for -c without value, got nil")
	}
}

func TestParseGlobalGitFlags_gitDirNonexistent(t *testing.T) {
	_, _, err := parseGlobalGitFlags([]string{"--git-dir=/nonexistent/path/that/cannot/exist"})
	if err == nil {
		t.Error("expected error for nonexistent --git-dir, got nil")
	}
}

func TestParseGlobalGitFlags_workTreeNonexistent(t *testing.T) {
	_, _, err := parseGlobalGitFlags([]string{"--work-tree=/nonexistent/path/that/cannot/exist"})
	if err == nil {
		t.Error("expected error for nonexistent --work-tree, got nil")
	}
}
