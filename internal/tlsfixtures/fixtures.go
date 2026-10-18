// Package tlsfixtures embeds TLS material written by OpenSSL 3, so the loaders are tested
// against the files operators produce rather than against Go-generated PEM. generate.sh
// recreates testdata; Password protects every encrypted key in it.
package tlsfixtures

import (
	"embed"
	"os"
	"path"
	"path/filepath"
	"testing"
)

//go:embed testdata
var files embed.FS

// Password protects every encrypted key in testdata.
const Password = "fixture-password"

// KeyPair is a self-signed server certificate with its private key in one of the encodings
// OpenSSL writes. Password is empty when the key is not encrypted.
type KeyPair struct {
	Name     string
	CertFile string
	KeyFile  string
	Password string
}

// KeyPairs lists every server key encoding in testdata.
func KeyPairs() []KeyPair {
	const (
		rsaCert     = "rsa-cert.pem"
		ecCert      = "ec-cert.pem"
		ed25519Cert = "ed25519-cert.pem"
	)

	return []KeyPair{
		{Name: "rsa PKCS#1", CertFile: rsaCert, KeyFile: "rsa-key-pkcs1.pem"},
		{Name: "rsa PKCS#1 legacy AES-256", CertFile: rsaCert, KeyFile: "rsa-key-pkcs1-aes256.pem", Password: Password},
		{Name: "rsa PKCS#8", CertFile: rsaCert, KeyFile: "rsa-key-pkcs8.pem"},
		{Name: "rsa PKCS#8 AES-256 PBKDF2", CertFile: rsaCert, KeyFile: "rsa-key-pkcs8-aes256.pem", Password: Password},
		{Name: "rsa PKCS#8 AES-256 scrypt", CertFile: rsaCert, KeyFile: "rsa-key-pkcs8-scrypt.pem", Password: Password},
		{Name: "ec SEC1", CertFile: ecCert, KeyFile: "ec-key-sec1.pem"},
		{Name: "ec SEC1 legacy AES-256", CertFile: ecCert, KeyFile: "ec-key-sec1-aes256.pem", Password: Password},
		{Name: "ec PKCS#8", CertFile: ecCert, KeyFile: "ec-key-pkcs8.pem"},
		{Name: "ec PKCS#8 AES-256 PBKDF2", CertFile: ecCert, KeyFile: "ec-key-pkcs8-aes256.pem", Password: Password},
		{Name: "ec PKCS#8 AES-256 scrypt", CertFile: ecCert, KeyFile: "ec-key-pkcs8-scrypt.pem", Password: Password},
		{Name: "ed25519 PKCS#8", CertFile: ed25519Cert, KeyFile: "ed25519-key-pkcs8.pem"},
		{
			Name: "ed25519 PKCS#8 AES-256 PBKDF2", CertFile: ed25519Cert,
			KeyFile: "ed25519-key-pkcs8-aes256.pem", Password: Password,
		},
		{
			Name: "ed25519 PKCS#8 AES-256 scrypt", CertFile: ed25519Cert,
			KeyFile: "ed25519-key-pkcs8-scrypt.pem", Password: Password,
		},
	}
}

// Dir copies testdata into a temporary directory and returns its path. The loaders take
// file paths, and the copy keeps a test that rewrites a fixture away from the source tree.
func Dir(tb testing.TB) string {
	tb.Helper()

	dir := tb.TempDir()
	entries, err := files.ReadDir("testdata")
	if err != nil {
		tb.Fatalf("list fixtures: %v", err)
	}
	for _, entry := range entries {
		data, err := files.ReadFile(path.Join("testdata", entry.Name()))
		if err != nil {
			tb.Fatalf("read fixture %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o600); err != nil {
			tb.Fatalf("write fixture %s: %v", entry.Name(), err)
		}
	}

	return dir
}
