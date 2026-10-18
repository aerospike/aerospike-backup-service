package tlsconfig

import (
	"crypto/x509"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/internal/tlsfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests read files written by OpenSSL 3 (see internal/tlsfixtures) instead of PEM
// generated in Go, so every encoding an operator can hand the listener is covered.

func TestLoadKeyPairOpenSSLEncodings(t *testing.T) {
	dir := tlsfixtures.Dir(t)

	for _, pair := range tlsfixtures.KeyPairs() {
		t.Run(pair.Name, func(t *testing.T) {
			certificate, err := loadKeyPair(
				filepath.Join(dir, pair.CertFile), filepath.Join(dir, pair.KeyFile), pair.Password,
			)
			require.NoError(t, err)
			assert.NotNil(t, certificate.PrivateKey)
		})
	}

	t.Run("wrong password for a PKCS#8 key", func(t *testing.T) {
		_, err := loadKeyPair(
			filepath.Join(dir, "rsa-cert.pem"), filepath.Join(dir, "rsa-key-pkcs8-aes256.pem"), "wrong",
		)
		require.ErrorContains(t, err, "failed to decrypt HTTPS private key")
		require.ErrorContains(t, err, "incorrect password")
	})

	t.Run("no password for a PKCS#8 key", func(t *testing.T) {
		_, err := loadKeyPair(
			filepath.Join(dir, "rsa-cert.pem"), filepath.Join(dir, "rsa-key-pkcs8-aes256.pem"), "",
		)
		require.ErrorContains(t, err, "no key-file-password is set")
	})
}

func TestLoadClientCAsOpenSSLEncodings(t *testing.T) {
	dir := tlsfixtures.Dir(t)

	tests := []struct {
		name string
		file string
		cas  []string
	}{
		{name: "plain PEM", file: "ca.pem", cas: []string{"ca.pem"}},
		{name: "x509 -text output", file: "ca-text.pem", cas: []string{"ca.pem"}},
		{name: "bundle of x509 -text outputs", file: "ca-bundle-text.pem", cas: []string{"ca.pem", "ca2.pem"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := loadClientCAs(filepath.Join(dir, tt.file))
			require.NoError(t, err)

			want := x509.NewCertPool()
			for _, ca := range tt.cas {
				require.True(t, want.AppendCertsFromPEM(readFile(t, filepath.Join(dir, ca))))
			}
			assert.True(t, pool.Equal(want))
		})
	}
}

func TestLoadCRLsOpenSSLEncodings(t *testing.T) {
	dir := tlsfixtures.Dir(t)

	tests := []struct {
		name    string
		file    string
		issuers int
		revoked int
	}{
		{name: "PEM", file: "crl.pem", issuers: 1, revoked: 1},
		{name: "DER", file: "crl.der", issuers: 1, revoked: 1},
		{name: "crl -text output", file: "crl-text.pem", issuers: 1, revoked: 1},
		{name: "bundle of crl -text outputs", file: "crl-bundle-text.pem", issuers: 2, revoked: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, err := loadCRLs(filepath.Join(dir, tt.file))
			require.NoError(t, err)
			assert.Len(t, index.byRawIssuer, tt.issuers)

			revoked := 0
			for _, lists := range index.byRawIssuer {
				for _, list := range lists {
					revoked += len(list.revokedSerials)
				}
			}
			assert.Equal(t, tt.revoked, revoked)
		})
	}
}
