package tlsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestGetCertificateReloadsOnFileChange(t *testing.T) {
	files := createTestCertificateFiles(t)
	tlsCfg := startReloadingConfig(t, files)
	original := leafSerial(t, servedCertificate(t, tlsCfg))

	replacement := createTestCertificateFiles(t)
	overwriteKeyPair(t, files, replacement)

	require.Eventually(t, func() bool {
		return leafSerial(t, servedCertificate(t, tlsCfg)).Cmp(original) != 0
	}, time.Second, 10*time.Millisecond)
}

func TestGetCertificateKeepsLastGoodOnInvalidPair(t *testing.T) {
	files := createTestCertificateFiles(t)
	tlsCfg := startReloadingConfig(t, files)
	original := leafSerial(t, servedCertificate(t, tlsCfg))

	require.NoError(t, os.WriteFile(files.certFile, []byte("not a certificate"), 0o600))
	bumpFileMtime(t, files.certFile)

	require.Never(t, func() bool {
		return leafSerial(t, servedCertificate(t, tlsCfg)).Cmp(original) != 0
	}, 150*time.Millisecond, 10*time.Millisecond)

	require.NotPanics(t, func() {
		_ = servedCertificate(t, tlsCfg)
	})
}

func TestHTTPSHandshakeServesRotatedCertificate(t *testing.T) {
	files := createTestCertificateFiles(t)
	tlsCfg := startReloadingConfig(t, files)

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ServeTLS(ln, "", "")
	}()
	t.Cleanup(func() {
		_ = srv.Close()
		<-errCh
	})

	addr := ln.Addr().String()
	var original *big.Int
	require.Eventually(t, func() bool {
		serial, err := handshakeSerial(addr)
		if err != nil {
			return false
		}
		original = serial
		return true
	}, time.Second, 10*time.Millisecond)

	replacement := createTestCertificateFiles(t)
	overwriteKeyPair(t, files, replacement)

	var rotated *big.Int
	require.Eventually(t, func() bool {
		serial, err := handshakeSerial(addr)
		if err != nil || serial.Cmp(original) == 0 {
			return false
		}
		rotated = serial
		return true
	}, time.Second, 20*time.Millisecond)

	require.NoError(t, os.WriteFile(files.certFile, []byte("broken"), 0o600))
	bumpFileMtime(t, files.certFile)

	// Fail closed: the broken rewrite must not become the served pair, and
	// handshakes must still succeed with the last good cert (not drop the listener).
	require.Never(t, func() bool {
		serial, err := handshakeSerial(addr)
		return err == nil && serial.Cmp(rotated) != 0
	}, 150*time.Millisecond, 10*time.Millisecond)

	afterBroken, err := handshakeSerial(addr)
	require.NoError(t, err)
	require.Equal(t, 0, afterBroken.Cmp(rotated), "should keep serving the last good cert after a broken rewrite")
}

func TestNoReloadIsANoOp(t *testing.T) {
	reloader := NoReload()
	require.NoError(t, reloader.Load(t.Context()))
	require.NotPanics(t, func() { reloader.Start(t.Context()) })
}

func startReloadingConfig(t *testing.T, files testCertificateFiles) *tls.Config {
	t.Helper()

	config := &model.ServerConfigHTTPS{CertFile: files.certFile, KeyFile: files.keyFile}
	reloader := NewCertificateReloader(config, newTestResolver(t), 20*time.Millisecond)
	require.NoError(t, reloader.Load(t.Context()))

	tlsCfg, err := NewTLSConfig(config, reloader.GetCertificate)
	require.NoError(t, err)
	reloader.Start(t.Context())

	return tlsCfg
}

func overwriteKeyPair(t *testing.T, dest, src testCertificateFiles) {
	t.Helper()

	certPEM, err := os.ReadFile(src.certFile)
	require.NoError(t, err)
	keyPEM, err := os.ReadFile(src.keyFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dest.certFile, certPEM, 0o600)) //nolint:gosec // test tempdir
	require.NoError(t, os.WriteFile(dest.keyFile, keyPEM, 0o600))   //nolint:gosec // test tempdir
	bumpFileMtime(t, dest.certFile)
	bumpFileMtime(t, dest.keyFile)
}

func bumpFileMtime(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(path, info.ModTime().Add(time.Second), info.ModTime().Add(time.Second)))
}

func leafSerial(t *testing.T, cert *tls.Certificate) *big.Int {
	t.Helper()

	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)

	return parsed.SerialNumber
}

func handshakeSerial(addr string) (*big.Int, error) {
	dialer := &tls.Dialer{
		Config: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // test handshake inspects the presented serial
			MinVersion:         tls.VersionTLS12,
		},
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, errors.New("not a TLS connection")
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, errors.New("no peer certificates")
	}

	return certs[0].SerialNumber, nil
}
