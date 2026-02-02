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

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

var protocolMap = map[string]uint16{
	"TLSv1.2": tls.VersionTLS12,
	// "TLSv1.3": tls.VersionTLS13, //uncomment when server supports 1.3
}

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

	// Parse protocol versions.
	minVersion, maxVersion, err := parseProtocols(t.Protocols)
	if err != nil {
		return nil, err
	}

	// Parse cipher suites.
	cipherSuites, err := parseCipherSuites(t.CipherSuite)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{ //nolint:gosec
		ServerName:   ptr.ValueOrZero(t.Name),
		Certificates: clientCerts,
		RootCAs:      rootCAs,
		MinVersion:   minVersion,
		MaxVersion:   maxVersion,
		CipherSuites: cipherSuites,
	}

	return tlsConfig, nil
}

// loadCertPool creates a new x509.CertPool and populates it from a file and a directory.
func loadCertPool(caFile, caPath *string) (*x509.CertPool, error) {
	// Try to load system CA certs, otherwise just make an empty pool
	pool, err := x509.SystemCertPool()
	if pool == nil || err != nil {
		slog.Warn("Failed to load system CA certificates", attr.Error(err))
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
			path := filepath.Join(*caPath, file.Name())
			fileInfo, err := os.Stat(path)
			if err != nil {
				slog.Warn("Failed to stat file, skipping",
					slog.String("path", path), attr.Error(err))
				continue
			}
			if fileInfo.IsDir() {
				continue
			}
			if err := appendCertFile(pool, path); err != nil {
				slog.Warn("Failed to append certificate file, skipping",
					slog.String("path", path), attr.Error(err))
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
	certFile := ptr.ValueOrZero(t.Certfile)
	keyFile := ptr.ValueOrZero(t.Keyfile)

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
	//noinspection GoDeprecation
	if t.KeyfilePassword != nil && x509.IsEncryptedPEMBlock(keyBlock) {
		decryptedDERBytes, err := x509.DecryptPEMBlock(keyBlock, []byte(*t.KeyfilePassword))
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

// parseProtocols parses a space-separated string of TLS protocol versions.
func parseProtocols(protocols *string) (minVersion, maxVersion uint16, err error) {
	// Default to TLS 1.2 as the minimum if nothing is specified.
	// A maxVersion of 0 means "use the highest supported version".
	minVersion, maxVersion = tls.VersionTLS12, 0
	if protocols == nil || *protocols == "" {
		return minVersion, maxVersion, nil
	}

	minVersion = 0xFFFF // Set to maxVersion value to find the true minimum.
	maxVersion = 0

	versionStrs := strings.Fields(*protocols)
	if len(versionStrs) == 0 {
		return tls.VersionTLS12, 0, nil
	}

	for _, vStr := range versionStrs {
		version, ok := protocolMap[strings.TrimSpace(vStr)]
		if !ok {
			return 0, 0, fmt.Errorf("unsupported TLS protocol: %s", vStr)
		}
		if version < minVersion {
			minVersion = version
		}
		if version > maxVersion {
			maxVersion = version
		}
	}

	// If only one version is specified, it serves as both minVersion and maxVersion.
	if len(versionStrs) == 1 {
		maxVersion = minVersion
	}

	return minVersion, maxVersion, nil
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
		return nil, errors.New("cipher suite string was provided but contained no valid ciphers")
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
