//go:build darwin

package util

import (
	"os"
	"syscall"
)

// RedirectStderr redirects the system-level stderr to the provided file.
func RedirectStderr(file *os.File) {
	_ = syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd()))
}

