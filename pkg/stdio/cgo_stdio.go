//go:build !ci

package stdio

/*
#include <stdio.h>
*/
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"
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

	return ExecuteAndCapture(f)
}

func ExecuteAndCapture(f func()) string {
	// Create a pipe
	r, w, err := os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("Error creating pipe: %v", err))
	}
	defer r.Close()
	defer w.Close()

	// Duplicate the standard error file descriptor
	originalFd, err := syscall.Dup(syscall.Stderr)
	if err != nil {
		panic(fmt.Sprintf("Error duplicating file descriptor: %v", err))
	}
	defer syscall.Close(originalFd) // Ensure originalFd is closed when main() exits

	// Redirect standard error to the write end of the pipe
	if err := syscall.Dup2(int(w.Fd()), syscall.Stderr); err != nil {
		panic(fmt.Sprintf("Error redirecting standard error: %v", err))
	}
	w.Close() // Close the write end of the pipe

	// Execute the function
	f()

	// Restore standard error to its original destination
	if err := syscall.Dup2(originalFd, syscall.Stderr); err != nil {
		panic(fmt.Sprintf("Error restoring standard error: %v", err))
	}

	// Read the output from the pipe into a string
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		panic(fmt.Sprintf("Error reading from pipe: %v", err))
	}

	return buf.String()
}

func old() string {
	var r, w *os.File
	var err error
	var originalFd int
	for i := 0; i < 5; i++ {
		originalFd, err = syscall.Dup(syscall.Stderr)
		if err != nil {
			slog.Warn("error in syscall.Dup", "sourceFd", syscall.Stderr, "attempt", i, "err", err)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	println(originalFd)
	if err == nil {
		r, w, err = os.Pipe()
		if err != nil {
			slog.Warn("error in os.Pipe", "err", err)
		} else {
			if err = dup2(int(w.Fd()), syscall.Stderr); err != nil {
				slog.Warn("error in dup2", "err", err)
			}
		}
	}

	// f()

	if err != nil {
		return ""
	}

	C.fflush(C.stderr)
	C.fflush(C.stdout)

	if err = w.Close(); err != nil {
		slog.Warn("error in w.Close", "err", err)
	}

	if err = syscall.Close(syscall.Stderr); err != nil {
		slog.Warn("error in syscall.Close", "err", err)
	}

	out := <-copyCaptured(r)

	// c.closeFd(originalFd, syscall.Stderr)
	return out
}

func (c *CgoStdio) closeFd(originalFd int, sourceFd int) {
	attempts := 0
	maxRetries := 5

	for {
		if err := dup2(originalFd, sourceFd); err != nil {
			slog.Warn("error in dup2", "attempt", attempts, "err", err)

			if attempts < maxRetries {
				time.Sleep(time.Second * 1)
				attempts++
				continue
			}
		}
		break
	}

	if err := syscall.Close(originalFd); err != nil {
		slog.Warn("error in syscall.Close", "err", err)
	}
}

func copyCaptured(r *os.File) <-chan string {
	out := make(chan string)
	go func() {
		var b bytes.Buffer
		_, err := io.Copy(&b, r)
		if err != nil {
			slog.Warn("error in io.Copy", "err", err)
			out <- ""
		} else {
			out <- b.String()
		}
		if err = r.Close(); err != nil {
			slog.Warn("error in r.Close", "err", err)
		}
	}()
	return out
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
