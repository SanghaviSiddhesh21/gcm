package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── TestStorePath ─────────────────────────────────────────────────────────────

func TestStorePath(t *testing.T) {
	got := StorePath("/some/repo/.git")
	want := filepath.Join("/some/repo/.git", "gcm.json")
	if got != want {
		t.Errorf("StorePath() = %q, want %q", got, want)
	}
}

// ── TestNewStore ──────────────────────────────────────────────────────────────

func TestNewStore(t *testing.T) {
	s := NewStore()

	if s.Version != StoreVersion {
		t.Errorf("Version = %q, want %q", s.Version, StoreVersion)
	}
	if len(s.Categories) != 1 {
		t.Fatalf("len(Categories) = %d, want 1", len(s.Categories))
	}
	cat := s.Categories[0]
	if cat.Name != UncategorizedName {
		t.Errorf("Categories[0].Name = %q, want %q", cat.Name, UncategorizedName)
	}
	if !cat.Immutable {
		t.Error("Categories[0].Immutable = false, want true")
	}
	if s.Assignments == nil {
		t.Error("Assignments is nil, want non-nil map")
	}
	if len(s.Assignments) != 0 {
		t.Errorf("len(Assignments) = %d, want 0", len(s.Assignments))
	}
}

// ── TestValidateCategoryName ──────────────────────────────────────────────────

func TestValidateCategoryName(t *testing.T) {
	valid64 := strings.Repeat("a", 64)
	invalid65 := strings.Repeat("a", 65)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// valid
		{"simple", "feature", false},
		{"with-hyphen", "team-alice", false},
		{"single-char", "a", false},
		{"alphanumeric", "v2", false},
		{"mixed-case", "HotFix", false},
		{"exactly-64-chars", valid64, false},
		// invalid
		{"empty", "", true},
		{"whitespace-only", "  ", true},
		{"reserved-name", UncategorizedName, true},
		{"has-space", "has space", true},
		{"has-special-char", "has!", true},
		{"has-underscore", "team_alice", true},
		{"too-long", invalid65, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCategoryName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCategoryName(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidName) {
				t.Errorf("ValidateCategoryName(%q) error = %v, want ErrInvalidName", tt.input, err)
			}
		})
	}
}

// ── TestCategoryExists ────────────────────────────────────────────────────────

func TestCategoryExists(t *testing.T) {
	s := NewStore()
	_ = s.AddCategory("feature")

	if !s.CategoryExists(UncategorizedName) {
		t.Error("CategoryExists(Uncategorized) = false, want true")
	}
	if !s.CategoryExists("feature") {
		t.Error("CategoryExists(feature) = false, want true")
	}
	if s.CategoryExists("missing") {
		t.Error("CategoryExists(missing) = true, want false")
	}
}

// ── TestGetCategory ───────────────────────────────────────────────────────────

func TestGetCategory(t *testing.T) {
	s := NewStore()
	_ = s.AddCategory("feature")

	cat := s.GetCategory("feature")
	if cat == nil {
		t.Fatal("GetCategory(feature) = nil, want non-nil")
	}
	if cat.Name != "feature" {
		t.Errorf("GetCategory(feature).Name = %q, want %q", cat.Name, "feature")
	}

	if s.GetCategory("missing") != nil {
		t.Error("GetCategory(missing) != nil, want nil")
	}
}

// ── TestAddCategory ───────────────────────────────────────────────────────────

func TestAddCategory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s := NewStore()
		if err := s.AddCategory("feature"); err != nil {
			t.Fatalf("AddCategory() unexpected error: %v", err)
		}
		if !s.CategoryExists("feature") {
			t.Error("category not found after AddCategory")
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		s := NewStore()
		_ = s.AddCategory("feature")
		err := s.AddCategory("feature")
		if !errors.Is(err, ErrCategoryExists) {
			t.Errorf("AddCategory(duplicate) error = %v, want ErrCategoryExists", err)
		}
	})

	t.Run("invalid-name", func(t *testing.T) {
		s := NewStore()
		err := s.AddCategory("has space")
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("AddCategory(invalid) error = %v, want ErrInvalidName", err)
		}
	})

	t.Run("reserved-name", func(t *testing.T) {
		s := NewStore()
		err := s.AddCategory(UncategorizedName)
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("AddCategory(Uncategorized) error = %v, want ErrInvalidName", err)
		}
	})
}

