//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tcAerospike "github.com/testcontainers/testcontainers-go/modules/aerospike"
)

const (
	containerConfPath = "/etc/aerospike/aerospike.conf"
	tlsPort           = "4333/tcp"
	// tlsName is both the Aerospike `tls` stanza name and the DNS SAN of the server
	// certificate. The client sends it as SNI and then verifies it against the cert,
	// so the two must stay in sync.
	tlsName = "test-tls"
	// clientKeyPassword unlocks the encrypted client key PEMs used in mTLS tests.
	clientKeyPassword = "client-key-pass"
	// tlsProtocols pins the handshake to TLS 1.2. That is the only version ABS
	// currently maps (see pkg/tlsconfig); a single token is both min and max.
	tlsProtocols = "TLSv1.2"
	// tlsCipherSuite is an IANA name, colon-separated if there are several.
	// These tests mint RSA certificates, so the suite must be ECDHE_RSA, not ECDSA.
	tlsCipherSuite = "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"

	adminUser     = "admin"
	adminPassword = "admin"
	intUser       = "intuser"
	intPassword   = "s3cr3t!"
	pkiUser       = "pkiuser"
	readWriteRole = "read-write"

	containerCAFile     = "/etc/aerospike/ca.crt"
	containerServerCert = "/etc/aerospike/server.crt"
	containerServerKey  = "/etc/aerospike/server.key"
)

// secureConf is the stock EE docker conf plus an empty security stanza, which is all it
// takes to turn RBAC on. The server then starts with the default admin/admin superuser.
const secureConf = `service {
	feature-key-file /etc/aerospike/features.conf
	cluster-name docker
}

logging {
	console {
		context any info
	}
}

network {
	service {
		address any
		port 3000
	}

	heartbeat {
		mode mesh
		address local
		port 3002
		interval 150
		timeout 10
	}

	fabric {
		address local
		port 3001
	}
}

security {
}

namespace test {
	replication-factor 1
	nsup-period 120
	storage-engine device {
		file /opt/aerospike/data/test.dat
		filesize 4G
		read-page-cache true
	}
}
`

// authCluster is one running secured node.
type authCluster struct {
	// seed is what ABS is pointed at, and carries the profile under test: the TLS port
	// and tls-name for the TLS profiles.
	seed dto.SeedNode
	// adminSeed is always the plaintext service port. User and role management is done
	// out of band so that bootstrapping never depends on the transport under test.
	adminSeed dto.SeedNode
	// adminClient is logged in as the superuser over adminSeed.
	adminClient *as.Client
}

// certificateFiles is a throwaway PKI generated per suite run.
type certificateFiles struct {
	// caCert is the CA bundle as a single PEM file (dto.TLS.ca-file).
	caCert string
	// caDir is a directory containing that same CA PEM (dto.TLS.ca-path).
	// ca-file and ca-path are mutually exclusive; tests pick one per cluster config.
	caDir string
	// serverCert and serverKey stay in memory because they are copied straight into the
	// container rather than read by the client.
	serverCert []byte
	serverKey  []byte
	// The client certificate common names are Aerospike usernames on purpose: with
	// auth-mode PKI the server takes the username from the CN rather than from the
	// connection credentials.
	internalCert string
	// internalKey is the unencrypted private key, used only by waitForTLS.
	internalKey string
	// Encrypted keys are what ABS loads via dto.TLS.key-file + key-file-password.
	internalKeyEncrypted string
	pkiCert              string
	pkiKeyEncrypted      string
}

// authProfile is the transport and client-authentication setup of a secured node.
//
// Each profile needs its own server process: plain and TLS differ in which ports are
// open, and the two TLS profiles differ in tls-authenticate-client.
type authProfile int

const (
	// profilePlain serves the plaintext port only.
	profilePlain authProfile = iota
	// profileServerTLS presents a server certificate and does not ask for a client one.
	profileServerTLS
	// profileMutualTLS additionally requires a client certificate, which is what PKI
	// authentication needs to identify the user.
	profileMutualTLS
)

func (p authProfile) usesTLS() bool {
	return p != profilePlain
}

func (p authProfile) requiresClientCert() bool {
	return p == profileMutualTLS
}

