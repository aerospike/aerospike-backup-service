package tlsconfig

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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

func parseCRLs(data []byte, path string) ([]*x509.RevocationList, error) {
	remaining := bytes.TrimSpace(data)
	if len(remaining) == 0 {
		return nil, fmt.Errorf("client CRL file %q contains no CRLs", path)
	}

	if bytes.HasPrefix(remaining, []byte("-----BEGIN ")) {
		return parsePEMCRLs(remaining, path)
	}

	list, err := x509.ParseRevocationList(remaining)
	if err != nil {
		return nil, fmt.Errorf("client CRL file %q contains an invalid CRL: %w", path, err)
	}

	return []*x509.RevocationList{list}, nil
}

func parsePEMCRLs(data []byte, path string) ([]*x509.RevocationList, error) {
	var lists []*x509.RevocationList
	remaining := data
	for {
		remaining = bytes.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}
		if !bytes.HasPrefix(remaining, []byte("-----BEGIN "+pemCRLType+"-----")) {
			if len(lists) == 0 {
				return nil, fmt.Errorf("client CRL file %q contains no CRLs", path)
			}

			return nil, fmt.Errorf("client CRL file %q contains invalid data after a CRL", path)
		}

		block, rest := pem.Decode(remaining)
		if block == nil {
			return nil, fmt.Errorf("client CRL file %q contains an invalid CRL PEM block", path)
		}
		list, err := x509.ParseRevocationList(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("client CRL file %q contains an invalid CRL: %w", path, err)
		}
		lists = append(lists, list)
		remaining = rest
	}
	if len(lists) == 0 {
		return nil, fmt.Errorf("client CRL file %q contains no CRLs", path)
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
