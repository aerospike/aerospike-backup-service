//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

type httpsCertificates struct {
	caFile           string
	certFile         string
	encryptedKeyFile string
}

// TestHTTPSListenerWithSecretAgentKeyPassword starts ABS on HTTPS only. The TLS
// private key is encrypted on disk; Secret Agent stores the passphrase and ABS
// fetches it at listener construction.
func (s *BackupSuite) TestHTTPSListenerWithSecretAgentKeyPassword() {
	certs := s.generateHTTPSCertificates()
	agent := s.startSecretAgent(clientKeyPassword)
	e := s.setupHTTPSEnv(certs, agent)

	s.seedRecords([]int{10, 20, 30})
	s.triggerFullBackup(e)
	fullBackup := s.waitForFullBackup(e)
	s.assertBackupDetails(fullBackup, 3)
}

func (s *Suite) setupHTTPSEnv(certs httpsCertificates, agent *dto.SecretAgent) *env {
	httpsPort := freeHostPort(s)
	backupDir := s.T().TempDir()
	components := s.initComponents(s.baseConfig(backupDir), withHTTPSListener(certs, agent, httpsPort))
	s.Require().Len(components.Servers, 1)

	s.startServers(components.Servers)
	client := s.newHTTPSClient(certs.caFile)
	baseURL := fmt.Sprintf("https://127.0.0.1:%d", httpsPort)
	s.waitForHTTPS(client, baseURL)

	return &env{backupDir: backupDir, baseURL: baseURL, client: client}
}

func withHTTPSListener(certs httpsCertificates, agent *dto.SecretAgent, port int) func(*dto.Config) {
	return func(c *dto.Config) {
		c.ServiceConfig.ServerHTTP = &dto.ServerConfigHTTP{
			ListenerConfig: dto.ListenerConfig{Address: "127.0.0.1", Disabled: true},
		}
		c.ServiceConfig.ServerHTTPS = &dto.ServerConfigHTTPS{
			ListenerConfig:  dto.ListenerConfig{Address: "127.0.0.1"},
			Port:            ptr.Of(dto.Port(port)),
			CertFile:        certs.certFile,
			KeyFile:         certs.encryptedKeyFile,
			KeyFilePassword: decoder.Secret(secretRef()),
			SecretAgentConfig: dto.SecretAgentConfig{
				SecretAgent: agent,
			},
		}
	}
}

func (s *Suite) startServers(servers []server.HTTP) {
	s.T().Helper()

	// Background: t.Context is canceled before t.Cleanup, which would leave the listener up.
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, servers)
	}()
	s.T().Cleanup(func() {
		cancel()
		if err := <-errCh; err != nil {
			s.T().Errorf("HTTPS server stopped with error: %v", err)
		}
	})
}

func (s *Suite) newHTTPSClient(caFile string) *http.Client {
	s.T().Helper()

	caPEM, err := os.ReadFile(caFile)
	s.Require().NoError(err)
	roots := x509.NewCertPool()
	s.Require().True(roots.AppendCertsFromPEM(caPEM))

	return &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    roots,
				MinVersion: tls.VersionTLS12,
			},
		},
	}
}

func (s *Suite) waitForHTTPS(client *http.Client, baseURL string) {
	s.T().Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(s.T().Context(), http.MethodGet, baseURL+"/health", nil)
		s.Require().NoError(err)

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	s.Failf("timed out waiting for HTTPS listener", "url %s", baseURL)
}

func (s *Suite) generateHTTPSCertificates() httpsCertificates {
	s.T().Helper()

	dir := s.T().TempDir()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ABS HTTPS test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	s.Require().NoError(err)

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(
		rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey,
	)
	s.Require().NoError(err)

	caFile := filepath.Join(dir, "ca.pem")
	certFile := filepath.Join(dir, "server.pem")
	keyFile := filepath.Join(dir, "server-key.pem")
	s.Require().NoError(os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))
	s.Require().NoError(os.WriteFile(
		certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), 0o600,
	))
	s.Require().NoError(os.WriteFile(
		keyFile,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}),
		0o600,
	))

	return httpsCertificates{
		caFile:           caFile,
		certFile:         certFile,
		encryptedKeyFile: s.encryptPEMKeyFile(dir, keyFile, clientKeyPassword),
	}
}

func (s *Suite) encryptPEMKeyFile(dir, keyFile, password string) string {
	s.T().Helper()

	keyPEM, err := os.ReadFile(keyFile)
	s.Require().NoError(err)
	block, _ := pem.Decode(keyPEM)
	s.Require().NotNil(block)

	encrypted, err := x509.EncryptPEMBlock( //nolint:staticcheck // DEK-Info PEM is what ABS decrypts
		rand.Reader, block.Type, block.Bytes, []byte(password), x509.PEMCipherAES256)
	s.Require().NoError(err)

	path := filepath.Join(dir, "server-key.enc")
	s.Require().NoError(os.WriteFile(path, pem.EncodeToMemory(encrypted), 0o600)) //nolint:gosec // test tempdir

	return path
}

func freeHostPort(s *Suite) int {
	s.T().Helper()

	listener, err := (&net.ListenConfig{}).Listen(s.T().Context(), "tcp", "127.0.0.1:0")
	s.Require().NoError(err)
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}
