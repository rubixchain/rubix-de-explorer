//go:build linux || darwin

package util

import (
	"os"
	"syscall"
)

// RedirectStderr redirects the system-level stderr to the provided file.
// This captures fatal Go runtime errors (like map races) that bypass the log package.
func RedirectStderr(file *os.File) {
	// Only redirect FD 2 (Stderr). Leave FD 1 (Stdout) alone so the user can see the terminal output.
	_ = syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd()))
}
