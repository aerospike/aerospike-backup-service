package tlsconfig

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testClient struct {
	cert     *x509.Certificate
	certFile string
	keyFile  string
}

type testPKI struct {
	caCert   *x509.Certificate
	caKey    *rsa.PrivateKey
	caFile   string
	certFile string
	keyFile  string
	clients  []testClient
}

func createTestPKI(t *testing.T, clients int) testPKI {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ski := make([]byte, 20)
	_, err = rand.Read(ski)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SubjectKeyId:          ski,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(
		caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600,
	))

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	serverSerial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(
		rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey,
	)
	require.NoError(t, err)
	certFile := filepath.Join(dir, "server.pem")
	keyFile := filepath.Join(dir, "server-key.pem")
	require.NoError(t, os.WriteFile(
		certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), 0o600,
	))
	require.NoError(t, os.WriteFile(
		keyFile,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}),
		0o600,
	))

	pki := testPKI{
		caCert:   caCert,
		caKey:    caKey,
		caFile:   caFile,
		certFile: certFile,
		keyFile:  keyFile,
	}
	for i := range clients {
		pki.clients = append(pki.clients, createPKIClient(t, dir, i, caCert, caKey))
	}

	return pki
}

func createPKIClient(t *testing.T, dir string, i int, caCert *x509.Certificate, caKey *rsa.PrivateKey) testClient {
	t.Helper()

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clientSerial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)
	clientTemplate := &x509.Certificate{
		SerialNumber: clientSerial,
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(
		rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey,
	)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(clientDER)
	require.NoError(t, err)
	certFile := filepath.Join(dir, "client-"+strconv.Itoa(i)+".pem")
	keyFile := filepath.Join(dir, "client-"+strconv.Itoa(i)+"-key.pem")
	require.NoError(t, os.WriteFile(
		certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}), 0o600,
	))
	require.NoError(t, os.WriteFile(
		keyFile,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}),
		0o600,
	))

	return testClient{cert: cert, certFile: certFile, keyFile: keyFile}
}

func (p testPKI) writeCRL(
	t *testing.T,
	path string,
	revoked []*big.Int,
	thisUpdate, nextUpdate time.Time,
	number int64,
	pemEncoded bool,
) {
	t.Helper()

	entries := make([]x509.RevocationListEntry, 0, len(revoked))
	for _, serial := range revoked {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   serial,
			RevocationTime: thisUpdate,
		})
	}
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:                    big.NewInt(number),
		ThisUpdate:                thisUpdate,
		NextUpdate:                nextUpdate,
		RevokedCertificateEntries: entries,
	}, p.caCert, p.caKey)
	require.NoError(t, err)

	data := der
	if pemEncoded {
		data = pem.EncodeToMemory(&pem.Block{Type: pemCRLType, Bytes: der})
	}
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func TestParseCRLs(t *testing.T) {
	pki := createTestPKI(t, 1)
	now := time.Now()
	path := filepath.Join(t.TempDir(), "crl")

	t.Run("DER CRL", func(t *testing.T) {
		pki.writeCRL(t, path, nil, now.Add(-time.Minute), now.Add(time.Hour), 1, false)
		index, err := loadCRLs(path)
		require.NoError(t, err)
		require.Len(t, index.byRawIssuer[string(pki.clients[0].cert.RawIssuer)], 1)
	})

	t.Run("PEM CRL", func(t *testing.T) {
		pki.writeCRL(
			t, path, []*big.Int{pki.clients[0].cert.SerialNumber},
			now.Add(-time.Minute), now.Add(time.Hour), 2, true,
		)
		index, err := loadCRLs(path)
		require.NoError(t, err)
		chosen := index.byRawIssuer[string(pki.clients[0].cert.RawIssuer)][0]
		_, revoked := chosen.revokedSerials[serialKey(pki.clients[0].cert.SerialNumber)]
		assert.True(t, revoked)
	})

	t.Run("PEM bundle", func(t *testing.T) {
		first := filepath.Join(t.TempDir(), "one.pem")
		second := filepath.Join(t.TempDir(), "two.pem")
		pki.writeCRL(t, first, nil, now.Add(-time.Minute), now.Add(time.Hour), 1, true)
		pki.writeCRL(
			t, second, []*big.Int{pki.clients[0].cert.SerialNumber},
			now.Add(-time.Minute), now.Add(time.Hour), 2, true,
		)
		bundle := append(readFile(t, first), readFile(t, second)...)
		require.NoError(t, os.WriteFile(path, bundle, 0o600))
		index, err := loadCRLs(path)
		require.NoError(t, err)
		assert.Len(t, index.byRawIssuer[string(pki.clients[0].cert.RawIssuer)], 2)
	})

	t.Run("empty file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("   \n"), 0o600))
		_, err := loadCRLs(path)
		require.ErrorContains(t, err, "contains no CRLs")
	})

	t.Run("malformed trailing PEM", func(t *testing.T) {
		pki.writeCRL(t, path, nil, now.Add(-time.Minute), now.Add(time.Hour), 1, true)
		partial := append(readFile(t, path), []byte("\n-----BEGIN X509 CRL-----\ntruncated\n")...)
		require.NoError(t, os.WriteFile(path, partial, 0o600))
		_, err := loadCRLs(path)
		require.ErrorContains(t, err, "invalid CRL PEM block")
	})

	t.Run("unsupported PEM block", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("x")}), 0o600))
		_, err := loadCRLs(path)
		require.ErrorContains(t, err, "contains no CRLs")
	})
}

