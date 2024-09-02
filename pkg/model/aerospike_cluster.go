//nolint:lll
package model

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v7"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
// @Description AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	pwd atomic.Pointer[string]
	// The cluster name.
	ClusterLabel *string
	// The seed nodes details.
	SeedNodes []SeedNode
	// The connection timeout in milliseconds.
	ConnTimeout *int32
	// Whether should use "services-alternate" instead of "services" in info request during cluster tending.
	UseServicesAlternate *bool
	// The authentication details to the Aerospike cluster.
	Credentials *Credentials
	// The cluster TLS configuration.
	TLS *TLS
	// Specifies the maximum number of parallel scans per the cluster.
	MaxParallelScans *int
}

// NewLocalAerospikeCluster returns a new AerospikeCluster to be used in tests.
func NewLocalAerospikeCluster() *AerospikeCluster {
	return &AerospikeCluster{
		SeedNodes:   []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{User: util.Ptr("tester"), Password: util.Ptr("psw")},
	}
}

// GetUser safely returns the username.
func (c *AerospikeCluster) GetUser() *string {
	if c.Credentials != nil {
		return c.Credentials.User
	}
	return nil
}

// GetPassword tries to read and set the password once from PasswordPath, if it exists.
// Returns the password value. If it failed to read password, it will return nil
// and try to read again next time.
func (c *AerospikeCluster) GetPassword() *string {
	if password := c.pwd.Load(); password != nil {
		return password
	}

	if c.Credentials != nil && c.Credentials.Password != nil {
		c.pwd.Store(c.Credentials.Password)
		return c.Credentials.Password
	}

	if c.Credentials == nil || c.Credentials.PasswordPath == nil {
		slog.Warn("No credentials provided to read password")
		return nil
	}

	data, err := os.ReadFile(*c.Credentials.PasswordPath)
	if err != nil {
		slog.Error("Failed to read password", "path", *c.Credentials.PasswordPath, "err", err)
		return nil
	}

	slog.Debug("Successfully read password", "path", *c.Credentials.PasswordPath)
	password := string(data)
	c.pwd.Store(&password)
	return &password
}

// GetAuthMode safely returns the authentication mode.
func (c *AerospikeCluster) GetAuthMode() *string {
	if c.Credentials != nil {
		return c.Credentials.AuthMode
	}
	return nil
}

// ASClientPolicy builds and returns a new ClientPolicy from the AerospikeCluster configuration.
func (c *AerospikeCluster) ASClientPolicy() *as.ClientPolicy {
	policy := as.NewClientPolicy()
	if c.Credentials != nil {
		policy.User = util.ValueOrZero(c.Credentials.User)
		policy.Password = util.ValueOrZero(c.GetPassword())
		if c.Credentials.AuthMode != nil {
			switch strings.ToUpper(*c.Credentials.AuthMode) {
			case "INTERNAL":
				policy.AuthMode = as.AuthModeInternal
			case "EXTERNAL":
				policy.AuthMode = as.AuthModeExternal
			case "PKI":
				policy.AuthMode = as.AuthModePKI
			}
		}
	}
	if c.ConnTimeout != nil {
		policy.Timeout = time.Duration(*c.ConnTimeout) * time.Millisecond
	}
	if c.UseServicesAlternate != nil {
		policy.UseServicesAlternate = *c.UseServicesAlternate
	}
	if c.TLS != nil {
		policy.TlsConfig = initTLS(c.TLS, c.ClusterLabel)
	}
	if c.MaxParallelScans != nil && *c.MaxParallelScans > 0 {
		policy.ConnectionQueueSize = max(256, *c.MaxParallelScans*2)
	}
	policy.LimitConnectionsToQueueSize = false
	return policy
}

