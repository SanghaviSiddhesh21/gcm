package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// git global flags stripped from os.Args by main.go; set once before Execute().
var globalGitFlags []string

// SetGlobalGitFlags stores git global flags so passthroughGit can prepend them.
func SetGlobalGitFlags(flags []string) {
	globalGitFlags = flags
}

// checkHelp reports whether --help or -h appears in args before any -- separator.
func checkHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// deniedConfigKeyPrefixes is the list of config key prefixes (lowercased) that
// cause git to execute arbitrary binaries. core.sshCommand and credential.helper
// are deliberately excluded — they are legitimate in CI/agent automation.
var deniedConfigKeyPrefixes = []string{
	"core.fsmonitor",
	"core.gitproxy",
	"core.hookspath",
	"core.editor",
	"sequence.editor",
	"diff.external",
	"diff.tool",
	"diff.guitool",
	"merge.tool",
	"merge.guitool",
	"gpg.program",
	"gpg.ssh.defaultkeycommand",
	"protocol.ext.allow",
}

// IsDeniedConfigKey reports whether the given config key (any case) is in the denylist.
func IsDeniedConfigKey(key string) bool {
	key = strings.ToLower(key)
	for _, prefix := range deniedConfigKeyPrefixes {
		if key == prefix || strings.HasPrefix(key, prefix+".") {
			return true
		}
	}
	// filter.<name>.{clean,smudge,process} — wildcard match
	parts := strings.SplitN(key, ".", 3)
	if len(parts) == 3 && parts[0] == "filter" {
		return parts[2] == "clean" || parts[2] == "smudge" || parts[2] == "process"
	}
	// alias.* — aliases starting with ! execute via /bin/sh -c
	if strings.HasPrefix(key, "alias.") {
		return true
	}
	return false
}

// fetchFamilySubcmds are the git subcommands where -u means --upload-pack (dangerous).
// For all other subcommands, -u is safe (e.g. git push -u sets upstream).
var fetchFamilySubcmds = map[string]bool{
	"fetch":     true,
	"clone":     true,
	"ls-remote": true,
	"archive":   true,
}

// checkPassthroughArgs validates args before forwarding to git.
// Returns a non-nil error (exit code 2) if a denied flag or URL is detected.
func checkPassthroughArgs(args []string) error {
	// Determine subcommand for context-sensitive checks.
	subcmd := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			subcmd = a
			break
		}
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		// -c key=val denylist (case-normalised, must contain =)
		if arg == "-c" && i+1 < len(args) {
			kv := args[i+1]
			if !strings.Contains(kv, "=") {
				return securityError(fmt.Sprintf("-c %q: malformed argument (missing '=')", kv))
			}
			key := strings.SplitN(kv, "=", 2)[0]
			if IsDeniedConfigKey(key) {
				return securityError(fmt.Sprintf("-c %s denied", key))
			}
			i += 2
			continue
		}
		if strings.HasPrefix(arg, "-c") && len(arg) > 2 {
			kv := arg[2:]
			if !strings.Contains(kv, "=") {
				return securityError(fmt.Sprintf("-c%q: malformed argument (missing '=')", kv))
			}
			key := strings.SplitN(kv, "=", 2)[0]
			if IsDeniedConfigKey(key) {
				return securityError(fmt.Sprintf("-c %s denied", key))
			}
			i++
			continue
		}

		// --upload-pack (= form and space form)
		if arg == "--upload-pack" || strings.HasPrefix(arg, "--upload-pack=") {
			return securityError("--upload-pack denied")
		}
		// --receive-pack
		if arg == "--receive-pack" || strings.HasPrefix(arg, "--receive-pack=") {
			return securityError("--receive-pack denied")
		}
		// --exec
		if arg == "--exec" || strings.HasPrefix(arg, "--exec=") {
			return securityError("--exec denied")
		}
		// -u (only dangerous in fetch-family subcommands)
		if arg == "-u" && fetchFamilySubcmds[subcmd] {
			return securityError("-u (--upload-pack) denied")
		}

		// ext:: and fd:: URL protocols
		if strings.HasPrefix(arg, "ext::") || strings.HasPrefix(arg, "fd::") {
			proto := strings.SplitN(arg, ":", 2)[0] + ":"
			return securityError(fmt.Sprintf("protocol %q denied", proto))
		}

		i++
	}
	return nil
}

func securityError(msg string) error {
	return &securityDeniedError{msg: msg}
}

type securityDeniedError struct{ msg string }

func (e *securityDeniedError) Error() string {
	return "gcm: security: " + e.msg
}

// IsSecurityDenied reports whether err is a security denylist rejection.
func IsSecurityDenied(err error) bool {
	_, ok := err.(*securityDeniedError)
	return ok
}

// buildPassthroughEnv constructs the subprocess environment:
//   - strips GIT_CONFIG_COUNT/KEY_n/VALUE_n env var config bypass vectors
//   - injects GIT_TERMINAL_PROMPT=0 in non-TTY to prevent hanging on credential prompts
func buildPassthroughEnv() []string {
	filtered := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		key := strings.SplitN(kv, "=", 2)[0]
		if key == "GIT_CONFIG_COUNT" ||
			strings.HasPrefix(key, "GIT_CONFIG_KEY_") ||
			strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		filtered = append(filtered, kv)
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		filtered = append(filtered, "GIT_TERMINAL_PROMPT=0")
	}
	return filtered
}
