package cmd

import (
	"testing"
	"time"
)

// ── SetGlobalGitFlags ─────────────────────────────────────────────────────────

func TestSetGlobalGitFlags(t *testing.T) {
	orig := globalGitFlags
	defer func() { globalGitFlags = orig }()

	flags := []string{"--git-dir=/tmp", "-c", "user.name=Test"}
	SetGlobalGitFlags(flags)
	if len(globalGitFlags) != len(flags) {
		t.Errorf("SetGlobalGitFlags: globalGitFlags = %v, want %v", globalGitFlags, flags)
	}
}

// ── securityDeniedError.Error ─────────────────────────────────────────────────

func TestSecurityDeniedError_Error(t *testing.T) {
	err := securityError("upload-pack denied")
	want := "gcm: security: upload-pack denied"
	if err.Error() != want {
		t.Errorf("securityDeniedError.Error() = %q, want %q", err.Error(), want)
	}
}

// ── checkHelp ─────────────────────────────────────────────────────────────────

func TestCheckHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"--help present", []string{"--help"}, true},
		{"-h present", []string{"-h"}, true},
		{"--help after other args", []string{"commit", "--help"}, true},
		{"-h after other args", []string{"status", "-h"}, true},
		{"--help after --", []string{"--", "--help"}, false},
		{"no help flag", []string{"commit", "-m", "msg"}, false},
		{"empty", []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkHelp(tt.args); got != tt.want {
				t.Errorf("checkHelp(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// ── IsDeniedConfigKey ─────────────────────────────────────────────────────────

func TestIsDeniedConfigKey_denied(t *testing.T) {
	denied := []string{
		"core.fsmonitor",
		"Core.Fsmonitor", // case-insensitive
		"core.editor",
		"core.hookspath",
		"core.gitproxy",
		"sequence.editor",
		"diff.external",
		"diff.tool",
		"diff.guitool",
		"merge.tool",
		"merge.guitool",
		"gpg.program",
		"gpg.ssh.defaultkeycommand",
		"protocol.ext.allow",
		"filter.lfs.clean",
		"filter.lfs.smudge",
		"filter.lfs.process",
		"filter.custom.clean",
		"alias.foo",
		"alias.st",
	}
	for _, key := range denied {
		t.Run(key, func(t *testing.T) {
			if !IsDeniedConfigKey(key) {
				t.Errorf("IsDeniedConfigKey(%q) = false, want true", key)
			}
		})
	}
}

func TestIsDeniedConfigKey_allowed(t *testing.T) {
	allowed := []string{
		"core.sshCommand",
		"credential.helper",
		"user.name",
		"user.email",
		"filter.lfs.required", // only clean/smudge/process are denied
		"http.proxy",
	}
	for _, key := range allowed {
		t.Run(key, func(t *testing.T) {
			if IsDeniedConfigKey(key) {
				t.Errorf("IsDeniedConfigKey(%q) = true, want false", key)
			}
		})
	}
}

// ── checkPassthroughArgs ──────────────────────────────────────────────────────

func TestCheckPassthroughArgs_allowed(t *testing.T) {
	cases := [][]string{
		{"status"},
		{"push", "-u", "origin", "main"}, // -u safe for push
		{"commit", "-m", "msg"},
		{"clone", "https://github.com/foo/bar"},
		{"-c", "user.name=Test", "status"},
		{"-c", "credential.helper=osxkeychain", "fetch"},
	}
	for _, args := range cases {
		if err := checkPassthroughArgs(args); err != nil {
			t.Errorf("checkPassthroughArgs(%v) unexpected error: %v", args, err)
		}
	}
}

func TestCheckPassthroughArgs_denied(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"upload-pack equals", []string{"fetch", "--upload-pack=/tmp/evil"}},
		{"upload-pack space", []string{"fetch", "--upload-pack", "/tmp/evil"}},
		{"receive-pack", []string{"push", "--receive-pack=/tmp/evil"}},
		{"exec flag", []string{"--exec=/tmp/evil"}},
		{"-u in fetch", []string{"fetch", "-u", "origin"}},
		{"-u in clone", []string{"clone", "-u", "origin"}},
		{"ext:: URL", []string{"fetch", "ext::bash -c 'id'"}},
		{"fd:: URL", []string{"fetch", "fd::0"}},
		{"denied -c key", []string{"-c", "core.fsmonitor=/tmp/evil", "status"}},
		{"denied -c inline", []string{"-ccore.editor=/tmp/evil", "status"}},
		{"malformed -c", []string{"-c", "nokeyequals", "status"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := checkPassthroughArgs(tt.args)
			if err == nil {
				t.Errorf("checkPassthroughArgs(%v) = nil, want security error", tt.args)
				return
			}
			if !IsSecurityDenied(err) {
				t.Errorf("expected IsSecurityDenied=true, got false for: %v", err)
			}
		})
	}
}

// ── containsGenerateFlag ──────────────────────────────────────────────────────

func TestContainsGenerateFlag(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"-g"}, true},
		{[]string{"-m", "msg", "-g"}, true},
		{[]string{"-m", "msg"}, false},
		{[]string{}, false},
		{[]string{"--generate"}, false}, // only exact "-g"
	}
	for _, tt := range tests {
		if got := containsGenerateFlag(tt.args); got != tt.want {
			t.Errorf("containsGenerateFlag(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

// ── containsAPIKey ────────────────────────────────────────────────────────────

func TestContainsAPIKey(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"api-key", "mykey"}, true},
		{[]string{"--unset", "api-key"}, true},
		{[]string{"user.name", "Test"}, false},
		{[]string{}, false},
	}
	for _, tt := range tests {
		if got := containsAPIKey(tt.args); got != tt.want {
			t.Errorf("containsAPIKey(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

// ── repoNameFromURL ───────────────────────────────────────────────────────────

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/repo.git", "repo"},
		{"https://github.com/user/repo", "repo"},
		{"git@github.com:user/repo.git", "repo"},
		{"/local/path/to/repo.git", "repo"},
		{"/local/path/to/repo/", "repo"},    // trailing slash stripped
		{"/local/path/to/repo.git/", "repo"}, // both
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := repoNameFromURL(tt.url); got != tt.want {
				t.Errorf("repoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// ── parseRenameArgs ───────────────────────────────────────────────────────────

func TestParseRenameArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantFlag string
		wantOld  string
		wantNew  string
		wantOK   bool
	}{
		{"two args -m", []string{"-m", "old", "new"}, "-m", "old", "new", true},
		{"two args -M", []string{"-M", "old", "new"}, "-M", "old", "new", true},
		{"two args --move", []string{"--move", "old", "new"}, "--move", "old", "new", true},
		{"one arg implicit current", []string{"-m", "newname"}, "-m", "", "newname", true},
		{"no rename flag", []string{"branch", "-d", "foo"}, "", "", "", false},
		{"flag with no args", []string{"-m"}, "-m", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag, old, new_, ok := parseRenameArgs(tt.args)
			if ok != tt.wantOK || flag != tt.wantFlag || old != tt.wantOld || new_ != tt.wantNew {
				t.Errorf("parseRenameArgs(%v) = (%q,%q,%q,%v), want (%q,%q,%q,%v)",
					tt.args, flag, old, new_, ok,
					tt.wantFlag, tt.wantOld, tt.wantNew, tt.wantOK)
			}
		})
	}
}

// ── parseDeleteArgs ───────────────────────────────────────────────────────────

func TestParseDeleteArgs(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		want   []string
		wantOK bool
	}{
		{"single -d", []string{"-d", "feature"}, []string{"feature"}, true},
		{"single -D", []string{"-D", "feature"}, []string{"feature"}, true},
		{"multiple branches", []string{"-d", "a", "b", "c"}, []string{"a", "b", "c"}, true},
		{"flag stops collection", []string{"-d", "a", "-v", "b"}, []string{"a"}, true},
		{"no delete flag", []string{"-m", "old", "new"}, nil, false},
		{"empty", []string{}, nil, false},
		{"-r stops names", []string{"-d", "-r", "origin/foo"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDeleteArgs(tt.args)
			if ok != tt.wantOK {
				t.Errorf("parseDeleteArgs(%v) ok=%v, want %v", tt.args, ok, tt.wantOK)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseDeleteArgs(%v) = %v, want %v", tt.args, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseDeleteArgs(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ── formatSyncTag ─────────────────────────────────────────────────────────────

func TestFormatSyncTag(t *testing.T) {
	tests := []struct {
		ahead, behind int
		want          string
	}{
		{0, 0, "[Remote] InSync"},
		{3, 0, "[Remote] ↑3"},
		{0, 2, "[Remote] ↓2"},
		{1, 1, "[Remote] ↑1 ↓1"},
	}
	for _, tt := range tests {
		if got := formatSyncTag(tt.ahead, tt.behind); got != tt.want {
			t.Errorf("formatSyncTag(%d,%d) = %q, want %q", tt.ahead, tt.behind, got, tt.want)
		}
	}
}

// ── sortedBranches ────────────────────────────────────────────────────────────

func TestSortedBranches(t *testing.T) {
	now := time.Now()
	times := map[string]time.Time{
		"a": now.Add(-1 * time.Hour),
		"b": now.Add(-2 * time.Hour),
		"c": now.Add(-3 * time.Hour),
	}

	t.Run("current branch first", func(t *testing.T) {
		result := sortedBranches([]string{"a", "b", "c"}, "c", times)
		if result[0] != "c" {
			t.Errorf("expected current branch 'c' first, got %v", result)
		}
	})

	t.Run("others sorted newest first", func(t *testing.T) {
		result := sortedBranches([]string{"a", "b", "c"}, "c", times)
		if result[1] != "a" || result[2] != "b" {
			t.Errorf("expected [c a b], got %v", result)
		}
	})

	t.Run("no current branch sorted newest first", func(t *testing.T) {
		result := sortedBranches([]string{"a", "b", "c"}, "main", times)
		if result[0] != "a" || result[1] != "b" || result[2] != "c" {
			t.Errorf("expected [a b c], got %v", result)
		}
	})

	t.Run("single branch returned as-is", func(t *testing.T) {
		result := sortedBranches([]string{"a"}, "a", times)
		if len(result) != 1 || result[0] != "a" {
			t.Errorf("expected [a], got %v", result)
		}
	})
}

// ── findCurrentCategory ───────────────────────────────────────────────────────

func TestFindCurrentCategory(t *testing.T) {
	branchMap := map[string][]string{
		"feature": {"feat-a", "feat-b"},
		"bugfix":  {"fix-1"},
	}
	cats := []string{"feature", "bugfix"}

	if got := findCurrentCategory(cats, branchMap, "feat-b"); got != "feature" {
		t.Errorf("expected 'feature', got %q", got)
	}
	if got := findCurrentCategory(cats, branchMap, "fix-1"); got != "bugfix" {
		t.Errorf("expected 'bugfix', got %q", got)
	}
	if got := findCurrentCategory(cats, branchMap, "main"); got != "" {
		t.Errorf("expected '' for unassigned branch, got %q", got)
	}
}

// ── maxCatTime ────────────────────────────────────────────────────────────────

func TestMaxCatTime(t *testing.T) {
	now := time.Now()
	branchMap := map[string][]string{
		"feature": {"a", "b", "c"},
	}
	times := map[string]time.Time{
		"a": now.Add(-3 * time.Hour),
		"b": now.Add(-1 * time.Hour), // newest
		"c": now.Add(-2 * time.Hour),
	}

	if got := maxCatTime("feature", branchMap, times); !got.Equal(times["b"]) {
		t.Errorf("maxCatTime = %v, want %v (branch 'b')", got, times["b"])
	}
}

func TestMaxCatTime_emptyCategory(t *testing.T) {
	got := maxCatTime("empty", map[string][]string{"empty": {}}, map[string]time.Time{})
	if !got.IsZero() {
		t.Errorf("expected zero time for empty category, got %v", got)
	}
}

// ── sortedCategories ──────────────────────────────────────────────────────────

func TestSortedCategories_currentFirst(t *testing.T) {
	now := time.Now()
	branchMap := map[string][]string{
		"feature": {"a"},
		"bugfix":  {"b"},
	}
	times := map[string]time.Time{
		"a": now.Add(-2 * time.Hour),
		"b": now.Add(-1 * time.Hour), // newer, but feature is current
	}
	result := sortedCategories([]string{"feature", "bugfix"}, "feature", branchMap, times)
	if result[0] != "feature" {
		t.Errorf("expected 'feature' first (current category), got %v", result)
	}
}

func TestSortedCategories_uncategorizedLast(t *testing.T) {
	now := time.Now()
	branchMap := map[string][]string{
		"feature":       {"a"},
		"Uncategorized": {"b"},
	}
	times := map[string]time.Time{
		"a": now.Add(-1 * time.Hour),
		"b": now.Add(-2 * time.Hour),
	}
	result := sortedCategories([]string{"feature", "Uncategorized"}, "", branchMap, times)
	if result[len(result)-1] != "Uncategorized" {
		t.Errorf("expected 'Uncategorized' last, got %v", result)
	}
}

func TestSortedCategories_othersNewestFirst(t *testing.T) {
	now := time.Now()
	branchMap := map[string][]string{
		"alpha": {"a"},
		"beta":  {"b"},
		"gamma": {"c"},
	}
	times := map[string]time.Time{
		"a": now.Add(-3 * time.Hour),
		"b": now.Add(-1 * time.Hour), // newest
		"c": now.Add(-2 * time.Hour),
	}
	result := sortedCategories([]string{"alpha", "beta", "gamma"}, "", branchMap, times)
	if result[0] != "beta" || result[1] != "gamma" || result[2] != "alpha" {
		t.Errorf("expected [beta gamma alpha] by newest commit, got %v", result)
	}
}
