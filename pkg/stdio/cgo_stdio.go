//go:build !ci

package stdio

/*
#include <stdio.h>
*/
import "C"

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"sync"
	"syscall"
)

type CgoStdio struct {
	sync.Mutex
}

// Capture captures and returns the stderr output produced by the
// given function f.
func (c *CgoStdio) Capture(f func()) string {
	c.Lock()
	defer c.Unlock()

	sourceFd := syscall.Stderr
	var r, w *os.File
	var err error

	originalFd, err := syscall.Dup(sourceFd)
	if err != nil {
		slog.Warn("error in syscall.Dup", "err", err)
		goto executeF
	}

	r, w, err = os.Pipe()
	if err != nil {
		slog.Warn("error in os.Pipe", "err", err)
		goto executeF
	}

	if err = dup2(int(w.Fd()), sourceFd); err != nil {
		slog.Warn("error in dup2", "err", err)
		goto executeF
	}
	defer func() {
		if err = dup2(originalFd, sourceFd); err != nil {
			slog.Warn("error in dup2", "err", err)
		}
		if err = syscall.Close(originalFd); err != nil {
			slog.Warn("error in syscall.Close", "err", err)
		}
	}()

executeF:
	f()
	if err != nil {
		return ""
	}

	C.fflush(C.stderr)
	C.fflush(C.stdout)

	if err = w.Close(); err != nil {
		slog.Warn("error in w.Close", "err", err)
	}
	if err = syscall.Close(sourceFd); err != nil {
		slog.Warn("error in syscall.Close", "err", err)
	}

	out := copyCaptured(r)

	return <-out
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