// ── TestGetAssignment ─────────────────────────────────────────────────────────

func TestGetAssignment(t *testing.T) {
	s := NewStore()
	_ = s.AddCategory("feature")
	_ = s.AssignBranch("my-branch", "feature")

	got := s.GetAssignment("my-branch")
	if got != "feature" {
		t.Errorf("GetAssignment(my-branch) = %q, want %q", got, "feature")
	}

	got = s.GetAssignment("unknown-branch")
	if got != UncategorizedName {
		t.Errorf("GetAssignment(unknown) = %q, want %q", got, UncategorizedName)
	}
}

// ── TestAssignBranch ──────────────────────────────────────────────────────────

func TestAssignBranch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s := NewStore()
		_ = s.AddCategory("feature")
		if err := s.AssignBranch("my-branch", "feature"); err != nil {
			t.Fatalf("AssignBranch() unexpected error: %v", err)
		}
		if s.Assignments["my-branch"] != "feature" {
			t.Errorf("Assignments[my-branch] = %q, want %q", s.Assignments["my-branch"], "feature")
		}
	})

	t.Run("category-not-found", func(t *testing.T) {
		s := NewStore()
		err := s.AssignBranch("my-branch", "missing-category")
		if !errors.Is(err, ErrCategoryNotFound) {
			t.Errorf("AssignBranch() error = %v, want ErrCategoryNotFound", err)
		}
	})
}

// ── TestBranchesInCategory ────────────────────────────────────────────────────

func TestBranchesInCategory(t *testing.T) {
	s := NewStore()
	_ = s.AddCategory("feature")
	_ = s.AssignBranch("feature-branch", "feature")

	allBranches := []string{"main", "develop", "feature-branch"}

	t.Run("uncategorized-returns-unassigned-branches", func(t *testing.T) {
		branches, err := s.BranchesInCategory(UncategorizedName, allBranches)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(branches) != 2 {
			t.Errorf("len(branches) = %d, want 2", len(branches))
		}
	})

	t.Run("named-category-returns-assigned-branches", func(t *testing.T) {
		branches, err := s.BranchesInCategory("feature", allBranches)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(branches) != 1 || branches[0] != "feature-branch" {
			t.Errorf("branches = %v, want [feature-branch]", branches)
		}
	})

	t.Run("empty-all-branches", func(t *testing.T) {
		branches, err := s.BranchesInCategory("feature", []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(branches) != 0 {
			t.Errorf("len(branches) = %d, want 0", len(branches))
		}
	})

	t.Run("stale-assignment-excluded", func(t *testing.T) {
		// feature-branch is assigned to feature but is NOT in allBranches
		branches, err := s.BranchesInCategory("feature", []string{"main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(branches) != 0 {
			t.Errorf("len(branches) = %d, want 0 (stale assignment must be excluded)", len(branches))
		}
	})

	t.Run("category-not-found", func(t *testing.T) {
		_, err := s.BranchesInCategory("missing", allBranches)
		if !errors.Is(err, ErrCategoryNotFound) {
			t.Errorf("error = %v, want ErrCategoryNotFound", err)
		}
	})
}

// ── TestRemoveCategory ────────────────────────────────────────────────────────

func TestRemoveCategory(t *testing.T) {
	t.Run("success-removes-category-and-assignments", func(t *testing.T) {
		s := NewStore()
		_ = s.AddCategory("feature")
		_ = s.AddCategory("hotfix")
		_ = s.AssignBranch("branch-a", "feature")
		_ = s.AssignBranch("branch-b", "hotfix")

		if err := s.RemoveCategory("feature"); err != nil {
			t.Fatalf("RemoveCategory() unexpected error: %v", err)
		}
		if s.CategoryExists("feature") {
			t.Error("category still exists after removal")
		}
		if _, exists := s.Assignments["branch-a"]; exists {
			t.Error("assignment for removed category still present")
		}
		// hotfix and its assignment must be untouched
		if !s.CategoryExists("hotfix") {
			t.Error("hotfix category incorrectly removed")
		}
		if s.Assignments["branch-b"] != "hotfix" {
			t.Error("hotfix assignment incorrectly removed")
		}
	})

	t.Run("not-found", func(t *testing.T) {
		s := NewStore()
		err := s.RemoveCategory("missing")
		if !errors.Is(err, ErrCategoryNotFound) {
			t.Errorf("error = %v, want ErrCategoryNotFound", err)
		}
	})

	t.Run("immutable-uncategorized", func(t *testing.T) {
		s := NewStore()
		err := s.RemoveCategory(UncategorizedName)
		if !errors.Is(err, ErrImmutableCategory) {
			t.Errorf("error = %v, want ErrImmutableCategory", err)
		}
	})
}

// ── TestSaveLoad ──────────────────────────────────────────────────────────────

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()

	original := NewStore()
	_ = original.AddCategory("feature")
	_ = original.AssignBranch("my-branch", "feature")

	if err := Save(dir, original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, original.Version)
	}
	if len(loaded.Categories) != len(original.Categories) {
		t.Errorf("len(Categories) = %d, want %d", len(loaded.Categories), len(original.Categories))
	}
	if loaded.Assignments["my-branch"] != "feature" {
		t.Errorf("Assignments[my-branch] = %q, want %q", loaded.Assignments["my-branch"], "feature")
	}
}

