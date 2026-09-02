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

	// Verify every certificate in the chain except the root trust anchor (chain[len(chain)-1])
	for i := 0; i < len(chain)-1; i++ {
		cert := chain[i]
		issuer := chain[i+1]

		candidates := index.byRawIssuer[string(cert.RawIssuer)]
		chosen := selectIssuerCRL(candidates, issuer)
		if chosen == nil {
			return fmt.Errorf("%w: issuer %q", errCRLNotFound, issuer.Subject.String())
		}

		if err := crlFreshness(chosen.list, now); err != nil {
			index.logStale(err)
			return err
		}

		if _, revoked := chosen.revokedSerials[serialKey(cert.SerialNumber)]; revoked {
			return fmt.Errorf("%w: serial %s", errCertificateRevoked, cert.SerialNumber)
		}
	}

	return nil
}
