package server_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listenerCertificates struct {
	caFile         string
	serverCertFile string
	serverKeyFile  string
	clientCertFile string
	clientKeyFile  string
}

// listenerCase describes one valid service.http/service.https configuration.
// The expected listener flags are checked against actual TCP connections rather
// than only checking which component was constructed.
type listenerCase struct {
	name           string
	config         func(httpPort, httpsPort int, certs listenerCertificates) dto.Config
	sendClientCert bool
	wantHTTP       bool
	wantHTTPS      bool
}

// TestConfiguredListenersServeFullStackAPI verifies listener configuration
// through the same wiring used by the application. Each case creates a
// minimal configuration, starts the configured listeners, and calls a real API
// endpoint without requiring Aerospike, storage, or backup routines.
func TestConfiguredListenersServeFullStackAPI(t *testing.T) {
	certs := createListenerCertificates(t)

	tests := []listenerCase{
		{
			name: "default HTTP",
			config: func(httpPort, _ int, _ listenerCertificates) dto.Config {
				return dto.Config{
					ServiceConfig: dto.ServiceConfig{
						ServerHTTP: &dto.ServerConfigHTTP{
							ListenerConfig: dto.ListenerConfig{
								Address: "127.0.0.1",
							},
							Port: ptr.Of(dto.Port(httpPort)),
						},
					},
				}
			},
			wantHTTP: true,
		},
		{
			name: "explicit HTTP enabled",
			config: func(httpPort, _ int, _ listenerCertificates) dto.Config {
				return dto.Config{
					ServiceConfig: dto.ServiceConfig{
						ServerHTTP: &dto.ServerConfigHTTP{
							ListenerConfig: dto.ListenerConfig{
								Address:  "127.0.0.1",
								Disabled: false,
							},
							Port: ptr.Of(dto.Port(httpPort)),
						},
					},
				}
			},
			wantHTTP: true,
		},
		{
			name: "HTTPS without client authentication",
			config: func(httpPort, httpsPort int, certs listenerCertificates) dto.Config {
				return dto.Config{
					ServiceConfig: dto.ServiceConfig{
						ServerHTTP: &dto.ServerConfigHTTP{
							ListenerConfig: dto.ListenerConfig{
								Address:  "127.0.0.1",
								Disabled: true,
							},
							Port: ptr.Of(dto.Port(httpPort)),
						},
						ServerHTTPS: &dto.ServerConfigHTTPS{
							ListenerConfig: dto.ListenerConfig{
								Address: "127.0.0.1",
							},
							Port:         ptr.Of(dto.Port(httpsPort)),
							CertFile:     certs.serverCertFile,
							KeyFile:      certs.serverKeyFile,
							ClientCAFile: certs.caFile,
							ClientAuth:   dto.TLSClientAuthNone,
						},
					},
				}
			},
			wantHTTPS: true,
		},
		{
			name: "HTTPS explicitly disabled",
			config: func(httpPort, httpsPort int, _ listenerCertificates) dto.Config {
				return dto.Config{
					ServiceConfig: dto.ServiceConfig{
						ServerHTTP: &dto.ServerConfigHTTP{
							ListenerConfig: dto.ListenerConfig{
								Address: "127.0.0.1",
							},
							Port: ptr.Of(dto.Port(httpPort)),
						},
						ServerHTTPS: &dto.ServerConfigHTTPS{
							ListenerConfig: dto.ListenerConfig{
								Address:  "127.0.0.1",
								Disabled: true,
							},
							Port: ptr.Of(dto.Port(httpsPort)),
						},
					},
				}
			},
			wantHTTP: true,
		},
		{
			name: "HTTPS request client certificate",
			config: func(httpPort, httpsPort int, certs listenerCertificates) dto.Config {
				return dto.Config{
					ServiceConfig: dto.ServiceConfig{
						ServerHTTP: &dto.ServerConfigHTTP{
							ListenerConfig: dto.ListenerConfig{
								Address:  "127.0.0.1",
								Disabled: true,
							},
							Port: ptr.Of(dto.Port(httpPort)),
						},
						ServerHTTPS: &dto.ServerConfigHTTPS{
							ListenerConfig: dto.ListenerConfig{
								Address: "127.0.0.1",
							},
							Port:         ptr.Of(dto.Port(httpsPort)),
							CertFile:     certs.serverCertFile,
							KeyFile:      certs.serverKeyFile,
							ClientCAFile: certs.caFile,
							ClientAuth:   dto.TLSClientAuthRequest,
						},
					},
				}
			},
			wantHTTPS: true,
		},
		{
			name: "HTTPS require and verify client certificate",
			config: func(httpPort, httpsPort int, certs listenerCertificates) dto.Config {
				return dto.Config{
					ServiceConfig: dto.ServiceConfig{
						ServerHTTP: &dto.ServerConfigHTTP{
							ListenerConfig: dto.ListenerConfig{
								Address:  "127.0.0.1",
								Disabled: true,
							},
							Port: ptr.Of(dto.Port(httpPort)),
						},
						ServerHTTPS: &dto.ServerConfigHTTPS{
							ListenerConfig: dto.ListenerConfig{
								Address: "127.0.0.1",
							},
							Port:         ptr.Of(dto.Port(httpsPort)),
							CertFile:     certs.serverCertFile,
							KeyFile:      certs.serverKeyFile,
							ClientCAFile: certs.caFile,
							ClientAuth:   dto.TLSClientAuthRequireAndVerify,
						},
					},
				}
			},
			sendClientCert: true,
			wantHTTPS:      true,
		},
		{
			name: "both listeners",
			config: func(httpPort, httpsPort int, certs listenerCertificates) dto.Config {
				return dto.Config{
					ServiceConfig: dto.ServiceConfig{
						ServerHTTP: &dto.ServerConfigHTTP{
							ListenerConfig: dto.ListenerConfig{
								Address: "127.0.0.1",
							},
							Port: ptr.Of(dto.Port(httpPort)),
						},
						ServerHTTPS: &dto.ServerConfigHTTPS{
							ListenerConfig: dto.ListenerConfig{
								Address: "127.0.0.1",
							},
							Port:         ptr.Of(dto.Port(httpsPort)),
							CertFile:     certs.serverCertFile,
							KeyFile:      certs.serverKeyFile,
							ClientCAFile: certs.caFile,
							ClientAuth:   dto.TLSClientAuthNone,
						},
					},
				}
			},
			wantHTTP:  true,
			wantHTTPS: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpPort := freeListenerPort(t)
			httpsPort := freeListenerPort(t)
			cfg := test.config(httpPort, httpsPort, certs)

			components := initListenerComponents(t, cfg)
			t.Cleanup(components.Scheduler.Stop)

			httpURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
			httpsURL := fmt.Sprintf("https://127.0.0.1:%d", httpsPort)
			// HTTPS clients must trust the temporary CA. The mTLS case also
			// presents a client certificate signed by that same CA.
			client := &http.Client{Timeout: time.Second}
			if test.sendClientCert {
				client = &http.Client{
					Timeout: time.Second,
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							RootCAs: loadCertificatePool(t, certs.caFile),
							Certificates: []tls.Certificate{
								loadKeyPair(t, certs.clientCertFile, certs.clientKeyFile),
							},
							MinVersion: tls.VersionTLS12,
						},
					},
				}
			} else if test.wantHTTPS {
				client.Transport = &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs:    loadCertificatePool(t, certs.caFile),
						MinVersion: tls.VersionTLS12,
					},
				}
			}

			// Run servers concurrently using production server.Run logic.
			srvCtx, srvCancel := context.WithCancel(t.Context())
			errCh := startListeners(t, srvCtx, components)

			if test.wantHTTP {
				waitForListener(t, client, httpURL)
				assertAPIResponse(t, client, httpURL)
			} else {
				assertListenerUnavailable(t, httpURL)
			}
			if test.wantHTTPS {
				waitForListener(t, client, httpsURL)
				assertAPIResponse(t, client, httpsURL)
			} else {
				assertListenerUnavailable(t, httpsURL)
			}

			srvCancel()
			require.NoError(t, <-errCh)
		})
	}
}

