package reload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatcherDetectsMtimeChange(t *testing.T) {
	path := writeTempFile(t, "original")
	var calls atomic.Int32

	watcher := New(path, 20*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	watcher.Start(t.Context())

	require.Never(t, func() bool { return calls.Load() > 0 }, 80*time.Millisecond, 10*time.Millisecond)

	require.NoError(t, os.WriteFile(path, []byte("updated"), 0o600))
	bumpMtime(t, path)

	require.Eventually(t, func() bool { return calls.Load() >= 1 }, time.Second, 10*time.Millisecond)
}

func TestWatcherDoesNotCallbackWhenUnchanged(t *testing.T) {
	path := writeTempFile(t, "stable")
	var calls atomic.Int32

	watcher := New(path, 20*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	watcher.Start(t.Context())

	require.Never(t, func() bool { return calls.Load() > 0 }, 100*time.Millisecond, 10*time.Millisecond)
}

func TestWatcherRetriesAfterCallbackError(t *testing.T) {
	// A failed onChange must not consume the new fingerprint; the next tick retries
	// without another write.
	path := writeTempFile(t, "v1")
	var calls atomic.Int32

	watcher := New(path, 20*time.Millisecond, func(context.Context) error {
		n := calls.Add(1)
		if n == 1 {
			return errors.New("reload failed")
		}
		return nil
	})
	watcher.Start(t.Context())

	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))
	bumpMtime(t, path)

	require.Eventually(t, func() bool { return calls.Load() >= 2 }, time.Second, 10*time.Millisecond)
}

func TestWatcherReloadsWhenFileChangesDuringCallback(t *testing.T) {
	// A rewrite during onChange must not be stored as the baseline; the next tick retries.
	path := writeTempFile(t, "v1")
	var calls atomic.Int32

	watcher := New(path, 20*time.Millisecond, func(context.Context) error {
		n := calls.Add(1)
		if n == 1 {
			require.NoError(t, os.WriteFile(path, []byte("v3-longer"), 0o600))
			bumpMtime(t, path)
		}
		return nil
	})
	watcher.Start(t.Context())

	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))
	bumpMtime(t, path)

	require.Eventually(t, func() bool { return calls.Load() >= 2 }, time.Second, 10*time.Millisecond)
}

func TestWatcherStopsPollingWhenContextCanceled(t *testing.T) {
	path := writeTempFile(t, "v1")
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(t.Context())

	watcher := New(path, 20*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	watcher.Start(ctx)

	cancel()
	require.Never(t, func() bool { return calls.Load() > 0 }, 80*time.Millisecond, 10*time.Millisecond)

	n := calls.Load()
	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))
	bumpMtime(t, path)

	require.Never(t, func() bool { return calls.Load() > n }, 100*time.Millisecond, 10*time.Millisecond)
}

func TestWatcherDetectsSizeChangeWithoutMtimeBump(t *testing.T) {
	// Same mtime, longer contents: size is the only signal.
	path := writeTempFile(t, "ab")
	info, err := os.Stat(path)
	require.NoError(t, err)
	frozen := info.ModTime()

	var calls atomic.Int32
	watcher := New(path, 20*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	watcher.Start(t.Context())

	require.NoError(t, os.WriteFile(path, []byte("abcdef"), 0o600))
	require.NoError(t, os.Chtimes(path, frozen, frozen))

	require.Eventually(t, func() bool { return calls.Load() >= 1 }, time.Second, 10*time.Millisecond)
}

func writeTempFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "watched.txt")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}

func bumpMtime(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(path, info.ModTime().Add(time.Second), info.ModTime().Add(time.Second)))
}
