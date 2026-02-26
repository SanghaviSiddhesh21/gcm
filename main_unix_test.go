//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"testing"
)

// TestExitCode_SignalKilled verifies that exitCode() returns 128+signal
// when a subprocess is killed by a signal (shell convention).
func TestExitCode_SignalKilled(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	err := cmd.Wait()
	got := exitCode(err)
	want := 128 + int(syscall.SIGTERM)
	if got != want {
		t.Errorf("exitCode for SIGTERM-killed process = %d, want %d (128 + SIGTERM=%d)",
			got, want, int(syscall.SIGTERM))
	}
}