// TestConfiguredContextPathPrefixesRoutes verifies that context-path moves both
// the system and the API routes under the configured prefix.
func TestConfiguredContextPathPrefixesRoutes(t *testing.T) {
	httpPort := freeListenerPort(t)
	cfg := dto.Config{
		ServiceConfig: dto.ServiceConfig{
			ServerHTTP: &dto.ServerConfigHTTP{
				ListenerConfig: dto.ListenerConfig{
					Address:     "127.0.0.1",
					ContextPath: "/abs",
				},
				Port: ptr.Of(dto.Port(httpPort)),
			},
		},
	}

	components := initListenerComponents(t, cfg)
	t.Cleanup(components.Scheduler.Stop)

	srvCtx, srvCancel := context.WithCancel(t.Context())
	errCh := startListeners(t, srvCtx, components)

	client := &http.Client{Timeout: time.Second}
	prefixedURL := fmt.Sprintf("http://127.0.0.1:%d/abs", httpPort)
	waitForListener(t, client, prefixedURL)
	assertAPIResponse(t, client, prefixedURL)

	// Nothing is registered outside the prefix, including the root handler.
	assertStatus(t, client, fmt.Sprintf("http://127.0.0.1:%d/v1/backups/full", httpPort), http.StatusNotFound)

	srvCancel()
	require.NoError(t, <-errCh)
}

