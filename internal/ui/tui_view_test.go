package ui

import (
	"testing"
)

func TestRenderTagLipgloss(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantEmpty bool
	}{
		{
			name:      "empty tag returns empty string",
			tag:       "",
			wantEmpty: true,
		},
		{
			name:      "unknown tag returns non-empty",
			tag:       "somerandometag",
			wantEmpty: false,
		},
		{
			name:      "local tag no status",
			tag:       "[Local]",
			wantEmpty: false,
		},
		{
			name:      "remote tag insync",
			tag:       "[Remote] InSync",
			wantEmpty: false,
		},
		{
			name:      "remote tag ahead",
			tag:       "[Remote] ↑2",
			wantEmpty: false,
		},
		{
			name:      "remote tag behind",
			tag:       "[Remote] ↓3",
			wantEmpty: false,
		},
		{
			name:      "remote tag diverged",
			tag:       "[Remote] ↑1↓2",
			wantEmpty: false,
		},
		{
			name:      "remote tag unknown",
			tag:       "[Remote] ?",
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTagLipgloss(tt.tag)
			if tt.wantEmpty && got != "" {
				t.Errorf("renderTagLipgloss(%q) = %q, want empty", tt.tag, got)
			}
			if !tt.wantEmpty && got == "" {
				t.Errorf("renderTagLipgloss(%q) = empty, want non-empty", tt.tag)
			}
		})
	}
}

func TestBuildItems(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		items := buildItems(nil, nil, "", nil)
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("category with no branches", func(t *testing.T) {
		items := buildItems(
			[]string{"work"},
			map[string][]string{"work": {}},
			"",
			nil,
		)
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].kind != itemCategory {
			t.Errorf("expected itemCategory, got %v", items[0].kind)
		}
	})

	t.Run("isCurrent set on current branch", func(t *testing.T) {
		items := buildItems(
			[]string{"work"},
			map[string][]string{"work": {"main", "feature"}},
			"feature",
			nil,
		)
		var currentCount int
		for _, it := range items {
			if it.kind == itemBranch && it.isCurrent {
				currentCount++
				if it.label != "feature" {
					t.Errorf("isCurrent set on wrong branch %q", it.label)
				}
			}
		}
		if currentCount != 1 {
			t.Errorf("expected 1 current branch, got %d", currentCount)
		}
	})

	t.Run("isLast set only on last branch in category", func(t *testing.T) {
		items := buildItems(
			[]string{"work"},
			map[string][]string{"work": {"a", "b", "c"}},
			"",
			nil,
		)
		branches := make([]item, 0)
		for _, it := range items {
			if it.kind == itemBranch {
				branches = append(branches, it)
			}
		}
		if len(branches) != 3 {
			t.Fatalf("expected 3 branch items, got %d", len(branches))
		}
		if branches[0].isLast {
			t.Error("first branch should not be isLast")
		}
		if branches[1].isLast {
			t.Error("middle branch should not be isLast")
		}
		if !branches[2].isLast {
			t.Error("last branch should be isLast")
		}
	})

	t.Run("multiple categories produce correct item count", func(t *testing.T) {
		items := buildItems(
			[]string{"work", "personal"},
			map[string][]string{
				"work":     {"a", "b"},
				"personal": {"c"},
			},
			"",
			nil,
		)
		// 2 categories + 2 branches + 1 branch = 5
		if len(items) != 5 {
			t.Errorf("expected 5 items, got %d", len(items))
		}
	})

	t.Run("tag assigned to branch item", func(t *testing.T) {
		items := buildItems(
			[]string{"work"},
			map[string][]string{"work": {"main"}},
			"",
			map[string]string{"main": "[Remote] InSync"},
		)
		for _, it := range items {
			if it.kind == itemBranch && it.label == "main" {
				if it.tag != "[Remote] InSync" {
					t.Errorf("expected tag %q, got %q", "[Remote] InSync", it.tag)
				}
			}
		}
	})
}
