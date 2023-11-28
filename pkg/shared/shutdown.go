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

func shutdownS3API(fullPath *string) {
	if fullPath != nil {
		fileProxyType := C.file_proxy_path_type(C.CString(*fullPath))
		if fileProxyType == C.FILE_PROXY_TYPE_S3 {
			C.file_proxy_cloud_shutdown()
		}
	}
}
