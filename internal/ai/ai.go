package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/siddhesh/gcm/internal/config"
)

const (
	workerURL = "https://gcm-ai-commit.sanghavisiddhesh21.workers.dev"
	model     = "llama-3.1-8b-instant"
)

var (
	// ErrNotConfigured is returned when the worker is unreachable.
	ErrNotConfigured = errors.New("AI not configured")

	// ErrGenerationFailed is returned when the API fails to produce a message.
	ErrGenerationFailed = errors.New("failed to generate commit message")

	// ErrRateLimited is returned when the shared API key is rate limited.
	ErrRateLimited = errors.New("rate limit exceeded")
)

// Generator generates commit messages from git diffs.
type Generator interface {
	Generate(ctx context.Context, diff string) (string, error)
}

// New returns a Generator backed by the Groq API via Cloudflare Worker proxy.
func New() Generator {
	return &groqGenerator{url: workerURL}
}

type groqGenerator struct {
	url string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// extractCommitMessage scans the model's raw response for the first line that
// looks like a conventional commit message, stripping backticks and whitespace.
func extractCommitMessage(raw string) string {
	conventionalTypes := []string{"feat", "fix", "docs", "refactor", "test", "chore", "style", "perf", "ci", "build"}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "`")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, t := range conventionalTypes {
			if strings.HasPrefix(line, t+":") || strings.HasPrefix(line, t+"(") {
				return line
			}
		}
	}
	// Fallback: return first non-empty line
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "`"))
		if line != "" {
			return line
		}
	}
	return ""
}

// prepareDiff strips unchanged context lines from the diff, keeping only file
// headers, hunk headers, and added/removed lines. A file summary is prepended
// so the model always knows what changed even when content is truncated.
func prepareDiff(diff string) string {
	const maxDiffChars = 6000

	var files []string
	var body strings.Builder

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) == 4 {
				files = append(files, strings.TrimPrefix(parts[3], "b/"))
			}
			body.WriteString(line + "\n")
		} else if strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "@@") ||
			strings.HasPrefix(line, "+") ||
			strings.HasPrefix(line, "-") {
			body.WriteString(line + "\n")
		}
	}

	var sb strings.Builder
	sb.WriteString("Changed files:\n")
	for _, f := range files {
		sb.WriteString("  " + f + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(body.String())

	result := sb.String()
	if len(result) > maxDiffChars {
		result = result[:maxDiffChars] + "\n... (truncated)"
	}
	return result
}

// Generate sends the diff to the Cloudflare Worker proxy and returns a
// conventional commit message.
func (g *groqGenerator) Generate(ctx context.Context, diff string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "You are a git commit message generator. Output ONLY the commit message, nothing else. No explanations, no backticks, no markdown. Just the single commit message line.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Write a conventional commit message for this diff. Format: <type>(<optional scope>): <description>. Output the commit message only.\n\n%s", prepareDiff(diff)),
			},
		},
		MaxTokens: 60,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGenerationFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGenerationFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")

	if key, err := config.GetAPIKey(); err == nil {
		req.Header.Set("X-User-Api-Key", key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGenerationFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: server returned %d", ErrGenerationFailed, resp.StatusCode)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("%w: %w", ErrGenerationFailed, err)
	}

	if len(cr.Choices) == 0 {
		return "", ErrGenerationFailed
	}

	return extractCommitMessage(cr.Choices[0].Message.Content), nil
}
