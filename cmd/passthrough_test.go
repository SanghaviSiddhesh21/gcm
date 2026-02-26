package cmd_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// runGCMExitCode runs the gcm binary and returns (output, exitCode).
// Differs from runGCM in that it always returns the exit code (0 on success).
func runGCMExitCode(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(gcmBin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("unexpected non-exit error: %v", err)
	return string(out), -1
}

// ── TestSecurityDenylist_uploadPack ───────────────────────────────────────────

func TestSecurityDenylist_uploadPack(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		args []string
	}{
		{"equals-form", []string{"fetch", "--upload-pack=/tmp/evil", "origin"}},
		{"space-form", []string{"fetch", "--upload-pack", "/tmp/evil", "origin"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, code := runGCMExitCode(t, dir, tt.args...)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (security rejection)", code)
			}
		})
	}
}

// ── TestSecurityDenylist_extURL ───────────────────────────────────────────────

func TestSecurityDenylist_extURL(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		args []string
	}{
		{"ext-in-fetch", []string{"fetch", "ext::bash -c 'id'"}},
		{"fd-in-fetch", []string{"fetch", "fd::0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, code := runGCMExitCode(t, dir, tt.args...)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (security rejection)", code)
			}
		})
	}
}

// ── TestPushUpstreamNotBlocked ────────────────────────────────────────────────

// TestPushUpstreamNotBlocked verifies that `gcm push -u origin main` is NOT
// rejected by the security denylist. -u means "set upstream" for push, not
// --upload-pack. It will fail at the git level (no remote), but must not exit 2.
func TestPushUpstreamNotBlocked(t *testing.T) {
	dir := t.TempDir()
	_, code := runGCMExitCode(t, dir, "push", "-u", "origin", "main")
	if code == 2 {
		t.Error("gcm push -u was rejected with exit 2 (security denylist); -u must not be blocked for push")
	}
}

// ── TestVersionFlag ───────────────────────────────────────────────────────────

// TestVersionFlag verifies that `gcm --version` prints gcm's own version
// string and does NOT forward to `git --version` (which would print "git version ...").
func TestVersionFlag(t *testing.T) {
	dir := t.TempDir()
	out, code := runGCMExitCode(t, dir, "--version")
	if code != 0 {
		t.Fatalf("gcm --version exited %d, want 0\noutput: %s", code, out)
	}
	if !strings.Contains(out, "gcm") {
		t.Errorf("gcm --version output %q does not contain 'gcm'", out)
	}
	if strings.Contains(out, "git version") {
		t.Errorf("gcm --version output %q contains 'git version' — passthrough fired instead of version intercept", out)
	}
}
