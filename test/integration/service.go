//go:build integration

package integration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// env is a running ABS instance under test, and a client to call its API with.
type env struct {
	baseURL string
	client  *http.Client
	// newestBeforeTrigger is when the newest backup of each type listed just before
	// the last trigger of that type was created.
	newestBeforeTrigger map[model.BackupType]time.Time
}

func newEnv(baseURL string, client *http.Client) *env {
	return &env{baseURL: baseURL, client: client, newestBeforeTrigger: map[model.BackupType]time.Time{}}
}

// setupEnv starts ABS on plain HTTP with baseConfig, after passing the config through
// each customize function. A test changes whatever it is about there, so this
// infrastructure never needs to know about the field.
func (s *Suite) setupEnv(customize ...func(*dto.Config)) *env {
	components := s.initComponents(customize...)

	srv := httptest.NewServer(components.Servers[0]) // baseConfig has only the HTTP listener.
	s.T().Cleanup(srv.Close)

	return newEnv(srv.URL, srv.Client())
}

// setupHTTPSEnv starts ABS with its HTTP listener disabled and an HTTPS listener
// serving certs. The listener's key-file-password is a Secret Agent reference that
// ABS resolves through agent when it builds the listener.
func (s *Suite) setupHTTPSEnv(certs httpsCertificates, agent *dto.SecretAgent) *env {
	port := s.freeHostPort()
	components := s.initComponents(func(c *dto.Config) {
		c.ServiceConfig.ServerHTTP = &dto.ServerConfigHTTP{
			ListenerConfig: dto.ListenerConfig{Address: "127.0.0.1", Disabled: true},
		}
		c.ServiceConfig.ServerHTTPS = &dto.ServerConfigHTTPS{
			ListenerConfig:  dto.ListenerConfig{Address: "127.0.0.1"},
			Port:            ptr.Of(dto.Port(port)),
			CertFile:        dto.Path(certs.CertFile),
			KeyFile:         dto.Path(certs.EncryptedKeyFile),
			KeyFilePassword: redact.Secret(secretRef()),
			SecretAgentConfig: dto.SecretAgentConfig{
				SecretAgent: agent,
			},
		}
	})
	s.Require().Len(components.Servers, 1)
	s.startServers(components.Servers)

	e := newEnv(fmt.Sprintf("https://127.0.0.1:%d", port), s.newHTTPSClient(certs.CAFile))
	s.waitForHealthy(e)

	return e
}

// initComponents writes the customized baseConfig to disk and starts ABS from it,
// the same way cmd/backup does.
func (s *Suite) initComponents(customize ...func(*dto.Config)) *app.Components {
	t := s.T()
	ctx := t.Context()

	config := s.baseConfig()
	for _, fn := range customize {
		fn(config)
	}

	configYAML, err := decoder.Marshal(config, decoder.YAML, false)
	s.Require().NoError(err)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	s.Require().NoError(os.WriteFile(configPath, configYAML, 0o600))

	components, err := app.InitComponents(ctx, configPath, false)
	s.Require().NoError(err)

	components.Start(ctx)

	return components
}

// baseConfig is a minimal working configuration: one cluster, one local storage, one
// policy and one routine that only runs when triggered explicitly.
func (s *Suite) baseConfig() *dto.Config {
	return &dto.Config{
		ServiceConfig: dto.ServiceConfig{
			Logger: &dto.LoggerConfig{Level: "ERROR"},
		},
		AerospikeClusters: map[string]*dto.AerospikeCluster{
			clusterName: {
				SeedNodes:            []dto.SeedNode{s.seedNode},
				UseServicesAlternate: ptr.Of(true),
			},
		},
		Storage: map[string]*dto.Storage{
			storageName: {
				LocalStorage: &dto.LocalStorage{Path: dto.Path(s.T().TempDir())},
			},
		},
		BackupPolicies: map[string]*dto.BackupPolicy{
			policyName: {
				Parallel: ptr.Of(1),
				RetentionPolicy: &dto.RetentionPolicy{
					FullBackups: ptr.Of(10),
					IncrBackups: ptr.Of(0),
				},
			},
		},
		BackupRoutines: map[string]*dto.BackupRoutine{
			routineName: {
				BackupPolicy:  policyName,
				SourceCluster: clusterName,
				Storage:       storageName,
				IntervalCron:  "@yearly",
				Namespaces:    ptr.Of([]dto.NamespaceName{namespace}),
			},
		},
	}
}

// startServers runs the listeners until the current test ends.
func (s *Suite) startServers(servers []server.HTTP) {
	t := s.T()

	// T().Context() is canceled before cleanups run; the listener must outlive it so
	// that the cleanup below is what stops it.
	ctx, cancel := context.WithCancel(s.cleanupContext())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, servers)
	}()

	t.Cleanup(func() {
		cancel()
		if err := <-errCh; err != nil {
			t.Errorf("HTTPS server stopped with error: %v", err)
		}
	})
}

func (s *Suite) newHTTPSClient(caFile string) *http.Client {
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

// waitForHealthy polls /health until the listener answers 200.
func (s *Suite) waitForHealthy(e *env) {
	ok := s.eventually(backupTimeout, 10*time.Millisecond, func() bool {
		req, err := http.NewRequestWithContext(s.T().Context(), http.MethodGet, e.baseURL+"/health", nil)
		s.Require().NoError(err)

		resp, err := e.client.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()

		return resp.StatusCode == http.StatusOK
	})
	s.Require().True(ok, "timed out waiting for %s/health", e.baseURL)
}
