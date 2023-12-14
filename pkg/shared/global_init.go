//go:build !ci

package shared

/*
#include <utils.h>
*/
import "C"

func init() {
	C.g_verbose = true
	C.enable_client_log()
}