// TestHTTPSListenerNegotiatesHTTP2 verifies that the HTTPS listener advertises h2
// through ALPN, which net/http only does when TLSConfig is attached to the server.
func TestHTTPSListenerNegotiatesHTTP2(t *testing.T) {
	certs := createListenerCertificates(t)
	httpPort := freeListenerPort(t)
	httpsPort := freeListenerPort(t)
	cfg := dto.Config{
		ServiceConfig: dto.ServiceConfig{
			ServerHTTP: &dto.ServerConfigHTTP{
				ListenerConfig: dto.ListenerConfig{
					Address:  "127.0.0.1",
					Disabled: true,
				},
				Port: ptr.Of(dto.Port(httpPort)),
			},
			ServerHTTPS: &dto.ServerConfigHTTPS{
				ListenerConfig: dto.ListenerConfig{
					Address: "127.0.0.1",
				},
				Port:     ptr.Of(dto.Port(httpsPort)),
				CertFile: certs.serverCertFile,
				KeyFile:  certs.serverKeyFile,
			},
		},
	}

	components := initListenerComponents(t, cfg)
	t.Cleanup(components.Scheduler.Stop)

	srvCtx, srvCancel := context.WithCancel(t.Context())
	errCh := startListeners(t, srvCtx, components)

	client := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				RootCAs:    loadCertificatePool(t, certs.caFile),
				MinVersion: tls.VersionTLS12,
			},
		},
	}
	httpsURL := fmt.Sprintf("https://127.0.0.1:%d", httpsPort)
	waitForListener(t, client, httpsURL)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, httpsURL+"/v1/backups/full", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "HTTP/2.0", resp.Proto)

	srvCancel()
	require.NoError(t, <-errCh)
}

func TestConfiguredHTTPSContextPathPrefixesRoutes(t *testing.T) {
	certs := createListenerCertificates(t)
	httpPort := freeListenerPort(t)
	httpsPort := freeListenerPort(t)
	cfg := httpsOnlyConfig(httpPort, httpsPort, certs)
	cfg.ServiceConfig.ServerHTTPS.ContextPath = "/abs"

	components := initListenerComponents(t, cfg)
	t.Cleanup(components.Scheduler.Stop)

	srvCtx, srvCancel := context.WithCancel(t.Context())
	errCh := startListeners(t, srvCtx, components)

	client := httpsClient(t, certs, false)
	prefixedURL := fmt.Sprintf("https://127.0.0.1:%d/abs", httpsPort)
	waitForListener(t, client, prefixedURL)
	assertAPIResponse(t, client, prefixedURL)
	assertStatus(t, client, fmt.Sprintf("https://127.0.0.1:%d/v1/backups/full", httpsPort), http.StatusNotFound)

	srvCancel()
	require.NoError(t, <-errCh)
}

