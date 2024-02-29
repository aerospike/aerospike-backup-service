//go:build !ci

package stdio

/*
#include <stdio.h>
*/
import "C"

import (
	"bytes"
	"errors"
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

	// don't capture shared library logs
	if !c.capture {
		f()
		return ""
	}

	slog.Debug("Descriptors", "limit", getLimit())

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
		c.closeFd(err, originalFd, sourceFd)
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

func (c *CgoStdio) closeFd(err error, originalFd int, sourceFd int) {
	attempts := 0
	maxRetries := 5

	for {
		if err = dup2(originalFd, sourceFd); err != nil {
			slog.Warn("error in dup2", "attempt", attempts, "err", err)

			// Check if the error is caused by a device or resource being busy.
			if errors.Is(err, syscall.EBUSY) && attempts < maxRetries {
				time.Sleep(time.Second * 1) // Delay for 1 second
				attempts++
				continue
			}
		}
		break
	}

	if err = syscall.Close(originalFd); err != nil {
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

func getLimit() syscall.Rlimit {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		fmt.Println("Error getting RLIMIT_NOFILE:", err)
		return syscall.Rlimit{}
	}
	return rLimit
}
