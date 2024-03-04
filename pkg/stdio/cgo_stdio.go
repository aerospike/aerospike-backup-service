//go:build !ci

package stdio

/*
#include <stdio.h>
*/
import "C"

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"syscall"
)

type CgoStdio struct {
	sync.Mutex
	capture bool
}

// NewCgoStdio returns a new CgoStdio.
func NewCgoStdio(capture bool) *CgoStdio {
	return &CgoStdio{
		capture: capture,
	}
}

// Stderr log capturer.
var Stderr *CgoStdio

// Capture captures and returns the stderr output produced by the
// given function f.
func (c *CgoStdio) Capture(f func()) string {
	c.Lock()
	defer c.Unlock()

	output, executed := ExecuteAndCapture(f)
	if !executed {
		f()
	}
	return output
}

func ExecuteAndCapture(f func()) (output string, functionExecuted bool) {
	r, w, err := os.Pipe()
	if err != nil {
		slog.Warn("Error creating pipe: %v", err)
		return "", false
	}
	defer r.Close()
	defer w.Close()

	originalFd, err := syscall.Dup(syscall.Stderr)
	if err != nil {
		slog.Warn("Error duplicating file descriptor: %v", err)
		return "", false
	}
	defer syscall.Close(originalFd) // Ensure originalFd is closed when main() exits

	if err := syscall.Dup2(int(w.Fd()), syscall.Stderr); err != nil {
		slog.Warn("Error redirecting standard error: %v", err)
		return "", false
	}

	// Execute the function
	f()

	// C.fflush(C.stderr)
	// C.fflush(C.stdout)

	if err := syscall.Dup2(originalFd, syscall.Stderr); err != nil {
		slog.Warn("Error restoring standard error: %v", err)
		return "", true
	}

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		slog.Warn("Error reading from pipe: %v", err)
		return "", true
	}

	return buf.String(), true
}

func GetNumFileDescriptors() (int, error) {
	pid := os.Getpid()
	// Path to the file descriptor directory of the process
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)

	// Read the file descriptor directory
	files, err := os.ReadDir(fdDir)
	if err != nil {
		return 0, err
	}

	// Count the number of files (file descriptors)
	numFileDescriptors := len(files)

	return numFileDescriptors, nil
}