func TestHTTPSRequestClientCertificateAllowsAnonymousClient(t *testing.T) {
	certs := createListenerCertificates(t)
	httpPort := freeListenerPort(t)
	httpsPort := freeListenerPort(t)
	cfg := httpsOnlyConfig(httpPort, httpsPort, certs)
	cfg.ServiceConfig.ServerHTTPS.ClientCAFile = certs.caFile
	cfg.ServiceConfig.ServerHTTPS.ClientAuth = dto.TLSClientAuthRequest

	components := initListenerComponents(t, cfg)
	t.Cleanup(components.Scheduler.Stop)

	srvCtx, srvCancel := context.WithCancel(t.Context())
	errCh := startListeners(t, srvCtx, components)

	client := httpsClient(t, certs, false)
	httpsURL := fmt.Sprintf("https://127.0.0.1:%d", httpsPort)
	waitForListener(t, client, httpsURL)
	assertAPIResponse(t, client, httpsURL)

	srvCancel()
	require.NoError(t, <-errCh)
}

func TestHTTPSRequireAndVerifyRejectsMissingClientCertificate(t *testing.T) {
	certs := createListenerCertificates(t)
	httpPort := freeListenerPort(t)
	httpsPort := freeListenerPort(t)
	cfg := httpsOnlyConfig(httpPort, httpsPort, certs)
	cfg.ServiceConfig.ServerHTTPS.ClientCAFile = certs.caFile
	cfg.ServiceConfig.ServerHTTPS.ClientAuth = dto.TLSClientAuthRequireAndVerify

	components := initListenerComponents(t, cfg)
	t.Cleanup(components.Scheduler.Stop)

	srvCtx, srvCancel := context.WithCancel(t.Context())
	errCh := startListeners(t, srvCtx, components)

	httpsURL := fmt.Sprintf("https://127.0.0.1:%d", httpsPort)
	waitForTCP(t, fmt.Sprintf("127.0.0.1:%d", httpsPort))

	anonymous := httpsClient(t, certs, false)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, httpsURL+"/health", nil)
	require.NoError(t, err)
	resp, err := anonymous.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)

	authenticated := httpsClient(t, certs, true)
	waitForListener(t, authenticated, httpsURL)
	assertAPIResponse(t, authenticated, httpsURL)

	srvCancel()
	require.NoError(t, <-errCh)
}

func TestHTTPSRequireAndVerifyRejectsUntrustedClientCertificate(t *testing.T) {
	certs := createListenerCertificates(t)
	other := createListenerCertificates(t)
	httpPort := freeListenerPort(t)
	httpsPort := freeListenerPort(t)
	cfg := httpsOnlyConfig(httpPort, httpsPort, certs)
	cfg.ServiceConfig.ServerHTTPS.ClientCAFile = certs.caFile
	cfg.ServiceConfig.ServerHTTPS.ClientAuth = dto.TLSClientAuthRequireAndVerify

	components := initListenerComponents(t, cfg)
	t.Cleanup(components.Scheduler.Stop)

	srvCtx, srvCancel := context.WithCancel(t.Context())
	errCh := startListeners(t, srvCtx, components)

	httpsURL := fmt.Sprintf("https://127.0.0.1:%d", httpsPort)
	waitForTCP(t, fmt.Sprintf("127.0.0.1:%d", httpsPort))

	untrusted := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: loadCertificatePool(t, certs.caFile),
				Certificates: []tls.Certificate{
					loadKeyPair(t, other.clientCertFile, other.clientKeyFile),
				},
				MinVersion: tls.VersionTLS12,
			},
		},
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, httpsURL+"/health", nil)
	require.NoError(t, err)
	resp, err := untrusted.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)

	authenticated := httpsClient(t, certs, true)
	waitForListener(t, authenticated, httpsURL)
	assertAPIResponse(t, authenticated, httpsURL)

	srvCancel()
	require.NoError(t, <-errCh)
}

