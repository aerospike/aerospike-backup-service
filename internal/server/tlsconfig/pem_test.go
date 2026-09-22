package tlsconfig

import (
	"bytes"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opensslCertificateText is the human-readable dump that `openssl x509 -text` writes in
// front of the PEM block, abridged.
const opensslCertificateText = `Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            30:86:16:3f:44:92:fc:44:77:8f:5d:fe:1b:0a:70:3d:66:9e:7a:33
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=test CA
        Validity
            Not Before: Sep 17 09:03:12 2026 GMT
            Not After : Sep 18 09:03:12 2026 GMT
        Subject: CN=test CA
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (2048 bit)
                Modulus:
                    00:c6:1d:2f:5b:a3:7e:91:04:d8:6c:ee:12:7a:b0:35:
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Basic Constraints: critical
                CA:TRUE
    Signature Algorithm: sha256WithRSAEncryption
    Signature Value:
        4f:cc:d4:75:e8:9e:c9:a2:e4:6a:d4:ed:01:4d:90:c0:47:ca:
        c3:3e:e7:05
`

// opensslCRLText is the human-readable dump that `openssl crl -text` writes in front of
// the PEM block.
const opensslCRLText = `Certificate Revocation List (CRL):
    Version 2 (0x1)
    Signature Algorithm: sha256WithRSAEncryption
    Issuer: CN=test CA
    Last Update: Sep 17 09:03:12 2026 GMT
    Next Update: Sep 18 09:03:12 2026 GMT
    CRL extensions:
        X509v3 CRL Number:
            1
No Revoked Certificates.
    Signature Algorithm: sha256WithRSAEncryption
    Signature Value:
        4f:cc:d4:75:e8:9e:c9:a2:e4:6a:d4:ed:01:4d:90:c0:47:ca:
        c3:3e:e7:05
`

func TestDecodePEMBlocks(t *testing.T) {
	first := pem.EncodeToMemory(&pem.Block{Type: pemCertificateType, Bytes: []byte("first")})
	second := pem.EncodeToMemory(&pem.Block{Type: pemCertificateType, Bytes: []byte("second")})
	key := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("key")})
	text := []byte(opensslCertificateText)
	corrupt := []byte("-----BEGIN CERTIFICATE-----\n!!!!\n-----END CERTIFICATE-----\n")
	truncated := []byte("-----BEGIN CERTIFICATE-----\ntruncated\n")

	tests := []struct {
		name    string
		data    []byte
		want    [][]byte
		wantErr string
	}{
		{name: "single block", data: first, want: [][]byte{[]byte("first")}},
		{name: "bundle", data: concat(first, second), want: [][]byte{[]byte("first"), []byte("second")}},
		{name: "explanatory text before the block", data: concat(text, first), want: [][]byte{[]byte("first")}},
		{
			name: "explanatory text before every block of a bundle",
			data: concat(text, first, text, second),
			want: [][]byte{[]byte("first"), []byte("second")},
		},
		{
			name: "explanatory text after the last block",
			data: concat(first, []byte("# end of bundle\n")),
			want: [][]byte{[]byte("first")},
		},
		{
			name: "CRLF line endings",
			data: bytes.ReplaceAll(concat(text, first), []byte("\n"), []byte("\r\n")),
			want: [][]byte{[]byte("first")},
		},
		{name: "no block", data: []byte("not a certificate\n")},
		{name: "empty"},
		{
			name:    "block of another type",
			data:    concat(first, key),
			wantErr: `unexpected "RSA PRIVATE KEY" PEM block, want "CERTIFICATE"`,
		},
		{name: "truncated last block", data: concat(first, truncated), wantErr: "malformed PEM block"},
		{name: "only a truncated block", data: truncated, wantErr: "malformed PEM block"},
		{name: "corrupt block between valid blocks", data: concat(first, corrupt, second), wantErr: "malformed PEM block"},
		{name: "unterminated block before a valid block", data: concat(truncated, first), wantErr: "malformed PEM block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePEMBlocks(tt.data, pemCertificateType)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func concat(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

// FuzzDecodePEMBlocks checks that blocks separated by arbitrary explanatory text decode
// back to exactly those blocks, whatever the text contains.
func FuzzDecodePEMBlocks(f *testing.F) {
	f.Add(opensslCertificateText, "", "", []byte("first"), []byte("second"))
	f.Add("", "# between\n", "# after", []byte{}, []byte("x"))
	f.Add("\n-----END CERTIFICATE-----\n", "text -----BEGIN CERTIFICATE----- mid-line\n", "", []byte("a"), []byte("b"))
	f.Fuzz(func(t *testing.T, before, between, after string, first, second []byte) {
		texts := []string{before, between, after}
		for i, text := range texts {
			if countBoundaries([]byte(text)) > 0 {
				t.Skip("text opens a PEM block")
			}
			// An encapsulation boundary is a line of its own.
			if text != "" && !strings.HasSuffix(text, "\n") {
				texts[i] = text + "\n"
			}
		}
		data := concat(
			[]byte(texts[0]), encodeCertificate(first),
			[]byte(texts[1]), encodeCertificate(second),
			[]byte(texts[2]),
		)

		got, err := decodePEMBlocks(data, pemCertificateType)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.True(t, bytes.Equal(first, got[0]))
		assert.True(t, bytes.Equal(second, got[1]))
	})
}

func encodeCertificate(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: pemCertificateType, Bytes: der})
}
