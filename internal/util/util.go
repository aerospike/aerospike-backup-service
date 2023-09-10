package util

import (
	"bytes"
	"io"
	"os"

	"log/slog"
)

var LogHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	Level:     slog.LevelDebug,
	AddSource: true,
})

// Check panics if the error is not nil.
func Check(e error) {
	if e != nil {
		panic(e)
	}
}

// CaptureStdout returns the stdout output written during the given function execution.
func CaptureStdout(f func()) string {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f() // run the function

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		Check(err)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out := <-outC
	return out
}
