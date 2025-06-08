package aerospike

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

var protocolMap = map[string]uint16{
	"TLSv1.0": tls.VersionTLS10,
	"TLSv1.1": tls.VersionTLS11,
	"TLSv1.2": tls.VersionTLS12,
	"TLSv1.3": tls.VersionTLS13,
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

	// Load client certificates for mutual authentication.
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
		ServerName:   util.ValueOrZero(t.Name),
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
	// Get system CA certs, or create an empty pool if that fails.
	pool, err := x509.SystemCertPool()
	if err != nil {
		// This error is not critical, we can proceed with an empty pool.
		// slog.Warn("Failed to load system CA certificates", "err", err)
		pool = x509.NewCertPool()
	}

	// Load from CAFile if provided.
	if caFile != nil && *caFile != "" {
		pemBytes, err := os.ReadFile(*caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file %s: %w", *caFile, err)
		}
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("failed to append certificates from CA file %s", *caFile)
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
			filePath := filepath.Join(*caPath, file.Name())
			pemBytes, err := os.ReadFile(filePath)
			if err != nil {
				// Log a warning and continue, as some files in the dir might not be certs.
				// slog.Warn("Failed to read file in CAPath", "file", filePath, "err", err)
				continue
			}
			pool.AppendCertsFromPEM(pemBytes)
		}
	}

	return pool, nil
}

// loadClientCerts loads the client certificate and key for mTLS.
func loadClientCerts(t *model.TLS) ([]tls.Certificate, error) {
	certFile := util.ValueOrZero(t.Certfile)
	keyFile := util.ValueOrZero(t.Keyfile)

	if certFile == "" || keyFile == "" {
		return nil, nil // Not an error, just no client certs provided.
	}

	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read client certificate file %s: %w", certFile, err)
	}

	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read client key file %s: %w", keyFile, err)
	}

	// Decrypt the key if it's encrypted and a password is provided.
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM block from key file %s", keyFile)
	}

	if x509.IsEncryptedPEMBlock(keyBlock) { //nolint:staticcheck
		keyPassword := util.ValueOrZero(t.KeyfilePassword)
		if keyPassword == "" {
			return nil, fmt.Errorf("client key %s is encrypted but no password was provided", keyFile)
		}

		decryptedBytes, err := x509.DecryptPEMBlock(keyBlock, []byte(keyPassword)) //nolint:staticcheck
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt client key file %s: %w", keyFile, err)
		}

		// Re-encode the decrypted key back to PEM format for X509KeyPair.
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: keyBlock.Type, Bytes: decryptedBytes})
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create client key pair: %w", err)
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
			return 0, 0, fmt.Errorf("unknown TLS protocol: %s", vStr)
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
		return nil, fmt.Errorf("cipher suite string was provided but contained no valid ciphers")
	}

	return suites, nil
}
