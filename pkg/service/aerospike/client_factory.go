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
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// DefaultClientFactory is the default implementation of ClientFactory.
type DefaultClientFactory struct{}

// NewClientWithPolicyAndHost creates a new Aerospike client with the given policy and hosts.
func (f *DefaultClientFactory) NewClientWithPolicyAndHost(
	cluster *model.AerospikeCluster,
) (backup.AerospikeClient, error) {
	return as.NewClientWithPolicyAndHost(clientPolicy(cluster), clientHosts(cluster)...)
}

// IsClusterHealthy checks if the cluster is connected and responding.
func (f *DefaultClientFactory) IsClusterHealthy(client Cluster) bool {
	if client == nil {
		return false
	}

	cluster := client.Cluster()
	if !cluster.IsConnected() {
		return false
	}

	node, err := cluster.GetRandomNode()
	if err != nil {
		return false
	}

	info, err := node.RequestInfo(as.NewInfoPolicy(), "status")

	return err == nil && info["status"] == "ok"
}

// clientHosts builds and returns a Host list from the AerospikeCluster configuration.
func clientHosts(c *model.AerospikeCluster) []*as.Host {
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

// clientPolicy builds and returns a new ClientPolicy from the AerospikeCluster configuration.
func clientPolicy(c *model.AerospikeCluster) *as.ClientPolicy {
	policy := as.NewClientPolicy()
	if c.Credentials != nil {
		policy.User = util.ValueOrZero(c.GetUser())
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
		policy.Timeout = *c.ConnTimeout
	}
	if c.UseServicesAlternate != nil {
		policy.UseServicesAlternate = *c.UseServicesAlternate
	}
	if c.TLS != nil {
		policy.TlsConfig = initTLS(c.TLS, c.ClusterLabel)
	}

	policy.ConnectionQueueSize = 256
	policy.LimitConnectionsToQueueSize = false

	return policy
}

//nolint:funlen,staticcheck,nestif
func initTLS(t *model.TLS, clusterLabel *string) *tls.Config {
	clusterName := "NA"
	if clusterLabel != nil {
		clusterName = *clusterLabel
	}
	errorLog := func(err error) {
		slog.Error("Failed to initialize tls.Config",
			slog.String("cluster", clusterName),
			slog.Any("err", err))
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

		// Check and Decrypt the Key Block using passphrase
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
			errorLog(fmt.Errorf("failed to add client certificate and key to the pool: %w", err))
		}

		clientPool = append(clientPool, cert)
		slog.Debug("Added TLS client certificate and key to the pool",
			slog.String("cluster", clusterName))
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
		return nil, fmt.Errorf("failed to read from file %s: %w", filePath, err)
	}
	data := bytes.TrimSuffix(dataBytes, []byte("\n"))

	return data, nil
}
