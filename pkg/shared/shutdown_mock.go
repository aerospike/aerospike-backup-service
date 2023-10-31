//go:build ci

package shared

import (
	"log/slog"
)

// Shutdown performs finalization operations on shared libraries.
func Shutdown() {
	slog.Debug("Shutdown mock call")
}
