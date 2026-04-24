//go:build !linux && !darwin

package util

import "os"

// RedirectStderr is a no-op on non-Unix systems.
func RedirectStderr(file *os.File) {
	// Not implemented on Windows
}
