package cmd

import (
	"strings"
	"testing"
)

// TestGITTerminalPromptInjected verifies that GIT_TERMINAL_PROMPT=0 is added
// to the subprocess environment when stdin is not a TTY (always true in tests).
func TestGITTerminalPromptInjected(t *testing.T) {
	env := buildPassthroughEnv()
	for _, kv := range env {
		if kv == "GIT_TERMINAL_PROMPT=0" {
			return
		}
	}
	t.Error("GIT_TERMINAL_PROMPT=0 not found in passthrough env (expected in non-TTY context)")
}

// TestSecurityDenylist_GITConfigCountStripped verifies that GIT_CONFIG_COUNT,
// GIT_CONFIG_KEY_n, and GIT_CONFIG_VALUE_n are stripped from the subprocess
// environment — they are git 2.31+ equivalents of -c flags and would bypass
// the -c denylist entirely if forwarded.
func TestSecurityDenylist_GITConfigCountStripped(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", "/tmp/evil")

	env := buildPassthroughEnv()
	for _, kv := range env {
		key := strings.SplitN(kv, "=", 2)[0]
		if key == "GIT_CONFIG_COUNT" ||
			strings.HasPrefix(key, "GIT_CONFIG_KEY_") ||
			strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			t.Errorf("passthrough env contains %q — should be stripped", kv)
		}
	}
}