// ── TestLoadNotInitialized ────────────────────────────────────────────────────

func TestLoadNotInitialized(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("Load() error = %v, want ErrNotInitialized", err)
	}
}

// ── TestLoadCorrupted ─────────────────────────────────────────────────────────

func TestLoadCorrupted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(StorePath(dir), []byte("not valid json {{{"), 0o600); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load() expected error for corrupted file, got nil")
	}
	if errors.Is(err, ErrNotInitialized) {
		t.Error("Load() returned ErrNotInitialized for corrupted file, want parse error")
	}
}

// ── TestUnassignBranch ────────────────────────────────────────────────────────

func TestUnassignBranch(t *testing.T) {
	t.Run("branch-with-assignment-removed", func(t *testing.T) {
		s := NewStore()
		_ = s.AddCategory("feature")
		_ = s.AssignBranch("my-branch", "feature")

		s.UnassignBranch("my-branch")

		if _, ok := s.Assignments["my-branch"]; ok {
			t.Error("assignment still present after UnassignBranch")
		}
		if s.GetAssignment("my-branch") != UncategorizedName {
			t.Errorf("GetAssignment after unassign = %q, want %q", s.GetAssignment("my-branch"), UncategorizedName)
		}
	})

	t.Run("branch-with-no-assignment-is-noop", func(t *testing.T) {
		s := NewStore()
		// Must not panic or error.
		s.UnassignBranch("nonexistent-branch")
		if len(s.Assignments) != 0 {
			t.Errorf("Assignments len = %d, want 0", len(s.Assignments))
		}
	})

	t.Run("save-round-trip", func(t *testing.T) {
		dir := t.TempDir()
		s := NewStore()
		_ = s.AddCategory("feature")
		_ = s.AssignBranch("my-branch", "feature")

		s.UnassignBranch("my-branch")
		if err := Save(dir, s); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		loaded, err := Load(dir)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if _, ok := loaded.Assignments["my-branch"]; ok {
			t.Error("assignment still present after Save/Load round-trip")
		}
	})
}

// ── TestRenameBranch ──────────────────────────────────────────────────────────

