//go:build linux || darwin

package util

import (
	"os"
	"syscall"
)

// RedirectStderr redirects the system-level stderr and stdout to the provided file.
// This captures fatal Go runtime errors (like map races) that bypass the log package.
func RedirectStderr(file *os.File) {
	_ = syscall.Dup2(int(file.Fd()), int(os.Stdout.Fd()))
	_ = syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd()))
}
