package tlsconfig

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

const pemCRLType = "X509 CRL"

var (
	errCertificateRevoked = errors.New("client certificate is revoked")
	errCRLExpired         = errors.New("client certificate CRL is expired")
	errCRLNotYetValid     = errors.New("client certificate CRL is not yet valid")
	errCRLNotFound        = errors.New("no CRL for client certificate issuer")
	errNoVerifiedChain    = errors.New("client certificate chain was not verified")
)

type indexedCRL struct {
	list           *x509.RevocationList
	revokedSerials map[string]struct{}
}

type crlIndex struct {
	byRawIssuer     map[string][]*indexedCRL
	expiredOnce     atomic.Bool
	notYetValidOnce atomic.Bool
}

func loadCRLs(path string) (*crlIndex, error) {
	data, err := safepath.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read client CRL file: %w", err)
	}

	lists, err := parseCRLs(data, path)
	if err != nil {
		return nil, err
	}

	index := &crlIndex{byRawIssuer: make(map[string][]*indexedCRL, len(lists))}
	for _, list := range lists {
		index.byRawIssuer[string(list.RawIssuer)] = append(
			index.byRawIssuer[string(list.RawIssuer)],
			indexCRL(list),
		)
	}

	return index, nil
}

// parseCRLs reads one DER-encoded CRL or a PEM file of one or more CRLs. A file with a PEM
// block anywhere in it is PEM; anything else is DER and is passed through untrimmed, because
// whitespace bytes can be part of the encoding.
func parseCRLs(data []byte, path string) ([]*x509.RevocationList, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("client CRL file %q contains no CRLs", path)
	}

	ders, err := decodePEMBlocks(data, pemCRLType)
	if err != nil {
		return nil, fmt.Errorf("client CRL file %q: %w", path, err)
	}
	if len(ders) == 0 {
		ders = [][]byte{data}
	}

	lists := make([]*x509.RevocationList, 0, len(ders))
	for _, der := range ders {
		list, parseErr := x509.ParseRevocationList(der)
		if parseErr != nil {
			return nil, fmt.Errorf("client CRL file %q contains an invalid CRL: %w", path, parseErr)
		}
		lists = append(lists, list)
	}

	return lists, nil
}

func indexCRL(list *x509.RevocationList) *indexedCRL {
	revoked := make(map[string]struct{}, len(list.RevokedCertificateEntries))
	for _, entry := range list.RevokedCertificateEntries {
		revoked[serialKey(entry.SerialNumber)] = struct{}{}
	}

	return &indexedCRL{list: list, revokedSerials: revoked}
}

func serialKey(serial *big.Int) string {
	if serial == nil {
		return ""
	}

	return serial.String()
}

// ClientCertificateVerifier validates an already verified client TLS connection.
type ClientCertificateVerifier interface {
	// Verify rejects a TLS connection that violates the verifier's policy.
	Verify(tls.ConnectionState) error
}

var _ ClientCertificateVerifier = (*crlIndex)(nil)

// Verify rejects client leaf certificates that are revoked by this CRL index.
func (idx *crlIndex) Verify(state tls.ConnectionState) error {
	return idx.verifyClientLeaf(state, time.Now())
}

func (idx *crlIndex) verifyClientLeaf(state tls.ConnectionState, now time.Time) error {
	if len(state.VerifiedChains) == 0 {
		return errNoVerifiedChain
	}

	var last error
	for _, chain := range state.VerifiedChains {
		err := idx.verifyLeafAgainstChain(chain, now)
		if err == nil {
			return nil
		}
		last = err
	}

	return last
}

func (idx *crlIndex) verifyLeafAgainstChain(chain []*x509.Certificate, now time.Time) error {
	if len(chain) < 2 {
		return errCRLNotFound
	}

	// Verify every certificate in the chain except the root trust anchor (chain[len(chain)-1])
	for i := 0; i < len(chain)-1; i++ {
		cert := chain[i]
		issuer := chain[i+1]

		candidates := idx.byRawIssuer[string(cert.RawIssuer)]
		chosen := selectIssuerCRL(candidates, issuer)
		if chosen == nil {
			return fmt.Errorf("%w: issuer %q", errCRLNotFound, issuer.Subject.String())
		}

		if err := crlFreshness(chosen.list, now); err != nil {
			idx.logStale(err)
			return err
		}

		if _, revoked := chosen.revokedSerials[serialKey(cert.SerialNumber)]; revoked {
			return fmt.Errorf("%w: serial %s", errCertificateRevoked, cert.SerialNumber)
		}
	}

	return nil
}

func (idx *crlIndex) logStale(err error) {
	switch {
	case errors.Is(err, errCRLExpired) && idx.expiredOnce.CompareAndSwap(false, true):
		slog.Error("HTTPS client CRL is expired; rejecting mTLS clients until a current CRL is loaded")
	case errors.Is(err, errCRLNotYetValid) && idx.notYetValidOnce.CompareAndSwap(false, true):
		slog.Error("HTTPS client CRL is not yet valid; rejecting mTLS clients until a current CRL is loaded")
	}
}

func (idx *crlIndex) logStaleIfNeeded(now time.Time) {
	for _, lists := range idx.byRawIssuer {
		for _, candidate := range lists {
			if err := crlFreshness(candidate.list, now); err != nil {
				idx.logStale(err)
				return
			}
		}
	}
}

func selectIssuerCRL(candidates []*indexedCRL, issuer *x509.Certificate) *indexedCRL {
	var chosen *indexedCRL
	for _, candidate := range candidates {
		if candidate.list.CheckSignatureFrom(issuer) != nil {
			continue
		}
		if chosen == nil || crlPreferred(candidate.list, chosen.list) {
			chosen = candidate
		}
	}

	return chosen
}

func crlPreferred(a, b *x509.RevocationList) bool {
	if a.Number != nil && b.Number != nil {
		if cmp := a.Number.Cmp(b.Number); cmp != 0 {
			return cmp > 0
		}
	}

	return a.ThisUpdate.After(b.ThisUpdate)
}

func crlFreshness(list *x509.RevocationList, now time.Time) error {
	if list.ThisUpdate.After(now) {
		return errCRLNotYetValid
	}
	if list.NextUpdate.IsZero() || !now.Before(list.NextUpdate) {
		return errCRLExpired
	}

	return nil
}