// AuthSuite runs connection smoke tests against secured EE nodes.
//
// Since no two profiles can share a server process, each Test* method starts the node
// it needs and testcontainers cleanup stops it when that test returns, keeping only one
// secured container alive at a time.
type AuthSuite struct {
	Suite

	certs certificateFiles
}

func (s *AuthSuite) SetupSuite() {
	s.certs = s.generateCertificates()
}

// SetupTest skips the shared namespace truncate: AuthSuite has no suite-wide
// client, and each scenario truncates the node it just started.
func (s *AuthSuite) SetupTest() {}

// startAndProvision boots a node for one profile and creates the users the tests log in as.
func (s *AuthSuite) startAndProvision(profile authProfile) authCluster {
	t := s.T()
	// Not t.Context(): it is canceled before t.Cleanup runs, which would make the
	// terminate call below fail and leave the container behind.
	cluster := s.startSecureAerospike(context.Background(), profile)

	admin := cluster.adminClient
	s.Require().NoError(admin.CreateUser(nil, intUser, intPassword, []string{readWriteRole}))
	s.Require().NoError(admin.GrantRoles(
		nil, adminUser, []string{"sys-admin", "truncate", readWriteRole},
	))
	if profile.requiresClientCert() {
		s.Require().NoError(admin.CreatePKIUser(nil, pkiUser, []string{readWriteRole}))
	}

	// Roles are baked into the session token at login, so the admin client that granted
	// them still holds a token without truncate. Reconnect to pick the new roles up.
	admin.Close()
	admin, err := newAdminClient(cluster.adminSeed)
	s.Require().NoError(err)
	t.Cleanup(admin.Close)
	cluster.adminClient = admin

	return cluster
}

// startSecureAerospike brings up a single node with security enabled.
//
// The image entrypoint rewrites /etc/aerospike/aerospike.conf every time it boots a
// fresh container, so the config cannot simply be mounted. The sequence is: let the
// image start normally, stop it, copy in the config (and certificates), start it again.
//
//nolint:funlen // Container lifecycle is clearer when setup remains in one place.
func (s *AuthSuite) startSecureAerospike(ctx context.Context, profile authProfile) authCluster {
	t := s.T()

	var options []testcontainers.ContainerCustomizer
	if profile.usesTLS() {
		// Docker Desktop hands out a new host port whenever a container is stopped and
		// started, but the port has to be baked into tls-alternate-access-port before the
		// restart. Pinning the binding at create time keeps it stable across the restart.
		mappedTLSPort := availableHostPort(ctx, s)
		options = append(
			options,
			testcontainers.WithExposedPorts(tlsPort),
			testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
				if hostConfig.PortBindings == nil {
					hostConfig.PortBindings = network.PortMap{}
				}
				hostConfig.PortBindings[network.MustParsePort(tlsPort)] = []network.PortBinding{{
					HostIP:   netip.MustParseAddr("127.0.0.1"),
					HostPort: strconv.Itoa(mappedTLSPort),
				}}
			}),
		)
	}

	asContainer, err := tcAerospike.Run(ctx, aerospikeImage, options...)
	s.Require().NoError(err)
	t.Cleanup(func() {
		if err := asContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate Aerospike container: %v", err)
		}
	})

	host, err := asContainer.Host(ctx)
	s.Require().NoError(err)

	var mappedTLSPort int
	if profile.usesTLS() {
		mapped, mapErr := asContainer.MappedPort(ctx, tlsPort)
		s.Require().NoError(mapErr)
		mappedTLSPort = int(mapped.Num())
	}

	stopTimeout := 10 * time.Second
	s.Require().NoError(asContainer.Stop(ctx, &stopTimeout))

	config := secureConf
	if profile.usesTLS() {
		config = tlsSecureConf(host, mappedTLSPort, profile)
		s.copyServerCertificates(ctx, asContainer)
	}
	s.Require().NoError(asContainer.CopyToContainer(ctx, []byte(config), containerConfPath, 0o644))
	s.Require().NoError(asContainer.Start(ctx))

	// Poll real connections.
	plainPort, err := asContainer.MappedPort(ctx, "3000/tcp")
	s.Require().NoError(err)
	adminSeed := dto.SeedNode{HostName: host, Port: dto.Port(plainPort.Num())}
	s.Require().NoError(waitForDBStart(ctx, adminSeed))

	seed := adminSeed
	if profile.usesTLS() {
		mapped, mapErr := asContainer.MappedPort(ctx, tlsPort)
		s.Require().NoError(mapErr)
		s.Equal(mappedTLSPort, int(mapped.Num()), "TLS port mapping changed after restart")
		s.Require().NoError(s.waitForTLS(ctx, host, mappedTLSPort, profile))
		seed = dto.SeedNode{
			HostName: host,
			Port:     dto.Port(mapped.Num()),
			TLSName:  tlsName,
		}
	}

	admin, err := newAdminClient(adminSeed)
	s.Require().NoError(err)

	return authCluster{seed: seed, adminSeed: adminSeed, adminClient: admin}
}

