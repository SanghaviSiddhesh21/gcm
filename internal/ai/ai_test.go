package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/siddhesh/gcm/internal/config"
)

// ── extractCommitMessage ──────────────────────────────────────────────────────

func TestExtractCommitMessage_ConventionalType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"feat colon", "feat: add login", "feat: add login"},
		{"fix with scope", "fix(auth): correct token expiry", "fix(auth): correct token expiry"},
		{"backticks stripped", "`feat: add login`", "feat: add login"},
		{"leading prose", "Here is the message:\nfeat: add login", "feat: add login"},
		{"whitespace trimmed", "  feat: add login  ", "feat: add login"},
		{"all types", "chore: update deps", "chore: update deps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCommitMessage(tt.input)
			if got != tt.want {
				t.Errorf("extractCommitMessage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractCommitMessage_Fallback(t *testing.T) {
	// No conventional type — returns first non-empty line
	got := extractCommitMessage("some random message")
	if got != "some random message" {
		t.Errorf("extractCommitMessage() fallback = %q, want %q", got, "some random message")
	}
}

func TestExtractCommitMessage_Empty(t *testing.T) {
	got := extractCommitMessage("")
	if got != "" {
		t.Errorf("extractCommitMessage(\"\") = %q, want \"\"", got)
	}
}

// ── prepareDiff ───────────────────────────────────────────────────────────────

const sampleDiff = `diff --git a/cmd/foo.go b/cmd/foo.go
index abc..def 100644
--- a/cmd/foo.go
+++ b/cmd/foo.go
@@ -1,3 +1,4 @@
 unchanged line
+added line
-removed line
diff --git a/internal/bar.go b/internal/bar.go
index 111..222 100644
--- a/internal/bar.go
+++ b/internal/bar.go
@@ -5,2 +5,3 @@
+another added line`

func TestPrepareDiff_FileList(t *testing.T) {
	result := prepareDiff(sampleDiff)
	if !strings.Contains(result, "cmd/foo.go") {
		t.Error("prepareDiff() missing cmd/foo.go in file list")
	}
	if !strings.Contains(result, "internal/bar.go") {
		t.Error("prepareDiff() missing internal/bar.go in file list")
	}
}

func TestPrepareDiff_StripsContextLines(t *testing.T) {
	result := prepareDiff(sampleDiff)
	if strings.Contains(result, "unchanged line") {
		t.Error("prepareDiff() should strip unchanged context lines")
	}
}

func TestPrepareDiff_KeepsChangedLines(t *testing.T) {
	result := prepareDiff(sampleDiff)
	if !strings.Contains(result, "+added line") {
		t.Error("prepareDiff() missing added line")
	}
	if !strings.Contains(result, "-removed line") {
		t.Error("prepareDiff() missing removed line")
	}
}

func TestPrepareDiff_Truncation(t *testing.T) {
	// Build a diff larger than 6000 chars
	var sb strings.Builder
	sb.WriteString("diff --git a/big.go b/big.go\n--- a/big.go\n+++ b/big.go\n@@ -1 +1 @@\n")
	for i := 0; i < 300; i++ {
		sb.WriteString("+this is a long added line that adds a lot of content to the diff\n")
	}
	result := prepareDiff(sb.String())
	if !strings.HasSuffix(result, "(truncated)") {
		t.Error("prepareDiff() should truncate large diffs with '(truncated)' suffix")
	}
	if len(result) > 6100 {
		t.Errorf("prepareDiff() result length = %d, want <= 6100", len(result))
	}
}

// ── Generate (HTTP behaviour) ─────────────────────────────────────────────────

func newTestGenerator(handler http.HandlerFunc) (*groqGenerator, *httptest.Server) {
	srv := httptest.NewServer(handler)
	return &groqGenerator{url: srv.URL}, srv
}

func TestGenerate_Success(t *testing.T) {
	gen, srv := newTestGenerator(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "feat: add new feature"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	msg, err := gen.Generate(context.Background(), sampleDiff)
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}
	if msg != "feat: add new feature" {
		t.Errorf("Generate() = %q, want %q", msg, "feat: add new feature")
	}
}

func TestGenerate_RateLimited(t *testing.T) {
	gen, srv := newTestGenerator(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer srv.Close()

	_, err := gen.Generate(context.Background(), sampleDiff)
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("Generate() error = %v, want ErrRateLimited", err)
	}
}

func TestGenerate_ServerError(t *testing.T) {
	gen, srv := newTestGenerator(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	_, err := gen.Generate(context.Background(), sampleDiff)
	if !errors.Is(err, ErrGenerationFailed) {
		t.Errorf("Generate() error = %v, want ErrGenerationFailed", err)
	}
}

func TestGenerate_EmptyChoices(t *testing.T) {
	gen, srv := newTestGenerator(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResponse{})
	})
	defer srv.Close()

	_, err := gen.Generate(context.Background(), sampleDiff)
	if !errors.Is(err, ErrGenerationFailed) {
		t.Errorf("Generate() error = %v, want ErrGenerationFailed", err)
	}
}

func TestGenerate_BadJSON(t *testing.T) {
	gen, srv := newTestGenerator(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json"))
	})
	defer srv.Close()

	_, err := gen.Generate(context.Background(), sampleDiff)
	if !errors.Is(err, ErrGenerationFailed) {
		t.Errorf("Generate() bad JSON error = %v, want ErrGenerationFailed", err)
	}
}

func TestGenerate_NetworkError(t *testing.T) {
	gen, srv := newTestGenerator(func(w http.ResponseWriter, r *http.Request) {})
	srv.Close() // close immediately so the request fails

	_, err := gen.Generate(context.Background(), sampleDiff)
	if !errors.Is(err, ErrGenerationFailed) {
		t.Errorf("Generate() network error = %v, want ErrGenerationFailed", err)
	}
}

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Error("New() returned nil")
	}
}

func TestGenerate_UserKeyHeader(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var receivedKey string
	gen, srv := newTestGenerator(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("X-User-Api-Key")
		resp := chatResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Content: "feat: test"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	if err := config.SetAPIKey("test-key-123"); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	_, err := gen.Generate(context.Background(), sampleDiff)
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}
	if receivedKey != "test-key-123" {
		t.Errorf("X-User-Api-Key header = %q, want %q", receivedKey, "test-key-123")
	}
}
