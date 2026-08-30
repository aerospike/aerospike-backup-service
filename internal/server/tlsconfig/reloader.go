package tlsconfig

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/reload"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
)

// DefaultWatchInterval is how often cert-file and key-file mtimes are polled.
const DefaultWatchInterval = 10 * time.Second

// CertificateReloader serves the HTTPS key pair and swaps it when cert-file or key-file change.
type CertificateReloader struct {
	config   *model.ServerConfigHTTPS
	resolver secrets.Resolver
	watchers []*reload.Watcher
	current  atomic.Pointer[tls.Certificate]
}

// NewCertificateReloader returns a reloader for the configured key pair.
// It performs no I/O: call Load for the initial key pair and Start to watch for changes.
func NewCertificateReloader(
	config *model.ServerConfigHTTPS,
	resolver secrets.Resolver,
	interval time.Duration,
) *CertificateReloader {
	reloader := &CertificateReloader{
		config:   config,
		resolver: resolver,
	}
	reloader.watchers = []*reload.Watcher{
		reload.New(config.CertFile, interval, reloader.Load),
		reload.New(config.KeyFile, interval, reloader.Load),
	}

	return reloader
}

// NoReload returns a reloader without a key pair, for listeners that serve plaintext.
func NoReload() *CertificateReloader {
	return &CertificateReloader{}
}

// Load reads the configured key pair and serves it to new handshakes.
// The caller's initial Load reports a broken pair so startup can fail fast. Later calls come
// from the watchers, where a failure leaves the last successfully loaded pair in place.
func (r *CertificateReloader) Load(ctx context.Context) error {
	password, err := r.resolver.Resolve(ctx, r.config.SecretAgent, r.config.KeyFilePassword)
	if err != nil {
		return fmt.Errorf("failed to resolve HTTPS key-file-password: %w", err)
	}

	certificate, err := loadKeyPair(r.config.CertFile, r.config.KeyFile, password)
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