//nolint:funlen,staticcheck
func initTLS(t *TLS, clusterLabel *string) *tls.Config {
	clusterName := "NA"
	if clusterLabel != nil {
		clusterName = *clusterLabel
	}
	errorLog := func(err error) {
		slog.Error("Failed to initialize tls.Config", "cluster", clusterName, "err", err)
	}

	// Try to load system CA certs, otherwise just make an empty pool
	serverPool, err := x509.SystemCertPool()
	if serverPool == nil || err != nil {
		serverPool = x509.NewCertPool()
	}

	if t.CAFile != nil && len(*t.CAFile) > 0 {
		// Try to load system CA certs and add them to the system cert pool
		caCert, err := readFromFile(*t.CAFile)
		if err != nil {
			errorLog(err)
			return nil
		}
		serverPool.AppendCertsFromPEM(caCert)
	}

	var clientPool []tls.Certificate
	if (t.Certfile != nil && len(*t.Certfile) > 0) ||
		t.Keyfile != nil && len(*t.Keyfile) > 0 {
		// Read cert file
		certFileBytes, err := readFromFile(*t.Certfile)
		if err != nil {
			errorLog(err)
			return nil
		}

		// Read key file
		keyFileBytes, err := readFromFile(*t.Keyfile)
		if err != nil {
			errorLog(err)
			return nil
		}

		// Decode PEM data
		keyBlock, _ := pem.Decode(keyFileBytes)
		certBlock, _ := pem.Decode(certFileBytes)

		if keyBlock == nil || certBlock == nil {
			errorLog(errors.New("failed to decode PEM data for key or certificate"))
			return nil
		}

		// Check and Decrypt the the Key Block using passphrase
		if t.KeyfilePassword != nil && x509.IsEncryptedPEMBlock(keyBlock) {
			decryptedDERBytes, err := x509.DecryptPEMBlock(keyBlock, []byte(*t.KeyfilePassword))
			if err != nil {
				errorLog(err)
				return nil
			}

			keyBlock.Bytes = decryptedDERBytes
			keyBlock.Headers = nil
		}

		// Encode PEM data
		keyPEM := pem.EncodeToMemory(keyBlock)
		certPEM := pem.EncodeToMemory(certBlock)

		if keyPEM == nil || certPEM == nil {
			errorLog(fmt.Errorf("failed to encode PEM data for key or certificate"))
		}

		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			errorLog(fmt.Errorf("failed to add client certificate and key to the pool: %s", err))
		}

		clientPool = append(clientPool, cert)
		slog.Debug("Added TLS client certificate and key to the pool", "cluster", clusterName)
	}
	tlsConfig := &tls.Config{
		Certificates:             clientPool,
		RootCAs:                  serverPool,
		InsecureSkipVerify:       false,
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
	}

	return tlsConfig
}

func readFromFile(filePath string) ([]byte, error) {
	dataBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read from file %s: %v", filePath, err)
	}
	data := bytes.TrimSuffix(dataBytes, []byte("\n"))

	return data, nil
}

// ASClientHosts builds and returns a Host list from the AerospikeCluster configuration.
func (c *AerospikeCluster) ASClientHosts() []*as.Host {
	hosts := make([]*as.Host, 0, len(c.SeedNodes))
	for _, node := range c.SeedNodes {
		hosts = append(hosts, &as.Host{
			Name:    node.HostName,
			Port:    int(node.Port),
			TLSName: node.TLSName,
		})
	}
	return hosts
}

// TLS represents the Aerospike cluster TLS configuration options.
// @Description TLS represents the Aerospike cluster TLS configuration options.
type TLS struct {
	// Path to a trusted CA certificate file.
	CAFile *string
	// Path to a directory of trusted CA certificates.
	CAPath *string
	// The default TLS name used to authenticate each TLS socket connection.
	Name *string
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string
}

// Credentials represents authentication details to the Aerospike cluster.
// @Description Credentials represents authentication details to the Aerospike cluster.
type Credentials struct {
	// The username for the cluster authentication.
	User *string
	// The password for the cluster authentication.
	Password *string
	// The file path with the password string, will take precedence over the password field.
	PasswordPath *string
	// The authentication mode string (INTERNAL, EXTERNAL, EXTERNAL_INSECURE, PKI).
	AuthMode *string
}

// SeedNode represents details of a node in the Aerospike cluster.
// @Description SeedNode represents details of a node in the Aerospike cluster.
type SeedNode struct {
	// The host name of the node.
	HostName string
	// The port of the node.
	Port int32
	// TLS certificate name used for secure connections (if enabled).
	TLSName string
}
