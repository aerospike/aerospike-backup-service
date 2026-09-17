// Package pemkey prepares PEM private keys for crypto/tls, decrypting password-protected ones.
package pemkey

import (
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/youmark/pkcs8"
)

const (
	encryptedPKCS8Type = "ENCRYPTED PRIVATE KEY"
	pkcs8Type          = "PRIVATE KEY"
)

var errNoPassword = errors.New("private key is encrypted but no key-file-password is set")

// Decrypt returns keyPEM in the form crypto/tls key-pair loading accepts. When the first
// PEM block is a password-protected private key, in PKCS#8 (RFC 5958, what OpenSSL 3
// writes by default) or in legacy RFC 1423 PEM encryption, it is decrypted with password;
// an encrypted key without a password is an error that names the setting. A plain key, or
// data without a PEM block, is returned unchanged for crypto/tls to load or report, so a
// password configured for a plain key is not an error.
func Decrypt(keyPEM []byte, password string) ([]byte, error) {
	block, rest := pem.Decode(keyPEM)
	if block == nil {
		return keyPEM, nil
	}

	//nolint:staticcheck // Legacy RFC 1423 PEM encryption stays supported for keys already in service.
	//noinspection GoDeprecation
	legacy := x509.IsEncryptedPEMBlock(block)
	if block.Type != encryptedPKCS8Type && !legacy {
		return keyPEM, nil
	}
	if password == "" {
		return nil, errNoPassword
	}

	var (
		decrypted *pem.Block
		err       error
	)
	if legacy {
		decrypted, err = decryptLegacy(block, password)
	} else {
		decrypted, err = decryptPKCS8(block.Bytes, password)
	}
	if err != nil {
		return nil, err
	}

	return append(pem.EncodeToMemory(decrypted), rest...), nil
}

func decryptPKCS8(der []byte, password string) (*pem.Block, error) {
	key, err := pkcs8.ParsePKCS8PrivateKey(der, []byte(password))
	if err != nil {
		return nil, err
	}
	plain, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}

	return &pem.Block{Type: pkcs8Type, Bytes: plain}, nil
}

func decryptLegacy(block *pem.Block, password string) (*pem.Block, error) {
	//nolint:staticcheck // Legacy RFC 1423 PEM encryption stays supported for keys already in service.
	//noinspection GoDeprecation
	plain, err := x509.DecryptPEMBlock(block, []byte(password))
	if err != nil {
		return nil, err
	}

	return &pem.Block{Type: block.Type, Bytes: plain}, nil
}