// tlsSecureConf keeps the plaintext service port for bootstrapping and adds a TLS port.
//
// The tls-alternate-access-* settings are what the node gossips back to clients; without
// them it would advertise its in-container address, which is unreachable from the host.
//
//nolint:funlen // The embedded Aerospike configuration is intentionally kept together.
func tlsSecureConf(host string, mappedTLSPort int, profile authProfile) string {
	clientAuthentication := "false"
	if profile.requiresClientCert() {
		clientAuthentication = "any"
	}

	return fmt.Sprintf(`service {
	feature-key-file /etc/aerospike/features.conf
	cluster-name docker
}

logging {
	console {
		context any info
	}
}

network {
	tls %[1]s {
		ca-file %[2]s
		cert-file %[3]s
		key-file %[4]s
	}

	service {
		address any
		port 3000
		tls-address any
		tls-port 4333
		tls-name %[1]s
		tls-authenticate-client %[6]s
		tls-alternate-access-address %[5]s
		tls-alternate-access-port %[7]d
	}

	heartbeat {
		mode mesh
		address local
		port 3002
		interval 150
		timeout 10
	}

	fabric {
		address local
		port 3001
	}
}

security {
}

namespace test {
	replication-factor 1
	nsup-period 120
	storage-engine device {
		file /opt/aerospike/data/test.dat
		filesize 4G
		read-page-cache true
	}
}
`,
		tlsName,
		containerCAFile,
		containerServerCert,
		containerServerKey,
		host,
		clientAuthentication,
		mappedTLSPort,
	)
}

