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
	ErrNotConfigured    = errors.New("AI not configured")
	ErrGenerationFailed = errors.New("failed to generate commit message")
	ErrRateLimited      = errors.New("rate limit exceeded")
)

type Generator interface {
	Generate(ctx context.Context, diff string, gist []string, summaryPrev []string, attempt int) (string, error)
}

// IsDiffExhausted returns true once all 6000-char windows of diff are consumed.
func IsDiffExhausted(diff string, attempt int) bool {
	const windowSize = 6000
	_, body := filterDiff(diff)
	return attempt*windowSize >= len(body)
}

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
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "`"))
		if line != "" {
			return line
		}
	}
	return ""
}

func filterDiff(diff string) (files []string, body string) {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) == 4 {
				files = append(files, strings.TrimPrefix(parts[3], "b/"))
			}
			b.WriteString(line + "\n")
		} else if strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "@@") ||
			strings.HasPrefix(line, "+") ||
			strings.HasPrefix(line, "-") {
			b.WriteString(line + "\n")
		}
	}
	return files, b.String()
}

func prepareDiff(diff string, attempt int) string {
	const windowSize = 6000

	files, bodyStr := filterDiff(diff)

	var sb strings.Builder
	sb.WriteString("Changed files:\n")
	for _, f := range files {
		sb.WriteString("  " + f + "\n")
	}
	sb.WriteString("\n")

	start := attempt * windowSize
	if start >= len(bodyStr) {
		start = max(0, len(bodyStr)-windowSize)
	}
	end := start + windowSize
	truncated := end < len(bodyStr)
	if end > len(bodyStr) {
		end = len(bodyStr)
	}
	sb.WriteString(bodyStr[start:end])
	if truncated {
		sb.WriteString("\n... (truncated)")
	}
	return sb.String()
}

func (g *groqGenerator) Generate(ctx context.Context, diff string, gist []string, summaryPrev []string, attempt int) (string, error) {
	var userContent string
	switch {
	case summaryPrev != nil:
		var sb strings.Builder
		sb.WriteString("These are commit messages generated from different parts of the diff — they represent the gist of the changes:\n")
		for _, m := range gist {
			sb.WriteString("  - " + m + "\n")
		}
		if len(summaryPrev) > 0 {
			sb.WriteString("\nAn earlier summary commit message was created from these:\n")
			for _, m := range summaryPrev {
				sb.WriteString("  - " + m + "\n")
			}
			if len(summaryPrev) == 1 {
				sb.WriteString("\nHowever, it has not captured the complete essence of the changes done. ")
			} else {
				sb.WriteString("\nHowever, they have not captured the complete essence of the changes done. ")
			}
		}
		sb.WriteString("Create a single conventional commit message that captures all these changes. ")
		sb.WriteString("Format: <type>(<optional scope>): <description>. Output the commit message only.\n\nValid types: feat, fix, docs, chore, refactor, test, ci.")
		userContent = sb.String()
	case len(gist) == 0:
		userContent = fmt.Sprintf(
			"Write a conventional commit message for this diff. Format: <type>(<optional scope>): <description>. Output the commit message only.\n\nValid types: feat, fix, docs, chore, refactor, test, ci.\n\n%s",
			prepareDiff(diff, attempt),
		)
	default:
		var sb strings.Builder
		sb.WriteString("Previous attempts generated these commit messages from earlier parts of the diff:\n")
		for _, m := range gist {
			sb.WriteString("  - " + m + "\n")
		}
		sb.WriteString("\nThe diff sent earlier may not have captured the full essence of the commit. ")
		sb.WriteString("Using the above as context, write a new conventional commit message for the following additional diff content. ")
		sb.WriteString("Format: <type>(<optional scope>): <description>. Output the commit message only.\n\nValid types: feat, fix, docs, chore, refactor, test, ci.\n\n")
		sb.WriteString(prepareDiff(diff, attempt))
		userContent = sb.String()
	}

	body, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "You are a git commit message generator. Output ONLY the commit message, nothing else. No explanations, no backticks, no markdown. Just the single commit message line.",
			},
			{
				Role:    "user",
				Content: userContent,
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

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // workerURL is a hardcoded constant, not user input
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGenerationFailed, err)
	}
	defer resp.Body.Close() //nolint:errcheck

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
