package aerospike

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

var cipherSuiteMap = func() map[string]uint16 {
	m := make(map[string]uint16)
	for _, suite := range tls.CipherSuites() {
		m[suite.Name] = suite.ID
	}
	for _, suite := range tls.InsecureCipherSuites() {
		m[suite.Name] = suite.ID
	}
	return m
}()

// NewTLSConfig creates a tls.Config from the provided model TLS struct.
func NewTLSConfig(t *model.TLS) (*tls.Config, error) {
	if t == nil {
		return nil, nil // If no TLS config is provided, return nil.
	}

	// Create the CA certificate pool.
	rootCAs, err := loadCertPool(t.CAFile, t.CAPath)
	if err != nil {
		return nil, err
	}

	// Load client certificates.
	clientCerts, err := loadClientCerts(t)
	if err != nil {
		return nil, err
	}

	// Parse cipher suites.
	cipherSuites, err := parseCipherSuites(t.CipherSuite)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{ //nolint:gosec
		ServerName:   util.ValueOrZero(t.Name),
		Certificates: clientCerts,
		RootCAs:      rootCAs,
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS12,
		CipherSuites: cipherSuites,
	}

	return tlsConfig, nil
}

// loadCertPool creates a new x509.CertPool and populates it from a file and a directory.
func loadCertPool(caFile, caPath *string) (*x509.CertPool, error) {
	// Try to load system CA certs, otherwise just make an empty pool
	pool, err := x509.SystemCertPool()
	if pool == nil || err != nil {
		slog.Warn("Failed to load system CA certificates", "err", err)
		pool = x509.NewCertPool()
	}

	// Load from CAFile if provided.
	if caFile != nil && *caFile != "" {
		if err := appendCertFile(pool, *caFile); err != nil {
			return nil, err
		}
	}

	// Load from CAPath if provided.
	if caPath != nil && *caPath != "" {
		files, err := os.ReadDir(*caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA path directory %s: %w", *caPath, err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if err := appendCertFile(pool, filepath.Join(*caPath, file.Name())); err != nil {
				return nil, err
			}
		}
	}

	return pool, nil
}

// appendCertFile reads a PEM file and appends its certificates to the given CertPool.
func appendCertFile(pool *x509.CertPool, path string) error {
	pemBytes, err := readFromFile(path)
	if err != nil {
		return err
	}

	if !pool.AppendCertsFromPEM(pemBytes) {
		return fmt.Errorf("failed to append certificates from CA file %s", path)
	}

	return nil
}

// loadClientCerts loads the client certificate and key.
func loadClientCerts(t *model.TLS) ([]tls.Certificate, error) {
	certFile := util.ValueOrZero(t.Certfile)
	keyFile := util.ValueOrZero(t.Keyfile)

	if certFile == "" || keyFile == "" {
		return nil, nil // Not an error, just no client certs provided.
	}

	// Read cert file
	certFileBytes, err := readFromFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read client certificate file %s: %w", certFile, err)
	}

	// Read and potentially decrypt key
	keyFileBytes, err := readFromFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read client key file %s: %w", keyFile, err)
	}

	// Try to decode and decrypt PEM if password is set
	keyBlock, _ := pem.Decode(keyFileBytes)
	if keyBlock == nil {
		return nil, errors.New("failed to decode PEM block in client key file")
	}

	// Check and Decrypt the Key Block using passphrase
	if t.KeyfilePassword != nil && x509.IsEncryptedPEMBlock(keyBlock) { //nolint:staticcheck
		decryptedDERBytes, err := x509.DecryptPEMBlock(keyBlock, []byte(*t.KeyfilePassword)) //nolint:staticcheck
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt client key file %s: %w", keyFile, err)
		}
		keyBlock.Bytes = decryptedDERBytes
		keyBlock.Headers = nil
		keyFileBytes = pem.EncodeToMemory(keyBlock)
	}

	// Use full cert PEM + decrypted (or raw) key PEM
	cert, err := tls.X509KeyPair(certFileBytes, keyFileBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509 key pair from cert and key files: %w", err)
	}

	return []tls.Certificate{cert}, nil
}

// parseCipherSuites parses a colon-separated string of IANA cipher suite names.
func parseCipherSuites(cipherSuiteStr *string) ([]uint16, error) {
	if cipherSuiteStr == nil || *cipherSuiteStr == "" {
		return nil, nil // Use default cipher suites.
	}

	names := strings.Split(*cipherSuiteStr, ":")
	suites := make([]uint16, 0, len(names))

	for _, name := range names {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		id, ok := cipherSuiteMap[trimmedName]
		if !ok {
			return nil, fmt.Errorf("unsupported or unknown cipher suite: %s", trimmedName)
		}
		suites = append(suites, id)
	}

	if len(suites) == 0 {
		return nil, fmt.Errorf("cipher suite string was provided but contained no valid ciphers")
	}

	return suites, nil
}

func readFromFile(filePath string) ([]byte, error) {
	dataBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read from file %s: %w", filePath, err)
	}
	data := bytes.TrimSuffix(dataBytes, []byte("\n"))

	return data, nil
}
