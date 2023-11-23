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
		logError(err)
		goto executeF
	}

	r, w, err = os.Pipe()
	if err != nil {
		logError(err)
		goto executeF
	}

	if err = dup2(int(w.Fd()), sourceFd); err != nil {
		logError(err)
		goto executeF
	}
	defer func() {
		logError(dup2(originalFd, sourceFd))
	}()

executeF:
	f()
	if err != nil {
		return ""
	}

	C.fflush(C.stderr)
	C.fflush(C.stdout)

	logError(w.Close())
	logError(syscall.Close(sourceFd))

	out := copyCaptured(r)

	return <-out
}

func copyCaptured(r *os.File) <-chan string {
	out := make(chan string)
	go func() {
		var b bytes.Buffer
		_, err := io.Copy(&b, r)
		if err != nil {
			logError(err)
			out <- ""
		} else {
			out <- b.String()
		}
	}()
	return out
}

func logError(err error) {
	if err != nil {
		slog.Warn("error in stdio capture", "err", err)
	}
}