func TestVerifyClientLeaf(t *testing.T) {
	pki := createTestPKI(t, 2)
	now := time.Now()
	path := filepath.Join(t.TempDir(), "crl.pem")
	pki.writeCRL(t, path, []*big.Int{pki.clients[0].cert.SerialNumber}, now.Add(-time.Minute), now.Add(time.Hour), 1, true)
	index, err := loadCRLs(path)
	require.NoError(t, err)

	revokedState := tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{pki.clients[0].cert, pki.caCert}},
	}
	trustedState := tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{pki.clients[1].cert, pki.caCert}},
	}

	require.ErrorIs(t, verifyClientLeaf(revokedState, index, now), errCertificateRevoked)
	require.NoError(t, verifyClientLeaf(trustedState, index, now))
}

func TestVerifyClientLeafFailClosed(t *testing.T) {
	pki := createTestPKI(t, 1)
	now := time.Now()
	leafState := tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{pki.clients[0].cert, pki.caCert}},
	}

	t.Run("expired CRL", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "expired.pem")
		pki.writeCRL(t, path, nil, now.Add(-2*time.Hour), now.Add(-time.Minute), 1, true)
		index, err := loadCRLs(path)
		require.NoError(t, err)
		require.ErrorIs(t, verifyClientLeaf(leafState, index, now), errCRLExpired)
	})

	t.Run("future ThisUpdate", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "future.pem")
		pki.writeCRL(t, path, nil, now.Add(time.Hour), now.Add(2*time.Hour), 1, true)
		index, err := loadCRLs(path)
		require.NoError(t, err)
		require.ErrorIs(t, verifyClientLeaf(leafState, index, now), errCRLNotYetValid)
	})

	t.Run("no matching issuer", func(t *testing.T) {
		other := createTestPKI(t, 0)
		path := filepath.Join(t.TempDir(), "other.pem")
		other.writeCRL(t, path, nil, now.Add(-time.Minute), now.Add(time.Hour), 1, true)
		index, err := loadCRLs(path)
		require.NoError(t, err)
		require.ErrorIs(t, verifyClientLeaf(leafState, index, now), errCRLNotFound)
	})

	t.Run("CRL signed by another CA with the same subject", func(t *testing.T) {
		other := createTestPKI(t, 0)
		path := filepath.Join(t.TempDir(), "other.pem")
		other.writeCRL(t, path, nil, now.Add(-time.Minute), now.Add(time.Hour), 1, true)
		index, err := loadCRLs(path)
		require.NoError(t, err)
		require.Equal(t, string(pki.clients[0].cert.RawIssuer), string(other.caCert.RawSubject))
		require.ErrorIs(t, verifyClientLeaf(leafState, index, now), errCRLNotFound)
	})

	t.Run("no verified chain", func(t *testing.T) {
		require.ErrorIs(t, verifyClientLeaf(tls.ConnectionState{}, &crlIndex{}, now), errNoVerifiedChain)
	})
}

func TestVerifyPrefersHigherCRLNumber(t *testing.T) {
	pki := createTestPKI(t, 1)
	now := time.Now()
	first := filepath.Join(t.TempDir(), "one.pem")
	second := filepath.Join(t.TempDir(), "two.pem")
	pki.writeCRL(
		t, first, []*big.Int{pki.clients[0].cert.SerialNumber},
		now.Add(-time.Minute), now.Add(time.Hour), 1, true,
	)
	pki.writeCRL(t, second, nil, now.Add(-time.Minute), now.Add(time.Hour), 2, true)
	bundle := append(readFile(t, first), readFile(t, second)...)
	path := filepath.Join(t.TempDir(), "bundle.pem")
	require.NoError(t, os.WriteFile(path, bundle, 0o600))
	index, err := loadCRLs(path)
	require.NoError(t, err)

	state := tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{pki.clients[0].cert, pki.caCert}},
	}
	require.NoError(t, verifyClientLeaf(state, index, now))
}

func TestLoadTLSConfigMissingCRLFile(t *testing.T) {
	files := createTestCertificateFiles(t)
	_, err := loadTLSConfig(t, &model.ServerConfigHTTPS{
		CertFile:     files.certFile,
		KeyFile:      files.keyFile,
		ClientCAFile: files.caFile,
		CRLFile:      filepath.Join(t.TempDir(), "missing.crl"),
		ClientAuth:   model.TLSClientAuthRequireAndVerify,
	}, newTestResolver(t))
	require.ErrorContains(t, err, "failed to read client CRL file")
}
