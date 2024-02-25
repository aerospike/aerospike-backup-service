//go:build !ci

package shared

/*
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>

#include <file_proxy.h>
#include <io_proxy.h>
#include <utils.h>
#include <aerospike/as_config.h>

bool is_null(void* pointer) {
  if (NULL == pointer) {
    return true;
  }
  return false;
}

encryption_key_t* enc_key_malloc() {
	return (encryption_key_t*) cf_malloc(sizeof(encryption_key_t));
}
*/
import "C"
import (
	"log/slog"
	"strings"
	"unsafe"

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
		tlsConfig.enable = true
		setCString(tlsName, tls.Name)
		setCString(&tlsConfig.cafile, tls.CAFile)
		setCString(&tlsConfig.capath, tls.CAPath)
		setCString(&tlsConfig.protocols, tls.Protocols)
		setCString(&tlsConfig.cipher_suite, tls.CipherSuite)
		setCString(&tlsConfig.keyfile, tls.Keyfile)
		setCString(&tlsConfig.keyfile_pw, tls.KeyfilePassword)
		setCString(&tlsConfig.certfile, tls.Certfile)
	}
}

//nolint:gocritic
func configureEncryption(encryptMode *C.encryption_opt, pKey **C.encryption_key_t,
	policy *model.EncryptionPolicy) {
	if policy != nil {
		switch policy.Mode {
		case model.EncryptNone:
			*encryptMode = C.IO_PROXY_ENCRYPT_NONE
		case model.EncryptAES128:
			*encryptMode = C.IO_PROXY_ENCRYPT_AES128
		case model.EncryptAES256:
			*encryptMode = C.IO_PROXY_ENCRYPT_AES256
		}
		if policy.KeyFile != nil {
			*pKey = C.enc_key_malloc()
			if res := C.read_private_key_file(C.CString(*policy.KeyFile), *pKey); res != 0 {
				slog.Error("Failed to read encryption key", "file", *policy.KeyFile)
			}
		} else if policy.KeyEnv != nil {
			*pKey = C.parse_encryption_key_env(C.CString(*policy.KeyEnv))
			if C.is_null(unsafe.Pointer(*pKey)) {
				slog.Error("Failed to read encryption key", "env", *policy.KeyEnv)
			}
		} else if policy.KeySecret != nil {
			*pKey = C.enc_key_malloc()
			if res := C.read_private_key(C.CString(*policy.KeySecret), *pKey); res != 0 {
				slog.Error("Failed to read encryption key", "secret", *policy.KeySecret)
			}
		}
	}
}

func configureCompression(mode *C.compression_opt, level *C.int32_t, policy *model.CompressionPolicy) {
	if policy != nil {
		switch policy.Mode {
		case model.CompressNone:
			*mode = C.IO_PROXY_COMPRESS_NONE
		case model.CompressZSTD:
			*mode = C.IO_PROXY_COMPRESS_ZSTD
		}
		if level != nil {
			*level = C.int(policy.Level)
		}
	}
}
