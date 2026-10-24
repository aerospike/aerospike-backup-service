//go:build integration

package integration

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// keyPassword unlocks every encrypted private key this package writes. Tests hand it
// to ABS as key-file-password, literally or through Secret Agent.
const keyPassword = "client-key-pass"

const pemTypeCertificate = "CERTIFICATE"

// clusterCertificates is the PKI for the secured Aerospike nodes: the CA both sides
// trust, the server certificate, and one client certificate per test user.
type clusterCertificates struct {
	// CAFile is the CA as a single PEM file (dto.TLS.ca-file).
	CAFile string
	// CADir is a directory holding that same PEM (dto.TLS.ca-path). ca-file and
	// ca-path are mutually exclusive, so a cluster config picks one.
	CADir string
	// Internal authenticates as intUser. Its CN only matters to the TLS layer.
	Internal clientCertificate
	// PKI authenticates as pkiUser: with auth-mode PKI the server takes the
	// username from the certificate CN rather than from credentials.
	PKI clientCertificate

	// serverCert and serverKey are copied into the container, never read by ABS.
	serverCert []byte
	serverKey  []byte
}

// clientCertificate is a client certificate and its key, in the files ABS reads.
type clientCertificate struct {
	CertFile string
	// EncryptedKeyFile is what ABS loads as dto.TLS.key-file; keyPassword unlocks it.
	EncryptedKeyFile string
	// keyFile is the same key unencrypted, for the TLS readiness probe.
	keyFile string
}

// httpsCertificates is the PKI for the ABS HTTPS listener.
type httpsCertificates struct {
	// CAFile is what the test's HTTP client trusts.
	CAFile string
	// CertFile is valid for localhost and 127.0.0.1.
	CertFile string
	// EncryptedKeyFile is what ABS loads as key-file; keyPassword unlocks it.
	EncryptedKeyFile string
}

// certificateAuthority signs the certificates of one PKI.
type certificateAuthority struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
	pem  []byte
}

func (s *Suite) generateClusterCertificates() clusterCertificates {
	dir := s.T().TempDir()
	ca := s.newCA("ABS integration CA")

	caFile := s.writeFile(dir, "ca.crt", ca.pem)
	caDir := filepath.Join(dir, "ca")
	s.Require().NoError(os.Mkdir(caDir, 0o700))
	s.writeFile(caDir, "ca.crt", ca.pem)

	// The client checks the DNS SAN, not the common name, so tlsName has to be there.
	serverCert, serverKey := s.issue(ca, x509.Certificate{
		Subject:     pkix.Name{CommonName: tlsName},
		DNSNames:    []string{tlsName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})

	return clusterCertificates{
		CAFile:     caFile,
		CADir:      caDir,
		Internal:   s.issueClientCertificate(ca, dir, intUser),
		PKI:        s.issueClientCertificate(ca, dir, pkiUser),
		serverCert: serverCert,
		serverKey:  serverKey,
	}
}

// generateHTTPSCertificates mints a CA and a server certificate for the ABS HTTPS
// listener, with the private key encrypted by keyPassword.
func (s *Suite) generateHTTPSCertificates() httpsCertificates {
	dir := s.T().TempDir()
	ca := s.newCA("ABS HTTPS test CA")

	cert, key := s.issue(ca, x509.Certificate{
		Subject:     pkix.Name{CommonName: "localhost"},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})

	return httpsCertificates{
		CAFile:           s.writeFile(dir, "ca.pem", ca.pem),
		CertFile:         s.writeFile(dir, "server.pem", cert),
		EncryptedKeyFile: s.writeFile(dir, "server-key.enc", s.encryptPEMKey(key)),
	}
}

// encryptionKeyPEM returns a fresh RSA private key for dto.EncryptionPolicy.
func (s *Suite) encryptionKeyPEM() string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	s.Require().NoError(err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func (s *Suite) issueClientCertificate(ca certificateAuthority, dir, user string) clientCertificate {
	cert, key := s.issue(ca, x509.Certificate{
		Subject:     pkix.Name{CommonName: user},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

	return clientCertificate{
		CertFile:         s.writeFile(dir, user+".crt", cert),
		EncryptedKeyFile: s.writeFile(dir, user+".enc.key", s.encryptPEMKey(key)),
		keyFile:          s.writeFile(dir, user+".key", key),
	}
}

func (s *Suite) newCA(commonName string) certificateAuthority {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	s.Require().NoError(err)

	cert, err := x509.ParseCertificate(der)
	s.Require().NoError(err)

	return certificateAuthority{
		cert: cert,
		key:  key,
		pem:  pem.EncodeToMemory(&pem.Block{Type: pemTypeCertificate, Bytes: der}),
	}
}

// issue signs a leaf certificate with the names and usages from template and a
// fresh key, and returns both as PEM.
func (s *Suite) issue(ca certificateAuthority, template x509.Certificate) (certPEM, keyPEM []byte) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	s.Require().NoError(err)

	template.SerialNumber = serial
	template.NotBefore = time.Now().Add(-time.Hour)
	template.NotAfter = time.Now().Add(24 * time.Hour)
	template.KeyUsage = x509.KeyUsageDigitalSignature

	der, err := x509.CreateCertificate(rand.Reader, &template, ca.cert, &key.PublicKey, ca.key)
	s.Require().NoError(err)

	return pem.EncodeToMemory(&pem.Block{Type: pemTypeCertificate, Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

// encryptPEMKey wraps a PKCS#1 key in a keyPassword-protected DEK-Info PEM block,
// the legacy encryption the Go stdlib still reads and ABS decrypts.
func (s *Suite) encryptPEMKey(keyPEM []byte) []byte {
	block, _ := pem.Decode(keyPEM)
	s.Require().NotNil(block, "private key PEM")

	//noinspection GoDeprecation
	encrypted, err := x509.EncryptPEMBlock( //nolint:staticcheck // DEK-Info PEM is what ABS decrypts
		rand.Reader, block.Type, block.Bytes, []byte(keyPassword), x509.PEMCipherAES256)
	s.Require().NoError(err)

	return pem.EncodeToMemory(encrypted)
}

func (s *Suite) writeFile(dir, name string, content []byte) string {
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, content, 0o600))

	return path
}