func availableHostPort(ctx context.Context, s *AuthSuite) int {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp4", "127.0.0.1:0")
	s.Require().NoError(err)
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func (s *AuthSuite) copyServerCertificates(ctx context.Context, container *tcAerospike.Container) {
	ca, err := os.ReadFile(s.certs.caCert)
	s.Require().NoError(err)
	s.Require().NoError(container.CopyToContainer(ctx, ca, containerCAFile, 0o644))
	s.Require().NoError(container.CopyToContainer(ctx, s.certs.serverCert, containerServerCert, 0o644))
	s.Require().NoError(container.CopyToContainer(ctx, s.certs.serverKey, containerServerKey, 0o600))
}

func (s *AuthSuite) waitForTLS(ctx context.Context, host string, port int, profile authProfile) error {
	caPEM, err := os.ReadFile(s.certs.caCert)
	if err != nil {
		return err
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return errors.New("append test CA")
	}

	config := &tls.Config{
		RootCAs:    roots,
		ServerName: tlsName,
		MinVersion: tls.VersionTLS12,
	}
	if profile.requiresClientCert() {
		cert, loadErr := tls.LoadX509KeyPair(s.certs.internalCert, s.certs.internalKey)
		if loadErr != nil {
			return loadErr
		}
		config.Certificates = []tls.Certificate{cert}
	}

	deadline := time.Now().Add(45 * time.Second)
	address := fmt.Sprintf("%s:%d", host, port)
	var last error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		dialer := tls.Dialer{Config: config}
		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr == nil {
			return conn.Close()
		}
		last = dialErr
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("TLS connection to %s: %w", address, last)
}

func (s *AuthSuite) generateCertificates() certificateFiles {
	dir := s.T().TempDir()

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ABS integration CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	s.Require().NoError(err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caCert, err := x509.ParseCertificate(caDER)
	s.Require().NoError(err)
	caFile, caDir := writeCAFiles(s, dir, caPEM)

	// The DNS SAN, not the common name, is what the client checks, so tlsName has to
	// appear there.
	serverCert, serverKey := issueCertificate(
		s,
		caCert,
		caKey,
		2,
		pkix.Name{CommonName: tlsName},
		[]string{tlsName},
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	)
	internalCert, internalKey := issueCertificate(
		s,
		caCert,
		caKey,
		3,
		pkix.Name{CommonName: intUser},
		nil,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	)
	pkiCert, pkiKey := issueCertificate(
		s,
		caCert,
		caKey,
		4, // https://xkcd.com/221/
		pkix.Name{CommonName: pkiUser},
		nil,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	)

	return certificateFiles{
		caCert:               caFile,
		caDir:                caDir,
		serverCert:           serverCert,
		serverKey:            serverKey,
		internalCert:         writeCertificateFile(s, dir, "internal.crt", internalCert),
		internalKey:          writeCertificateFile(s, dir, "internal.key", internalKey),
		internalKeyEncrypted: writeCertificateFile(s, dir, "internal.enc.key", encryptPEMKey(s, internalKey)),
		pkiCert:              writeCertificateFile(s, dir, "pki.crt", pkiCert),
		pkiKeyEncrypted:      writeCertificateFile(s, dir, "pki.enc.key", encryptPEMKey(s, pkiKey)),
	}
}

// encryptPEMKey wraps a PKCS#1 key in a password-protected PEM block so the tests
// can exercise dto.TLS.key-file-password. ABS decrypts this with the same
// (deprecated) PEM encryption the Go stdlib still uses for DEK-Info keys.
func encryptPEMKey(s *AuthSuite, keyPEM []byte) []byte {
	block, _ := pem.Decode(keyPEM)
	s.Require().NotNil(block, "client key PEM")

	encrypted, err := x509.EncryptPEMBlock( //nolint:staticcheck // DEK-Info PEM is what ABS decrypts
		rand.Reader, block.Type, block.Bytes, []byte(clientKeyPassword), x509.PEMCipherAES256)
	s.Require().NoError(err)

	return pem.EncodeToMemory(encrypted)
}

func writeCAFiles(s *AuthSuite, dir string, caPEM []byte) (caFile, caDir string) {
	caFile = filepath.Join(dir, "ca.crt")
	s.Require().NoError(os.WriteFile(caFile, caPEM, 0o600))
	caDir = filepath.Join(dir, "ca")
	s.Require().NoError(os.Mkdir(caDir, 0o700))
	s.Require().NoError(os.WriteFile(filepath.Join(caDir, "ca.crt"), caPEM, 0o600))

	return caFile, caDir
}

func issueCertificate(
	s *AuthSuite,
	ca *x509.Certificate,
	caKey *rsa.PrivateKey,
	serial int64,
	subject pkix.Name,
	dnsNames []string,
	usage []x509.ExtKeyUsage,
) ([]byte, []byte) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      subject,
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  usage,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	s.Require().NoError(err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func writeCertificateFile(s *AuthSuite, dir, name string, content []byte) string {
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, content, 0o600))
	return path
}

func waitForDBStart(ctx context.Context, seed dto.SeedNode) error {
	deadline := time.Now().Add(45 * time.Second)
	var last error

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		client, err := newAdminClient(seed)
		if err == nil {
			client.Close()
			return nil
		}

		last = err
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("admin login to %s:%d: %w", seed.HostName, seed.Port, last)
}

// newAdminClient connects as the default EE superuser over the plaintext port. Every
// profile keeps that port open so bootstrapping is identical regardless of the transport
// under test.
func newAdminClient(seed dto.SeedNode) (*as.Client, error) {
	policy := as.NewClientPolicy()
	policy.User = adminUser
	policy.Password = adminPassword
	policy.Timeout = 2 * time.Second
	policy.UseServicesAlternate = true

	return as.NewClientWithPolicyAndHost(policy, as.NewHost(seed.HostName, int(seed.Port)))
}
