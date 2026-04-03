//go:build !windows

package pid

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// isProcessRunning checks whether a process with the given PID is alive
// on Unix-like systems using signal(0), then verifies it is actually a
// picoclaw process via /proc to avoid false positives from PID reuse
// (common in containers).
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal(0) does not kill the process but checks existence on Unix.
	if p.Signal(syscall.Signal(0)) != nil {
		return false
	}
	// Process exists; verify it is actually a picoclaw gateway.
	// On Linux, read /proc/<pid>/cmdline; on other Unix systems where
	// /proc is unavailable, fall back to assuming the process is valid.
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return true
	}
	return strings.Contains(string(cmdline), "picoclaw")
}