func TestNewServerHTTPSReturnsTLSConfigError(t *testing.T) {
	_, err := server.NewServerHTTPS(
		t.Context(),
		&model.ServerConfigHTTPS{
			CertFile: filepath.Join(t.TempDir(), "missing.pem"),
			KeyFile:  filepath.Join(t.TempDir(), "missing-key.pem"),
		},
		newListenerHandler(t),
		secrets.NewResolver(),
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load HTTPS certificate and key")
}

func TestRunFailsFastWhenListenerCannotBind(t *testing.T) {
	certs := createListenerCertificates(t)
	occupied := occupyPort(t)
	httpPort := occupied.Addr().(*net.TCPAddr).Port
	httpsPort := freeListenerPort(t)
	cfg := dto.Config{
		ServiceConfig: dto.ServiceConfig{
			ServerHTTP: &dto.ServerConfigHTTP{
				ListenerConfig: dto.ListenerConfig{Address: "127.0.0.1"},
				Port:           ptr.Of(dto.Port(httpPort)),
			},
			ServerHTTPS: &dto.ServerConfigHTTPS{
				ListenerConfig: dto.ListenerConfig{Address: "127.0.0.1"},
				Port:           ptr.Of(dto.Port(httpsPort)),
				CertFile:       certs.serverCertFile,
				KeyFile:        certs.serverKeyFile,
			},
		},
	}

	components := initListenerComponents(t, cfg)
	t.Cleanup(components.Scheduler.Stop)

	err := server.Run(t.Context(), components.Servers)
	require.ErrorContains(t, err, fmt.Sprintf("HTTP listener 127.0.0.1:%d", httpPort))
}

func TestHTTPSStartFailsWhenPortIsOccupied(t *testing.T) {
	certs := createListenerCertificates(t)
	occupied := occupyPort(t)
	httpsPort := occupied.Addr().(*net.TCPAddr).Port

	srv, err := server.NewServerHTTPS(
		t.Context(),
		&model.ServerConfigHTTPS{
			ListenerConfig: model.ListenerConfig{Address: "127.0.0.1"},
			Port:           ptr.Of(model.Port(httpsPort)),
			CertFile:       certs.serverCertFile,
			KeyFile:        certs.serverKeyFile,
		},
		newListenerHandler(t),
		secrets.NewResolver(),
	)
	require.NoError(t, err)

	err = srv.Start()
	require.Error(t, err)
	require.ErrorContains(t, err, fmt.Sprintf("HTTPS listener 127.0.0.1:%d", httpsPort))
	require.NotErrorIs(t, err, http.ErrServerClosed)
}

func initListenerComponents(t *testing.T, cfg dto.Config) *app.Components {
	t.Helper()

	// Serialize dto.Config directly into YAML file.
	yamlBytes, err := decoder.Marshal(&cfg, decoder.YAML, false)
	require.NoError(t, err)

	// InitComponents loads and validates the file, then wires the complete
	// service graph.
	configPath := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(configPath, yamlBytes, 0o600))

	components, err := app.InitComponents(t.Context(), configPath, false)
	require.NoError(t, err)
	require.NotNil(t, components)
	return components
}

func startListeners(t *testing.T, ctx context.Context, components *app.Components) <-chan error {
	t.Helper()

	listeners := components.Servers
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, listeners)
	}()

	return errCh
}

