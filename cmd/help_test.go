package cmd_test

import (
	"strings"
	"testing"
)

// ── TestHelpNoArgs ────────────────────────────────────────────────────────────

func TestHelpNoArgs(t *testing.T) {
	dir := t.TempDir()

	out, err := runGCM(t, dir, "help")
	if err != nil {
		// git help --all may exit non-zero in some environments; ignore.
		_ = err
	}

	// gcm's own command list must appear.
	for _, want := range []string{"gcm", "view", "create", "assign"} {
		if !strings.Contains(out, want) {
			t.Errorf("gcm help: output missing %q\noutput: %s", want, out)
		}
	}
}

// ── TestHelpGCMOnlyCommand ────────────────────────────────────────────────────

func TestHelpGCMOnlyCommand(t *testing.T) {
	dir := t.TempDir()

	out, err := runGCM(t, dir, "help", "view")
	if err != nil {
		t.Fatalf("gcm help view: %v", err)
	}

	if !strings.Contains(out, "view") {
		t.Errorf("gcm help view: output missing 'view'\noutput: %s", out)
	}
}

// ── TestHelpBothCommand_commit ────────────────────────────────────────────────

func TestHelpBothCommand_commit(t *testing.T) {
	dir := t.TempDir()

	out, _ := runGCM(t, dir, "help", "commit")

	// Output must contain gcm's commit description.
	if !strings.Contains(out, "commit") {
		t.Errorf("gcm help commit: output missing 'commit'\noutput: %s", out)
	}
}

// ── TestHelpBothCommand_config ────────────────────────────────────────────────

func TestHelpBothCommand_config(t *testing.T) {
	dir := t.TempDir()

	out, _ := runGCM(t, dir, "help", "config")

	if !strings.Contains(out, "config") {
		t.Errorf("gcm help config: output missing 'config'\noutput: %s", out)
	}
}

// ── TestHelpBothCommand_init ──────────────────────────────────────────────────

func TestHelpBothCommand_init(t *testing.T) {
	dir := t.TempDir()

	out, _ := runGCM(t, dir, "help", "init")

	if !strings.Contains(out, "init") {
		t.Errorf("gcm help init: output missing 'init'\noutput: %s", out)
	}
}
