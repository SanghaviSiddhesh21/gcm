package config

import (
	"errors"
	"os"
	"testing"
)

// override configPath to use a temp dir for each test
func withTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
}

// ── TestGetAPIKey ─────────────────────────────────────────────────────────────

func TestGetAPIKey_NotSet(t *testing.T) {
	withTempHome(t)
	_, err := GetAPIKey()
	if !errors.Is(err, ErrNotSet) {
		t.Errorf("GetAPIKey() error = %v, want ErrNotSet", err)
	}
}

func TestGetAPIKey_NoFile(t *testing.T) {
	withTempHome(t)
	_, err := GetAPIKey()
	if !errors.Is(err, ErrNotSet) {
		t.Errorf("GetAPIKey() with no file: error = %v, want ErrNotSet", err)
	}
}

// ── TestSetAPIKey ─────────────────────────────────────────────────────────────

func TestSetAPIKey(t *testing.T) {
	withTempHome(t)
	if err := SetAPIKey("gsk_test123"); err != nil {
		t.Fatalf("SetAPIKey() unexpected error: %v", err)
	}
	val, err := GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey() after set: unexpected error: %v", err)
	}
	if val != "gsk_test123" {
		t.Errorf("GetAPIKey() = %q, want %q", val, "gsk_test123")
	}
}

func TestSetAPIKey_Overwrite(t *testing.T) {
	withTempHome(t)
	_ = SetAPIKey("first")
	if err := SetAPIKey("second"); err != nil {
		t.Fatalf("SetAPIKey() overwrite unexpected error: %v", err)
	}
	val, _ := GetAPIKey()
	if val != "second" {
		t.Errorf("GetAPIKey() = %q, want %q", val, "second")
	}
}

// ── TestUnsetAPIKey ───────────────────────────────────────────────────────────

func TestUnsetAPIKey(t *testing.T) {
	withTempHome(t)
	_ = SetAPIKey("gsk_test123")
	if err := UnsetAPIKey(); err != nil {
		t.Fatalf("UnsetAPIKey() unexpected error: %v", err)
	}
	_, err := GetAPIKey()
	if !errors.Is(err, ErrNotSet) {
		t.Errorf("GetAPIKey() after unset: error = %v, want ErrNotSet", err)
	}
}

func TestUnsetAPIKey_NoFile(t *testing.T) {
	withTempHome(t)
	// unset when config file doesn't exist should not error
	if err := UnsetAPIKey(); err != nil {
		t.Errorf("UnsetAPIKey() with no file: unexpected error: %v", err)
	}
}

// ── TestFilePermissions ───────────────────────────────────────────────────────

func TestFilePermissions(t *testing.T) {
	withTempHome(t)
	_ = SetAPIKey("gsk_test123")

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}