func httpsOnlyConfig(httpPort, httpsPort int, certs listenerCertificates) dto.Config {
	return dto.Config{
		ServiceConfig: dto.ServiceConfig{
			ServerHTTP: &dto.ServerConfigHTTP{
				ListenerConfig: dto.ListenerConfig{
					Address:  "127.0.0.1",
					Disabled: true,
				},
				Port: ptr.Of(dto.Port(httpPort)),
			},
			ServerHTTPS: &dto.ServerConfigHTTPS{
				ListenerConfig: dto.ListenerConfig{
					Address: "127.0.0.1",
				},
				Port:     ptr.Of(dto.Port(httpsPort)),
				CertFile: certs.serverCertFile,
				KeyFile:  certs.serverKeyFile,
			},
		},
	}
}

func httpsClient(t *testing.T, certs listenerCertificates, sendClientCert bool) *http.Client {
	t.Helper()

	tlsConfig := &tls.Config{
		RootCAs:    loadCertificatePool(t, certs.caFile),
		MinVersion: tls.VersionTLS12,
	}
	if sendClientCert {
		tlsConfig.Certificates = []tls.Certificate{
			loadKeyPair(t, certs.clientCertFile, certs.clientKeyFile),
		}
	}

	return &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()

	dialer := &net.Dialer{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := dialer.DialContext(t.Context(), "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for TCP listener %s", addr)
}

func occupyPort(t *testing.T) net.Listener {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func newListenerHandler(t *testing.T) *handlers.Service {
	t.Helper()

	return handlers.NewService(
		t.Context(),
		model.NewConfig(),
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func waitForListener(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()

	// Start is asynchronous, so health polling avoids a race between listener
	// startup and the first API request.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/health", nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", baseURL)
}

func assertAPIResponse(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()

	// The full-backups endpoint returns a map keyed by routine. With no
	// routines configured, its valid empty JSON response is an object: {}.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/v1/backups/full", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, "{}", string(body))
}

func assertStatus(t *testing.T, client *http.Client, url string, wantStatus int) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, wantStatus, resp.StatusCode)
}

func assertListenerUnavailable(t *testing.T, baseURL string) {
	t.Helper()

	// Disabled listeners are omitted from Components, so no process should be
	// accepting connections on their configured port.
	targetAddr := baseURL[len("http://"):]
	if len(baseURL) >= len("https://") && baseURL[:len("https://")] == "https://" {
		targetAddr = baseURL[len("https://"):]
	}
	dialer := &net.Dialer{Timeout: 100 * time.Millisecond}
	conn, err := dialer.DialContext(t.Context(), "tcp", targetAddr)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("listener %s accepted a connection", baseURL)
	}
}

func freeListenerPort(t *testing.T) int {
	t.Helper()

	// Configuration validation requires a non-zero port. Reserve an ephemeral
	// port briefly, then use it for the service under test.
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func createListenerCertificates(t *testing.T) listenerCertificates {
	t.Helper()

	// Create a CA, server certificate, and client certificate on disk because
	// the application configuration accepts certificate file paths.
	dir := t.TempDir()
	caKey := generateRSAKey(t)
	caTemplate := &x509.Certificate{
		SerialNumber:          new(big.Int).SetInt64(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caFile := writePEM(t, dir, "ca.pem", "CERTIFICATE", caDER)

	serverKey := generateRSAKey(t)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	require.NoError(t, err)
	serverCertFile := writePEM(t, dir, "server.pem", "CERTIFICATE", serverDER)
	serverKeyFile := writePEM(t, dir, "server-key.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverKey))

	clientKey := generateRSAKey(t)
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	require.NoError(t, err)
	clientCertFile := writePEM(t, dir, "client.pem", "CERTIFICATE", clientDER)
	clientKeyFile := writePEM(t, dir, "client-key.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(clientKey))

	return listenerCertificates{caFile, serverCertFile, serverKeyFile, clientCertFile, clientKeyFile}
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func writePEM(t *testing.T, dir, name, pemType string, der []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: der}), 0o600))
	return path
}

func loadCertificatePool(t *testing.T, path string) *x509.CertPool {
	t.Helper()
	pemData, err := os.ReadFile(path)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(pemData))
	return pool
}

func loadKeyPair(t *testing.T, certFile, keyFile string) tls.Certificate {
	t.Helper()
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	return certificate
}
