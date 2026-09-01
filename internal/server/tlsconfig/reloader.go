package tlsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/reload"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
)

// WatchInterval is how often TLS files are polled for rotation.
const WatchInterval = 10 * time.Second

// Reloader loads the HTTPS TLS material and can watch it for rotation.
type Reloader interface {
	DynamicTLS
	// Load reads the key pair and, when configured, the client CA pool.
	// The first call must succeed; later failures keep the last good material.
	Load(ctx context.Context) error
	// Start watches the TLS files until ctx is canceled.
	Start(ctx context.Context)
}

var (
	_ Reloader = (*CertificateReloader)(nil)
	_ Reloader = noOpReloader{}
)

// CertificateReloader is the single point that reads HTTPS TLS material from disk.
// It swaps the key pair when cert-file or key-file change and, when client-ca-file
// is set, swaps the mTLS trust pool served through GetConfigForClient.
type CertificateReloader struct {
	config    *model.ServerConfigHTTPS
	resolver  secrets.Resolver
	watchers  []reload.Watcher
	current   atomic.Pointer[tls.Certificate]
	clientCAs atomic.Pointer[x509.CertPool]
	baseTLS   atomic.Pointer[tls.Config]
	mu        sync.Mutex
}

// NewCertificateReloader returns a reloader that watches the configured TLS files.
// It performs no I/O beyond fingerprinting them: call Load for the initial material
// and Start to watch for changes.
func NewCertificateReloader(
	config *model.ServerConfigHTTPS,
	resolver secrets.Resolver,
	interval time.Duration,
) *CertificateReloader {
	reloader := NewCertificateLoader(config, resolver)
	// The unit of serving is a matched tls.Certificate (cert PEM + key), not two
	// independent files. Reloading a half would handshake with new-cert/old-key
	// (or the reverse). Either path changing therefore re-reads both key-pair files.
	reloader.watchers = []reload.Watcher{
		reload.New(config.CertFile, interval, reloader.loadKeyPair),
		reload.New(config.KeyFile, interval, reloader.loadKeyPair),
	}
	if config.ClientCAFile != "" {
		reloader.watchers = append(reloader.watchers,
			reload.New(config.ClientCAFile, interval, reloader.loadClientCAs),
		)
	}

	return reloader
}

// NewCertificateLoader returns a reloader that reads the same TLS material but never
// watches it. Configuration validation uses it to exercise the listener's load path.
func NewCertificateLoader(config *model.ServerConfigHTTPS, resolver secrets.Resolver) *CertificateReloader {
	return &CertificateReloader{
		config:   config,
		resolver: resolver,
	}
}

// NoReload returns a reloader that does nothing, for listeners that serve plaintext.
func NoReload() Reloader {
	return noOpReloader{}
}

type noOpReloader struct{}

func (noOpReloader) Load(context.Context) error { return nil }

func (noOpReloader) Start(context.Context) {}

func (noOpReloader) SetBaseConfig(*tls.Config) {}

func (noOpReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, errors.New("HTTPS certificate is not loaded")
}

func (noOpReloader) GetConfigForClient(*tls.ClientHelloInfo) (*tls.Config, error) {
	return nil, errors.New("HTTPS client CA pool is not loaded")
}

// Load reads the key pair and the client CA pool and serves them to new handshakes.
func (r *CertificateReloader) Load(ctx context.Context) error {
	if err := r.loadKeyPair(ctx); err != nil {
		return err
	}

	return r.loadClientCAs(ctx)
}

func (r *CertificateReloader) loadKeyPair(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Disk I/O and Secret Agent stay under this lock so two watchers cannot interleave
	// half-applied pairs. GetCertificate reads the atomic pointer and is not blocked.
	password, err := r.resolver.Resolve(ctx, r.config.SecretAgent, r.config.KeyFilePassword)
	if err != nil {
		return fmt.Errorf("failed to resolve HTTPS key-file-password: %w", err)
	}

	certificate, err := loadKeyPair(r.config.CertFile, r.config.KeyFile, password)
	if err != nil {
		return err
	}

	message := "loaded HTTPS certificate"
	if r.current.Load() != nil {
		message = "rotated HTTPS certificate"
	}
	r.current.Store(&certificate)
	slog.Info(message,
		slog.String("certFile", r.config.CertFile),
		slog.String("keyFile", r.config.KeyFile),
	)

	return nil
}

func (r *CertificateReloader) loadClientCAs(_ context.Context) error {
	if r.config.ClientCAFile == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	pool, err := loadClientCAs(r.config.ClientCAFile)
	if err != nil {
		return err
	}

	message := "loaded HTTPS client CA pool"
	if r.clientCAs.Load() != nil {
		message = "rotated HTTPS client CA pool"
	}
	r.clientCAs.Store(pool)
	slog.Info(message, slog.String("clientCaFile", r.config.ClientCAFile))

	return nil
}

// SetBaseConfig stores the listener tls.Config template cloned by GetConfigForClient.
func (r *CertificateReloader) SetBaseConfig(config *tls.Config) {
	r.baseTLS.Store(config)
}

// GetCertificate returns the currently loaded key pair, for tls.Config.GetCertificate.
func (r *CertificateReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certificate := r.current.Load()
	if certificate == nil {
		return nil, errors.New("HTTPS certificate is not loaded")
	}

	return certificate, nil
}

// GetConfigForClient returns a shallow clone carrying the current client CA pool.
// ClientCAs on the serving tls.Config must not be mutated; this hook is the safe path.
func (r *CertificateReloader) GetConfigForClient(*tls.ClientHelloInfo) (*tls.Config, error) {
	base := r.baseTLS.Load()
	if base == nil {
		return nil, errors.New("HTTPS TLS configuration is not bound")
	}

	pool := r.clientCAs.Load()
	if pool == nil {
		return nil, errors.New("HTTPS client CA pool is not loaded")
	}

	// The clone is per-handshake and must not recurse back into this hook.
	clone := base.Clone()
	clone.ClientCAs = pool
	clone.GetConfigForClient = nil

	return clone, nil
}

// Start polls the watched TLS files until ctx is canceled.
func (r *CertificateReloader) Start(ctx context.Context) {
	for _, watcher := range r.watchers {
		watcher.Start(ctx)
	}
}
