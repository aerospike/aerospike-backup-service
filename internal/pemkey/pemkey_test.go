package pemkey

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/youmark/pkcs8"
)

const password = "password"

func TestDecrypt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	plainPKCS8 := pem.EncodeToMemory(&pem.Block{Type: pkcs8Type, Bytes: pkcs8DER})
	encryptedPKCS8 := encryptPKCS8(t, key)
	legacy := encryptLegacy(t, key)

	decrypted := []struct {
		name     string
		data     []byte
		password string
		wantType string
	}{
		{name: "PKCS#8 encrypted", data: encryptedPKCS8, password: password, wantType: pkcs8Type},
		{name: "legacy encrypted", data: legacy, password: password, wantType: "RSA PRIVATE KEY"},
		{
			name:     "explanatory text before the block",
			data:     slices.Concat([]byte("Private-Key: (2048 bit)\n"), encryptedPKCS8),
			password: password,
			wantType: pkcs8Type,
		},
	}
	for _, tt := range decrypted {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(tt.data, tt.password)
			require.NoError(t, err)
			block, _ := pem.Decode(got)
			require.NotNil(t, block)
			assert.Equal(t, tt.wantType, block.Type)
			assert.True(t, key.Equal(parseKey(t, block)))
		})
	}

	unchanged := []struct {
		name     string
		data     []byte
		password string
	}{
		{name: "plain PKCS#8 with a password", data: plainPKCS8, password: "unused"},
		{name: "plain PKCS#1 with a password", data: pkcs1, password: "unused"},
		{name: "plain PKCS#1 without a password", data: pkcs1},
		{name: "no PEM block", data: []byte("not a key"), password: password},
	}
	for _, tt := range unchanged {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(tt.data, tt.password)
			require.NoError(t, err)
			assert.Equal(t, tt.data, got)
		})
	}

	failures := []struct {
		name     string
		data     []byte
		password string
		wantErr  string
	}{
		{name: "PKCS#8 wrong password", data: encryptedPKCS8, password: "wrong", wantErr: "incorrect password"},
		{name: "legacy wrong password", data: legacy, password: "wrong", wantErr: "decryption password incorrect"},
		{name: "PKCS#8 without a password", data: encryptedPKCS8, wantErr: "no key-file-password is set"},
		{name: "legacy without a password", data: legacy, wantErr: "no key-file-password is set"},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.data, tt.password)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}

	t.Run("blocks after the key are kept", func(t *testing.T) {
		certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not checked")})
		got, err := Decrypt(slices.Concat(encryptedPKCS8, certificate), password)
		require.NoError(t, err)
		_, rest := pem.Decode(got)
		assert.Equal(t, certificate, rest)
	})
}

func encryptPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()

	der, err := pkcs8.MarshalPrivateKey(key, []byte(password), &pkcs8.Opts{
		Cipher:  pkcs8.AES256CBC,
		KDFOpts: pkcs8.PBKDF2Opts{SaltSize: 8, IterationCount: 2048, HMACHash: crypto.SHA256},
	})
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: encryptedPKCS8Type, Bytes: der})
}

func encryptLegacy(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()

	//nolint:staticcheck // Legacy PEM encryption is what this package must keep reading.
	//noinspection GoDeprecation
	block, err := x509.EncryptPEMBlock(
		rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), []byte(password), x509.PEMCipherAES256,
	)
	require.NoError(t, err)

	return pem.EncodeToMemory(block)
}

func parseKey(t *testing.T, block *pem.Block) crypto.PrivateKey {
	t.Helper()

	if block.Type == pkcs8Type {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		require.NoError(t, err)

		return key
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	return key
}
