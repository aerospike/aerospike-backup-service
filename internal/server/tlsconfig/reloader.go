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
	// Load reads the key pair and, when configured, the client CA pool and CRLs.
	// The first call must succeed; later failures keep the last good material.
	Load(ctx context.Context) error
	// Start watches the TLS files until ctx is canceled.
	Start(ctx context.Context)
	// GetCertificate returns the server key pair for a TLS handshake.
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
	// ClientAuth returns the current client CA pool and optional revocation verifier.
	ClientAuth() (*x509.CertPool, ClientCertificateVerifier, error)
}

type clientAuthState struct {
	clientCAs *x509.CertPool
	crls      *crlIndex
}

// certificateReloader is the single point that reads HTTPS TLS material from disk.
// It swaps the key pair when cert-file or key-file change and, when client-ca-file
// is set, swaps the mTLS trust pool and optional CRL index served through GetConfigForClient.
type certificateReloader struct {
	config     *model.ServerConfigHTTPS
	resolver   secrets.Resolver
	watchers   []reload.Watcher
	current    atomic.Pointer[tls.Certificate]
	clientAuth atomic.Pointer[clientAuthState]
	mu         sync.Mutex
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
	if config.CRLFile != "" {
		reloader.watchers = append(reloader.watchers,
			reload.New(config.CRLFile, interval, reloader.loadCRLs),
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

func (noOpReloader) ClientAuth() (*x509.CertPool, ClientCertificateVerifier, error) {
	return nil, nil, errors.New("HTTPS client CA pool is not loaded")
}

// Load reads the key pair, client CA pool, and CRLs and serves them to new handshakes.
func (r *certificateReloader) Load(ctx context.Context) error {
	if err := r.loadKeyPair(ctx); err != nil {
		return err
	}
	if err := r.loadClientCAs(ctx); err != nil {
		return err
	}

	return r.loadCRLs(ctx)
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
	if current := r.clientAuth.Load(); current != nil && current.clientCAs != nil {
		message = "rotated HTTPS client CA pool"
	}
	r.storeClientAuth(pool, r.currentCRLs())
	slog.Info(message, slog.String("clientCaFile", r.config.ClientCAFile))

	return nil
}

func (r *certificateReloader) loadCRLs(_ context.Context) error {
	if r.config.CRLFile == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	index, err := loadCRLs(r.config.CRLFile)
	if err != nil {
		return err
	}
	index.logStaleIfNeeded(time.Now())

	message := "loaded HTTPS client CRL"
	if current := r.clientAuth.Load(); current != nil && current.crls != nil {
		message = "rotated HTTPS client CRL"
	}
	r.storeClientAuth(r.currentClientCAs(), index)
	slog.Info(message, slog.String("crlFile", r.config.CRLFile))

	return nil
}

func (r *certificateReloader) currentClientCAs() *x509.CertPool {
	if current := r.clientAuth.Load(); current != nil {
		return current.clientCAs
	}

	return nil
}

func (r *certificateReloader) currentCRLs() *crlIndex {
	if current := r.clientAuth.Load(); current != nil {
		return current.crls
	}

	return nil
}

func (r *certificateReloader) storeClientAuth(pool *x509.CertPool, crls *crlIndex) {
	r.clientAuth.Store(&clientAuthState{clientCAs: pool, crls: crls})
}

// GetCertificate returns the currently loaded key pair, for tls.Config.GetCertificate.
func (r *certificateReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certificate := r.current.Load()
	if certificate == nil {
		return nil, errors.New("HTTPS certificate is not loaded")
	}

	return certificate, nil
}

// ClientAuth returns the current immutable client CA pool and revocation verifier.
func (r *certificateReloader) ClientAuth() (*x509.CertPool, ClientCertificateVerifier, error) {
	state := r.clientAuth.Load()
	if state == nil || state.clientCAs == nil {
		return nil, nil, errors.New("HTTPS client CA pool is not loaded")
	}

	return state.clientCAs, state.crls, nil
}

// Start polls the watched TLS files until ctx is canceled.
func (r *certificateReloader) Start(ctx context.Context) {
	for _, watcher := range r.watchers {
		watcher.Start(ctx)
	}
}
