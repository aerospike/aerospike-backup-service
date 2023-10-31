//go:build !ci

package shared

/*
#include <file_proxy.h>
*/
import "C"
import (
	"log/slog"
)

// Shutdown performs finalization operations on shared resources.
func Shutdown() {
	slog.Info("Finalizing shared resources")
	C.file_proxy_cloud_shutdown()
}
