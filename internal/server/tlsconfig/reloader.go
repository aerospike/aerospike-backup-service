package tlsconfig

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/reload"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
)

// DefaultWatchInterval is how often cert-file and key-file mtimes are polled.
const DefaultWatchInterval = 10 * time.Second

// Reloader serves the HTTPS key pair and can watch it for rotation.
type Reloader interface {
	// Load reads the key pair from disk. The first call must succeed; later failures keep the last good pair.
	Load(ctx context.Context) error
	// Start watches cert-file and key-file until ctx is canceled.
	Start(ctx context.Context)
	// GetCertificate returns the currently loaded key pair.
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
}

var _ Reloader = (*CertificateReloader)(nil)
var _ Reloader = noOpReloader{}

// CertificateReloader serves the HTTPS key pair and swaps it when cert-file or key-file change.
type CertificateReloader struct {
	config   *model.ServerConfigHTTPS
	resolver secrets.Resolver
	watchers []reload.Watcher
	current  atomic.Pointer[tls.Certificate]
	mu       sync.Mutex
}

// NewCertificateReloader returns a reloader for the configured key pair.
// It performs no I/O beyond fingerprinting the watched files: call Load for the
// initial key pair and Start to watch for changes.
func NewCertificateReloader(
	config *model.ServerConfigHTTPS,
	resolver secrets.Resolver,
	interval time.Duration,
) *CertificateReloader {
	reloader := &CertificateReloader{
		config:   config,
		resolver: resolver,
	}
	// The unit of serving is a matched tls.Certificate (cert PEM + key), not two
	// independent files. Reloading a half would handshake with new-cert/old-key
	// (or the reverse). Either path changing therefore re-reads both files via Load.
	reloader.watchers = []reload.Watcher{
		reload.New(config.CertFile, interval, reloader.Load),
		reload.New(config.KeyFile, interval, reloader.Load),
	}

	return reloader
}

// NoReload returns a reloader that does nothing, for listeners that serve plaintext.
func NoReload() Reloader {
	return noOpReloader{}
}

type noOpReloader struct{}

func (noOpReloader) Load(context.Context) error { return nil }

func (noOpReloader) Start(context.Context) {}

func (noOpReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, errors.New("HTTPS certificate is not loaded")
}

// Load reads the configured key pair and serves it to new handshakes.
func (r *CertificateReloader) Load(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// A mid-rewrite mismatch fails the parse; that does not Store, so the last good pair
	// stays in service until both files load together.
	certificate, err := LoadKeyPair(ctx, r.config, r.resolver)
	if err != nil {
		return err
	}

	r.current.Store(&certificate)

	return nil
}

// GetCertificate returns the currently loaded key pair, for tls.Config.GetCertificate.
func (r *CertificateReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certificate := r.current.Load()
	if certificate == nil {
		return nil, errors.New("HTTPS certificate is not loaded")
	}

	return certificate, nil
}

// Start polls cert-file and key-file until ctx is canceled.
func (r *CertificateReloader) Start(ctx context.Context) {
	for _, watcher := range r.watchers {
		watcher.Start(ctx)
	}
}

// LoadKeyPair resolves the key-file password and loads the certificate and key from disk.
func LoadKeyPair(
	ctx context.Context,
	config *model.ServerConfigHTTPS,
	resolver secrets.Resolver,
) (tls.Certificate, error) {
	password, err := resolver.Resolve(ctx, config.SecretAgent, config.KeyFilePassword)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to resolve HTTPS key-file-password: %w", err)
	}

	return loadKeyPair(config.CertFile, config.KeyFile, password)
}
