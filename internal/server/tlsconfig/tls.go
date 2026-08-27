// Package tlsconfig builds server-safe TLS configurations for the HTTPS listener.
package tlsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

var secureCipherSuites = func() map[string]uint16 {
	suites := make(map[string]uint16)
	for _, suite := range tls.CipherSuites() {
		suites[suite.Name] = suite.ID
	}
	return suites
}()

// NewTLSConfig builds a static server TLS configuration.
// resolver is required to resolve KeyFilePassword when it is a Secret Agent reference.
func NewTLSConfig(
	ctx context.Context,
	config *model.ServerConfigHTTPS,
	resolver secrets.Resolver,
) (*tls.Config, error) {
	if config == nil {
		return nil, errors.New("HTTPS server config is required")
	}

	password, err := resolver.Resolve(ctx, config.SecretAgent, config.KeyFilePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve HTTPS key-file-password: %w", err)
	}

	certificate, err := loadKeyPair(config.CertFile, config.KeyFile, password)
	if err != nil {
		return nil, err
	}

	minVersion, err := parseMinVersion(config.GetMinVersionOrDefault())
	if err != nil {
		return nil, err
	}

	cipherSuites, err := parseCipherSuites(config.GetCipherSuitesOrDefault())
	if err != nil {
		return nil, err
	}

	clientAuth, err := config.GetClientAuthOrDefault().ToTLS()
	if err != nil {
		return nil, err
	}
	if clientAuth != tls.NoClientCert && config.ClientCAFile == "" {
		return nil, errors.New("TLS client authentication requires a client CA file")
	}

	result := &tls.Config{
		MinVersion:   minVersion,
		CipherSuites: cipherSuites,
		Certificates: []tls.Certificate{certificate},
	}

	if config.ClientCAFile != "" {
		result.ClientCAs, err = loadClientCAs(config.ClientCAFile)
		if err != nil {
			return nil, err
		}
		result.ClientAuth = clientAuth
	}

	return result, nil
}

func loadKeyPair(certFile, keyFile, password string) (tls.Certificate, error) {
	if password == "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("failed to load HTTPS certificate and key: %w", err)
		}
		return certificate, nil
	}

	certPEM, err := safepath.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to read HTTPS certificate: %w", err)
	}
	keyPEM, err := safepath.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to read HTTPS private key: %w", err)
	}

	keyBlock, rest := pem.Decode(keyPEM)
	if keyBlock == nil {
		return tls.Certificate{}, errors.New("failed to decode PEM block in HTTPS private key")
	}

	//nolint:staticcheck // Legacy PEM encryption is supported for compatibility with existing TLS configuration.
	if x509.IsEncryptedPEMBlock(keyBlock) {
		//nolint:staticcheck // Legacy PEM encryption is supported for compatibility with existing TLS configuration.
		decrypted, decryptErr := x509.DecryptPEMBlock(keyBlock, []byte(password))
		if decryptErr != nil {
			return tls.Certificate{}, fmt.Errorf("failed to decrypt HTTPS private key: %w", decryptErr)
		}
		keyBlock.Bytes = decrypted
		keyBlock.Headers = nil
		keyPEM = append(pem.EncodeToMemory(keyBlock), rest...)
	}

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load HTTPS certificate and key: %w", err)
	}

	return certificate, nil
}

func loadClientCAs(path string) (*x509.CertPool, error) {
	caPEM, err := safepath.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read client CA file: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("client CA file %q contains no certificates", path)
	}

	return pool, nil
}

func parseMinVersion(version model.TLSMinVersion) (uint16, error) {
	switch version {
	case model.TLSMinVersion12:
		return tls.VersionTLS12, nil
	case model.TLSMinVersion13:
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported minimum TLS version %q", version)
	}
}

func parseCipherSuites(names []string) ([]uint16, error) {
	if len(names) == 0 {
		return nil, nil
	}

	result := make([]uint16, 0, len(names))
	for _, name := range names {
		id, ok := secureCipherSuites[name]
		if !ok {
			return nil, fmt.Errorf("unsupported or insecure TLS cipher suite %q", name)
		}
		result = append(result, id)
	}

	return result, nil
}
