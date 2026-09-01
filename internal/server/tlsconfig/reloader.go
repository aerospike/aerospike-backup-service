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

// TLSProvider loads, watches, and supplies TLS material for HTTPS handshakes.
type TLSProvider interface {
	// Load reads the key pair and, when configured, the client CA pool.
	// The first call must succeed; later failures keep the last good material.
	Load(ctx context.Context) error
	// Start watches the TLS files until ctx is canceled.
	Start(ctx context.Context)
	// GetCertificate returns the server key pair for a TLS handshake.
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
	// ClientCAs returns the current immutable client CA pool.
	ClientCAs() (*x509.CertPool, error)
}

// certificateReloader is the single point that reads HTTPS TLS material from disk.
// It swaps the key pair when cert-file or key-file change and, when client-ca-file
// is set, swaps the mTLS trust pool served through GetConfigForClient.
type certificateReloader struct {
	config    *model.ServerConfigHTTPS
	resolver  secrets.Resolver
	watchers  []reload.Watcher
	current   atomic.Pointer[tls.Certificate]
	clientCAs atomic.Pointer[x509.CertPool]
	mu        sync.Mutex
}

var (
	_ TLSProvider = (*certificateReloader)(nil)
	_ TLSProvider = noOpReloader{}
)

// NewCertificateProvider returns a TLS provider that watches the configured files.
// It performs no I/O beyond fingerprinting them: call Load for the initial material
// and Start to watch for changes.
func NewCertificateProvider(
	config *model.ServerConfigHTTPS,
	resolver secrets.Resolver,
) TLSProvider {
	return newCertificateProvider(config, resolver, WatchInterval)
}

func newCertificateProvider(
	config *model.ServerConfigHTTPS,
	resolver secrets.Resolver,
	interval time.Duration,
) TLSProvider {
	reloader := &certificateReloader{
		config:   config,
		resolver: resolver,
	}
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

// NoReload returns a reloader that does nothing, for listeners that serve plaintext.
func NoReload() TLSProvider {
	return noOpReloader{}
}

type noOpReloader struct{}

func (noOpReloader) Load(context.Context) error { return nil }

func (noOpReloader) Start(context.Context) {}

func (noOpReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, errors.New("HTTPS certificate is not loaded")
}

func (noOpReloader) ClientCAs() (*x509.CertPool, error) {
	return nil, errors.New("HTTPS client CA pool is not loaded")
}

// Load reads the key pair and the client CA pool and serves them to new handshakes.
func (r *certificateReloader) Load(ctx context.Context) error {
	if err := r.loadKeyPair(ctx); err != nil {
		return err
	}

	return r.loadClientCAs(ctx)
}

func (r *certificateReloader) loadKeyPair(ctx context.Context) error {
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

func (r *certificateReloader) loadClientCAs(_ context.Context) error {
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

// GetCertificate returns the currently loaded key pair, for tls.Config.GetCertificate.
func (r *certificateReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certificate := r.current.Load()
	if certificate == nil {
		return nil, errors.New("HTTPS certificate is not loaded")
	}

	return certificate, nil
}

// ClientCAs returns the current immutable client CA pool.
func (r *certificateReloader) ClientCAs() (*x509.CertPool, error) {
	pool := r.clientCAs.Load()
	if pool == nil {
		return nil, errors.New("HTTPS client CA pool is not loaded")
	}

	return pool, nil
}

// Start polls the watched TLS files until ctx is canceled.
func (r *certificateReloader) Start(ctx context.Context) {
	for _, watcher := range r.watchers {
		watcher.Start(ctx)
	}
}
