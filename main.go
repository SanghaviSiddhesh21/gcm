package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/siddhesh/gcm/cmd"
	"github.com/siddhesh/gcm/internal/git"
)

func main() {
	remaining, globalFlags, err := parseGlobalGitFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(2)
	}
	os.Args = append(os.Args[:1], remaining...)
	cmd.SetGlobalGitFlags(globalFlags)
	git.SetGlobalFlags(globalFlags)

	if err := cmd.Execute(); err != nil {
		if cmd.IsSecurityDenied(err) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(2)
		}
		// For passthrough errors (exec.ExitError), git already printed its own
		// error message — just forward the exit code without adding a second line.
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		os.Exit(exitCode(err))
	}
}

// parseGlobalGitFlags strips known git global flags from args, applies -C via os.Chdir,
// and returns (remainingArgs, collectedGlobalFlags, error).
func parseGlobalGitFlags(args []string) (remaining []string, globalFlags []string, err error) {
	i := 0
	for i < len(args) {
		arg := args[i]

		// -C <path> — change process CWD (so GetRepoInfo resolves the right repo)
		if arg == "-C" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("-C requires a path argument")
			}
			path := args[i+1]
			if err := os.Chdir(path); err != nil {
				return nil, nil, fmt.Errorf("-C %s: %w", path, err)
			}
			i += 2
			continue
		}

		// --git-dir=<path> — validate then thread via globalFlags into each subprocess env
		if strings.HasPrefix(arg, "--git-dir=") {
			path := arg[len("--git-dir="):]
			if err := validatePath("--git-dir", path); err != nil {
				return nil, nil, err
			}
			globalFlags = append(globalFlags, arg)
			i++
			continue
		}
		// --git-dir <path> (space form)
		if arg == "--git-dir" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--git-dir requires a path argument")
			}
			path := args[i+1]
			if err := validatePath("--git-dir", path); err != nil {
				return nil, nil, err
			}
			globalFlags = append(globalFlags, "--git-dir="+path)
			i += 2
			continue
		}

		// --work-tree=<path>
		if strings.HasPrefix(arg, "--work-tree=") {
			path := arg[len("--work-tree="):]
			if err := validatePath("--work-tree", path); err != nil {
				return nil, nil, err
			}
			globalFlags = append(globalFlags, arg)
			i++
			continue
		}
		if arg == "--work-tree" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--work-tree requires a path argument")
			}
			path := args[i+1]
			if err := validatePath("--work-tree", path); err != nil {
				return nil, nil, err
			}
			globalFlags = append(globalFlags, "--work-tree="+path)
			i += 2
			continue
		}

		// -c key=val — validate key then carry into globalFlags
		if arg == "-c" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("-c requires a key=val argument")
			}
			kv := args[i+1]
			if !strings.Contains(kv, "=") {
				return nil, nil, fmt.Errorf("-c %q: malformed argument (missing '=')", kv)
			}
			key := strings.SplitN(kv, "=", 2)[0]
			if cmd.IsDeniedConfigKey(key) {
				return nil, nil, fmt.Errorf("gcm: security: -c %s denied", key)
			}
			globalFlags = append(globalFlags, "-c", kv)
			i += 2
			continue
		}

		// Stop at the first non-global-flag token (subcommand or positional arg).
		// Also stop at -- separator.
		remaining = append(remaining, args[i:]...)
		break
	}

	return remaining, globalFlags, nil
}

// validatePath resolves a path to absolute and verifies it exists.
// Used for --git-dir and --work-tree to catch typos early.
func validatePath(flag, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", flag, path, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("%s %q: path does not exist", flag, path)
	}
	return nil
}

// exitCode extracts the process exit code from an exec error.
// Returns 128+signal for signal-killed processes (shell convention).
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 1
	}
	code := exitErr.ExitCode()
	if code != -1 {
		return code
	}
	// Killed by signal — shell convention: 128 + signal number
	if ws, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok {
		if ws.Signaled() {
			return 128 + int(ws.Signal())
		}
	}
	return 1
}
