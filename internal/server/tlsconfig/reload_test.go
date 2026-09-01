package tlsconfig

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
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

func TestGetCertificateReloadsOnKeyFileChange(t *testing.T) {
	files := createTestCertificateFiles(t)
	tlsCfg := startReloadingConfig(t, files)
	original := leafSerial(t, servedCertificate(t, tlsCfg))
	certInfo, err := os.Stat(files.certFile)
	require.NoError(t, err)

	replacement := createTestCertificateFiles(t)
	overwriteKeyPair(t, files, replacement)
	require.NoError(t, os.Chtimes(files.certFile, certInfo.ModTime(), certInfo.ModTime()))
	bumpFileMtime(t, files.keyFile)

	require.Eventually(t, func() bool {
		return leafSerial(t, servedCertificate(t, tlsCfg)).Cmp(original) != 0
	}, time.Second, 10*time.Millisecond)
}

func TestGetCertificateKeepsLastGoodOnInvalidKeyFile(t *testing.T) {
	files := createTestCertificateFiles(t)
	tlsCfg := startReloadingConfig(t, files)
	original := leafSerial(t, servedCertificate(t, tlsCfg))

	require.NoError(t, os.WriteFile(files.keyFile, []byte("not a private key"), 0o600))
	bumpFileMtime(t, files.keyFile)

	require.Never(t, func() bool {
		return leafSerial(t, servedCertificate(t, tlsCfg)).Cmp(original) != 0
	}, 150*time.Millisecond, 10*time.Millisecond)
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

	addr := startHTTPSServer(t, tlsCfg)
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

func TestClientCAReloadAcceptsNewClientAndRejectsOld(t *testing.T) {
	files := createTestCertificateFiles(t)
	tlsCfg := startReloadingMTLSConfig(t, files)

	addr := startHTTPSServer(t, tlsCfg)
	require.NoError(t, mtlsGet(addr, files.clientCertFile, files.clientKeyFile, files.caFile))

	replacement := createTestCertificateFiles(t)
	overlapBundle := append(readFile(t, files.caFile), readFile(t, replacement.caFile)...)
	require.NoError(t, os.WriteFile(files.caFile, overlapBundle, 0o600))
	bumpFileMtime(t, files.caFile)

	require.Eventually(t, func() bool {
		return mtlsGet(addr, replacement.clientCertFile, replacement.clientKeyFile, replacement.caFile) == nil
	}, time.Second, 20*time.Millisecond)
	require.NoError(t, mtlsGet(addr, files.clientCertFile, files.clientKeyFile, files.caFile))

	require.NoError(t, os.WriteFile(files.caFile, readFile(t, replacement.caFile), 0o600))
	bumpFileMtime(t, files.caFile)
	require.Eventually(t, func() bool {
		return mtlsGet(addr, files.clientCertFile, files.clientKeyFile, files.caFile) != nil
	}, time.Second, 20*time.Millisecond)
	require.NoError(t, mtlsGet(addr, replacement.clientCertFile, replacement.clientKeyFile, replacement.caFile))
}

func TestClientCAKeepsLastGoodOnInvalidFileAndRecovers(t *testing.T) {
	logs := &lockedLogBuffer{}
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	files := createTestCertificateFiles(t)
	tlsCfg := startReloadingMTLSConfig(t, files)

	addr := startHTTPSServer(t, tlsCfg)
	require.NoError(t, mtlsGet(addr, files.clientCertFile, files.clientKeyFile, files.caFile))

	replacement := createTestCertificateFiles(t)
	partiallyInvalidBundle := append(
		readFile(t, replacement.caFile),
		[]byte("\n-----BEGIN CERTIFICATE-----\ntruncated\n")...,
	)
	require.NoError(t, os.WriteFile(files.caFile, partiallyInvalidBundle, 0o600))
	bumpFileMtime(t, files.caFile)

	require.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "level=ERROR") &&
			strings.Contains(logs.String(), "file change callback failed")
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, mtlsGet(addr, files.clientCertFile, files.clientKeyFile, files.caFile))
	require.Error(t, mtlsGet(addr, replacement.clientCertFile, replacement.clientKeyFile, replacement.caFile))

	require.NoError(t, os.WriteFile(files.caFile, readFile(t, replacement.caFile), 0o600))
	bumpFileMtime(t, files.caFile)

	require.Eventually(t, func() bool {
		return mtlsGet(addr, replacement.clientCertFile, replacement.clientKeyFile, replacement.caFile) == nil
	}, time.Second, 20*time.Millisecond)
	require.Error(t, mtlsGet(addr, files.clientCertFile, files.clientKeyFile, files.caFile))
}

func startReloadingConfig(t *testing.T, files testCertificateFiles) *tls.Config {
	t.Helper()

	config := &model.ServerConfigHTTPS{CertFile: files.certFile, KeyFile: files.keyFile}
	return startReloadingConfigWithModel(t, config)
}

func startReloadingMTLSConfig(t *testing.T, files testCertificateFiles) *tls.Config {
	t.Helper()

	config := &model.ServerConfigHTTPS{
		CertFile:     files.certFile,
		KeyFile:      files.keyFile,
		ClientCAFile: files.caFile,
		ClientAuth:   model.TLSClientAuthRequireAndVerify,
	}
	return startReloadingConfigWithModel(t, config)
}

func startReloadingConfigWithModel(t *testing.T, config *model.ServerConfigHTTPS) *tls.Config {
	t.Helper()

	reloader := NewCertificateReloader(config, newTestResolver(t), 20*time.Millisecond)
	require.NoError(t, reloader.Load(t.Context()))

	tlsCfg, err := NewTLSConfig(config, reloader)
	require.NoError(t, err)
	reloader.Start(t.Context())

	return tlsCfg
}

func startHTTPSServer(t *testing.T, tlsCfg *tls.Config) string {
	t.Helper()

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

	return ln.Addr().String()
}

func mtlsGet(addr, clientCertFile, clientKeyFile, _ string) error {
	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test verifies client certificate acceptance only
				Certificates:       []tls.Certificate{clientCert},
				MinVersion:         tls.VersionTLS12,
			},
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+addr, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
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

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