func TestRenameBranch(t *testing.T) {
	t.Run("branch-with-assignment-key-updated", func(t *testing.T) {
		s := NewStore()
		_ = s.AddCategory("feature")
		_ = s.AssignBranch("old-branch", "feature")

		s.RenameBranch("old-branch", "new-branch")

		if _, ok := s.Assignments["old-branch"]; ok {
			t.Error("old key still present after RenameBranch")
		}
		if s.Assignments["new-branch"] != "feature" {
			t.Errorf("Assignments[new-branch] = %q, want %q", s.Assignments["new-branch"], "feature")
		}
	})

	t.Run("branch-with-no-assignment-is-noop", func(t *testing.T) {
		s := NewStore()
		before := len(s.Assignments)
		s.RenameBranch("old-branch", "new-branch")
		if len(s.Assignments) != before {
			t.Error("Assignments changed for branch with no assignment")
		}
		if _, ok := s.Assignments["new-branch"]; ok {
			t.Error("new key created for branch with no assignment")
		}
	})

	t.Run("new-name-overwrites-stale-entry", func(t *testing.T) {
		s := NewStore()
		_ = s.AddCategory("feature")
		_ = s.AddCategory("hotfix")
		_ = s.AssignBranch("old-branch", "hotfix")
		// Manually place a stale entry at the new name.
		s.Assignments["new-branch"] = "feature"

		s.RenameBranch("old-branch", "new-branch")

		if s.Assignments["new-branch"] != "hotfix" {
			t.Errorf("Assignments[new-branch] = %q, want %q", s.Assignments["new-branch"], "hotfix")
		}
		if _, ok := s.Assignments["old-branch"]; ok {
			t.Error("old key still present")
		}
	})
}

// ── TestLoadOrCreate ──────────────────────────────────────────────────────────

func TestLoadOrCreate(t *testing.T) {
	t.Run("store-exists-loads-without-modification", func(t *testing.T) {
		dir := t.TempDir()
		original := NewStore()
		_ = original.AddCategory("feature")
		_ = original.AssignBranch("my-branch", "feature")
		if err := Save(dir, original); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		loaded, err := LoadOrCreate(dir)
		if err != nil {
			t.Fatalf("LoadOrCreate() error: %v", err)
		}
		if loaded.Assignments["my-branch"] != "feature" {
			t.Errorf("assignment = %q, want %q", loaded.Assignments["my-branch"], "feature")
		}
	})

	t.Run("store-not-found-creates-and-saves-new", func(t *testing.T) {
		dir := t.TempDir()

		s, err := LoadOrCreate(dir)
		if err != nil {
			t.Fatalf("LoadOrCreate() error: %v", err)
		}
		if !s.CategoryExists(UncategorizedName) {
			t.Error("Uncategorized not present in new store")
		}
		// File must exist on disk.
		if _, statErr := os.Stat(StorePath(dir)); statErr != nil {
			t.Errorf("gcm.json not created: %v", statErr)
		}
	})

	t.Run("invalid-json-returns-parse-error-does-not-overwrite", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(StorePath(dir), []byte("not json {{{"), 0o600); err != nil {
			t.Fatalf("test setup: %v", err)
		}

		_, err := LoadOrCreate(dir)
		if err == nil {
			t.Fatal("LoadOrCreate() expected error for invalid JSON, got nil")
		}
		// Original file must not be overwritten.
		data, _ := os.ReadFile(StorePath(dir))
		if string(data) != "not json {{{" {
			t.Error("corrupted file was overwritten — LoadOrCreate must not overwrite on parse error")
		}
	})

	t.Run("empty-git-dir-returns-error-no-files-created", func(t *testing.T) {
		_, err := LoadOrCreate("")
		if err == nil {
			t.Fatal("LoadOrCreate(\"\") expected error, got nil")
		}
		if _, statErr := os.Stat("gcm.json"); statErr == nil {
			t.Error("gcm.json created in CWD with empty gitDir")
		}
	})

	t.Run("nonexistent-git-dir-returns-error-no-files-created", func(t *testing.T) {
		_, err := LoadOrCreate("/nonexistent/path/that/cannot/exist")
		if err == nil {
			t.Fatal("LoadOrCreate() with nonexistent dir expected error, got nil")
		}
	})
}

// ── TestSaveAtomic ────────────────────────────────────────────────────────────

func TestSaveAtomic(t *testing.T) {
	dir := t.TempDir()

	if err := Save(dir, NewStore()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	_, err := os.Stat(StorePath(dir) + ".tmp")
	if !errors.Is(err, os.ErrNotExist) {
		t.Error("tmp file still exists after Save(), expected atomic cleanup")
	}
}
