package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"
)

// ClientCertificateVerifier validates an already verified client TLS connection.
type ClientCertificateVerifier interface {
	// Verify rejects a TLS connection that violates the verifier's policy.
	Verify(tls.ConnectionState) error
}

// Verify rejects client leaf certificates that are revoked by this CRL index.
func (i *crlIndex) Verify(state tls.ConnectionState) error {
	return verifyClientLeaf(state, i, time.Now())
}

func verifyClientLeaf(state tls.ConnectionState, index *crlIndex, now time.Time) error {
	if len(state.VerifiedChains) == 0 {
		return errNoVerifiedChain
	}

	var last error
	for _, chain := range state.VerifiedChains {
		err := verifyLeafAgainstChain(chain, index, now)
		if err == nil {
			return nil
		}
		last = err
	}

	return last
}

func verifyLeafAgainstChain(chain []*x509.Certificate, index *crlIndex, now time.Time) error {
	if len(chain) < 2 {
		return errCRLNotFound
	}

	leaf := chain[0]
	issuer := chain[1]
	candidates := index.byRawIssuer[string(leaf.RawIssuer)]
	chosen := selectIssuerCRL(candidates, issuer)
	if chosen == nil {
		return fmt.Errorf("%w: issuer %q", errCRLNotFound, issuer.Subject.String())
	}

	if err := crlFreshness(chosen.list, now); err != nil {
		index.logStale(err)
		return err
	}

	if _, revoked := chosen.revokedSerials[serialKey(leaf.SerialNumber)]; revoked {
		return fmt.Errorf("%w: serial %s", errCertificateRevoked, leaf.SerialNumber)
	}

	return nil
}
