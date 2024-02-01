//go:build !ci

package shared

/*
#include <stdbool.h>
#include <stdint.h>

#include <file_proxy.h>
#include <aerospike/as_config.h>
*/
import "C"
import (
	"strings"

	"github.com/aerospike/backup/pkg/model"
)

func setCString(cchar **C.char, str *string) {
	if str != nil {
		*cchar = C.CString(*str)
	}
}

func setCInt(cint *C.int, i *int32) {
	if i != nil {
		*cint = C.int(*i)
	}
}

func setCUint(cint *C.uint, i *uint32) {
	if i != nil {
		*cint = C.uint(*i)
	}
}

func setCLong(clong *C.int64_t, l *int64) {
	if l != nil {
		*clong = C.int64_t(*l)
	}
}

func setCUlong(clong *C.uint64_t, l *uint64) {
	if l != nil {
		*clong = C.uint64_t(*l)
	}
}

func setCBool(cbool *C.bool, b *bool) {
	if b != nil {
		*cbool = C.bool(*b)
	}
}

func setS3LogLevel(logLevel *C.s3_log_level_t, value *string) {
	if value == nil {
		return
	}
	switch strings.ToUpper(*value) {
	case "OFF":
		*logLevel = C.Off
	case "FATAL":
		*logLevel = C.Fatal
	case "ERROR":
		*logLevel = C.Error
	case "WARN":
		*logLevel = C.Warn
	case "INFO":
		*logLevel = C.Info
	case "DEBUG":
		*logLevel = C.Debug
	case "TRACE":
		*logLevel = C.Trace
	}
}

func setTLSOptions(tlsName **C.char, tlsConfig *C.as_config_tls, tls *model.TLS) {
	if tls != nil {
		*tlsName = C.CString(*tls.Name)
		tlsConfig.cafile = C.CString(*tls.CAFile)
		tlsConfig.capath = C.CString(*tls.CAPath)
		tlsConfig.protocols = C.CString(*tls.Protocols)
		tlsConfig.cipher_suite = C.CString(*tls.CipherSuite)
		tlsConfig.keyfile = C.CString(*tls.Keyfile)
		tlsConfig.keyfile_pw = C.CString(*tls.KeyfilePassword)
		tlsConfig.certfile = C.CString(*tls.Certfile)
		tlsConfig.cert_blacklist = C.CString(*tls.CertBlacklist)
	}
}
