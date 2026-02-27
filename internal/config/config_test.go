package config

import (
	"errors"
	"os"
	"sync"
	"testing"
)

// withTempHome sets HOME to a fresh temp dir and resets the sync.Once cache.
func withTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	configOnce = sync.Once{}
	t.Cleanup(func() { configOnce = sync.Once{} })
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
	configOnce = sync.Once{}
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
	configOnce = sync.Once{}
	if err := SetAPIKey("second"); err != nil {
		t.Fatalf("SetAPIKey() overwrite unexpected error: %v", err)
	}
	configOnce = sync.Once{}
	val, _ := GetAPIKey()
	if val != "second" {
		t.Errorf("GetAPIKey() = %q, want %q", val, "second")
	}
}

// ── TestUnsetAPIKey ───────────────────────────────────────────────────────────

func TestUnsetAPIKey(t *testing.T) {
	withTempHome(t)
	_ = SetAPIKey("gsk_test123")
	configOnce = sync.Once{}
	if err := UnsetAPIKey(); err != nil {
		t.Fatalf("UnsetAPIKey() unexpected error: %v", err)
	}
	configOnce = sync.Once{}
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

// ── TestGetOrCreateInstallID ──────────────────────────────────────────────────

func TestGetOrCreateInstallID_DisabledByCI(t *testing.T) {
	withTempHome(t)
	t.Setenv("CI", "1")
	id, err := GetOrCreateInstallID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("GetOrCreateInstallID() = %q, want empty string when CI set", id)
	}
}

func TestGetOrCreateInstallID_GeneratesAndPersists(t *testing.T) {
	withTempHome(t)
	id1, err := GetOrCreateInstallID()
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if id1 == "" {
		t.Fatal("GetOrCreateInstallID() returned empty ID on first call")
	}
	// Reset cache — second call must return same ID from disk.
	configOnce = sync.Once{}
	id2, err := GetOrCreateInstallID()
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if id1 != id2 {
		t.Errorf("GetOrCreateInstallID() not stable: %q != %q", id1, id2)
	}
}

func TestGetOrCreateInstallID_DoesNotAffectAPIKey(t *testing.T) {
	withTempHome(t)
	_ = SetAPIKey("gsk_test123")
	configOnce = sync.Once{}

	id, err := GetOrCreateInstallID()
	if err != nil {
		t.Fatalf("GetOrCreateInstallID() unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("GetOrCreateInstallID() returned empty ID")
	}

	configOnce = sync.Once{}
	key, err := GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey() after GetOrCreateInstallID: %v", err)
	}
	if key != "gsk_test123" {
		t.Errorf("GetAPIKey() = %q, want %q after install ID created", key, "gsk_test123")
	}
}

// ── TestSave_AtomicWrite ──────────────────────────────────────────────────────

func TestSave_AtomicWrite(t *testing.T) {
	withTempHome(t)
	_ = SetAPIKey("gsk_test")

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath() error: %v", err)
	}
	// After a successful save, the .tmp file must not exist.
	_, statErr := os.Stat(path + ".tmp")
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Error("save() left a .tmp file after successful write")
	}
}
