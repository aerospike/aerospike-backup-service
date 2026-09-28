//go:build integration

package integration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/testcontainers/testcontainers-go"
	tcAerospike "github.com/testcontainers/testcontainers-go/modules/aerospike"
)

// Users on the secured nodes.
const (
	// intUser logs in with auth-mode INTERNAL and intPassword.
	intUser     = "intuser"
	intPassword = "s3cr3t!"
	// pkiUser logs in with auth-mode PKI: no password, identified by the CN of
	// clusterCertificates.PKI.
	pkiUser = "pkiuser"

	adminUser     = "admin"
	adminPassword = "admin"
	readWriteRole = "read-write"
)

const (
	containerConfPath   = "/etc/aerospike/aerospike.conf"
	containerCAFile     = "/etc/aerospike/ca.crt"
	containerServerCert = "/etc/aerospike/server.crt"
	containerServerKey  = "/etc/aerospike/server.key"

	plainPort = "3000/tcp"
	tlsPort   = "4333/tcp"
	// tlsName is both the Aerospike `tls` stanza name and the DNS SAN of the server
	// certificate. The client sends it as SNI and then verifies it against the cert,
	// so the two must stay in sync.
	tlsName = "test-tls"
	// tlsProtocols pins the handshake to TLS 1.2. That is the only version ABS
	// currently maps (see pkg/tlsconfig); a single token is both min and max.
	tlsProtocols = "TLSv1.2"
	// tlsCipherSuite is an IANA name, colon-separated if there are several.
	// The certificates are RSA, so the suite must be ECDHE_RSA, not ECDSA.
	tlsCipherSuite = "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"

	// nodeStartTimeout bounds how long a restarted node takes to accept logins.
	nodeStartTimeout = 45 * time.Second
	nodePollInterval = 500 * time.Millisecond
)

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

// SecuredClusterSuite backs up from Aerospike nodes with security enabled. Since no
// two profiles can share a server process, each test starts the node it needs with
// startSecuredNode, and the node stops when that test returns, keeping only one
// secured container alive at a time.
type SecuredClusterSuite struct {
	Suite

	certs clusterCertificates
}

// SetupSuite mints the PKI every secured node and TLS config in the suite shares.
func (s *SecuredClusterSuite) SetupSuite() {
	s.certs = s.generateClusterCertificates()
}

// SetupTest does nothing: there is no suite-wide node to truncate, and each test
// truncates the node it starts.
func (s *SecuredClusterSuite) SetupTest() {}

// securedNode is one running secured node, with intUser and (for profileMutualTLS)
// pkiUser provisioned.
type securedNode struct {
	// Seed is what ABS is pointed at, and carries the profile under test: the TLS
	// port and tls-name for the TLS profiles.
	Seed dto.SeedNode

	// adminSeed is always the plaintext service port. User and role management is
	// done out of band so that bootstrapping never depends on the transport under test.
	adminSeed dto.SeedNode
	// adminClient is logged in as the superuser over adminSeed.
	adminClient *as.Client
}

// startSecuredNode boots a node for one profile and creates the users tests log in as.
func (s *SecuredClusterSuite) startSecuredNode(profile authProfile) securedNode {
	node := s.startSecureAerospike(profile)

	admin := node.adminClient
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
	admin, err := newAdminClient(node.adminSeed)
	s.Require().NoError(err)
	s.T().Cleanup(admin.Close)
	node.adminClient = admin

	return node
}

// assertBackupFromCluster points ABS at node through cluster, runs a full backup of
// three freshly seeded records, and asserts all three are in it.
func (s *SecuredClusterSuite) assertBackupFromCluster(node securedNode, cluster *dto.AerospikeCluster) {
	s.Require().NoError(node.adminClient.Truncate(nil, namespace, "", nil))
	s.seedRecordsWith(node.adminClient, []int{10, 20, 30})

	e := s.setupEnv(func(c *dto.Config) {
		c.AerospikeClusters[clusterName] = cluster
	})
	s.triggerFullBackup(e)

	s.assertBackupDetails(s.waitForFullBackup(e), 3)
}

// dto.TLS is ABS's client-side TLS config for Aerospike. It does not configure the
// database; it tells ABS how to verify the server and, for mTLS, how to prove who
// it is.
//
// Handshake, in one pass:
//  1. ABS opens a TLS connection and sends SNI (the name it expects the cert to use).
//  2. The server presents its certificate. ABS trusts it only if it chains to a CA
//     loaded from ca-file or ca-path.
//  3. If the server asks for a client cert (tls-authenticate-client), ABS presents
//     cert-file, proving possession with key-file (unlocked by key-file-password
//     when the PEM is encrypted).
//  4. Both sides agree a protocol version (protocols) and a cipher (cipher-suite).
//
// ca-file and ca-path are mutually exclusive. The two builders below split them:
// server-only TLS uses ca-path; mTLS uses ca-file. Together they set every field.

// serverOnlyTLS is the dto.TLS for profileServerTLS: ABS verifies the server and
// presents no certificate of its own.
func (s *SecuredClusterSuite) serverOnlyTLS() *dto.TLS {
	return &dto.TLS{
		ClientTLS: dto.ClientTLS{
			// ca-file is the other way to load CAs (a single PEM bundle). Left empty
			// here because ca-path is set below; both at once is a validation error.
			CAFile: "",
			// name is SNI / ServerName on the cluster TLS object. Validation requires
			// it together with cert-file and key-file, so server-only TLS cannot set
			// it. The seed node's tls-name (already set on securedNode.Seed) is what
			// the Aerospike client sends as SNI in this profile.
			Name:     "",
			Certfile: "",
			Keyfile:  "",
		},
		// ca-path is a directory of PEM CA files. Same purpose as ca-file: decide
		// whether the server's certificate is trusted. Used when CAs are dropped
		// in as separate files (for example a Kubernetes secret volume).
		CAPath: dto.Path(s.certs.CADir),
		// protocols is Apache SSLProtocol syntax, space-separated. ABS currently
		// accepts TLSv1.2 only. One token pins both the minimum and the maximum.
		Protocols: tlsProtocols,
		// cipher-suite is colon-separated IANA names (not OpenSSL nicknames). It
		// restricts which TLS 1.2 ciphers ABS offers. Empty means Go's defaults.
		CipherSuite: tlsCipherSuite,
		// key-file-password is only meaningful with key-file (encrypted client key).
		KeyfilePassword: "",
	}
}

// mutualTLS is the dto.TLS for profileMutualTLS, presenting client as ABS's
// certificate with the literal keyPassword unlocking its key.
func (s *SecuredClusterSuite) mutualTLS(client clientCertificate) *dto.TLS {
	return &dto.TLS{
		ClientTLS: dto.ClientTLS{
			// ca-file: PEM bundle of CAs ABS trusts to sign the server certificate.
			CAFile: dto.Path(s.certs.CAFile),
			// name: hostname ABS puts in SNI and then checks against the server cert.
			// Must be set together with cert-file and key-file.
			Name: tlsName,
			// cert-file: the client certificate. The server uses this to decide who
			// ABS is. For auth-mode PKI the Aerospike username is the cert CN.
			Certfile: dto.Path(client.CertFile),
			// key-file: private key matching cert-file. Never sent on the wire;
			// used to prove ownership of the certificate. Encrypted here.
			Keyfile: dto.Path(client.EncryptedKeyFile),
		},
		// ca-path left empty: mutually exclusive with ca-file.
		CAPath: "",
		// protocols / cipher-suite: same meaning as in serverOnlyTLS.
		Protocols:   tlsProtocols,
		CipherSuite: tlsCipherSuite,
		// key-file-password: passphrase for an encrypted key-file PEM. Can also be
		// a secrets:<resource>:<key> reference when a secret agent is configured.
		KeyfilePassword: keyPassword,
	}
}

// startSecureAerospike brings up a single node with security enabled.
//
// The image entrypoint rewrites /etc/aerospike/aerospike.conf every time it boots a
// fresh container, so the config cannot simply be mounted. The sequence is: let the
// image start normally, stop it, copy in the config (and certificates), start it again.
func (s *SecuredClusterSuite) startSecureAerospike(profile authProfile) securedNode {
	ctx := s.T().Context()

	var (
		options       []testcontainers.ContainerCustomizer
		mappedTLSPort int
	)
	if profile.usesTLS() {
		// The port has to be baked into tls-alternate-access-port before the restart.
		port, pin := s.pinnedHostPort(tlsPort)
		mappedTLSPort = port
		options = append(options, testcontainers.WithExposedPorts(tlsPort), pin)
	}

	asContainer, err := tcAerospike.Run(ctx, aerospikeImage, options...)
	s.Require().NoError(err)
	s.terminateOnCleanup(asContainer, "secured aerospike")

	host, err := asContainer.Host(ctx)
	s.Require().NoError(err)

	stopTimeout := 10 * time.Second
	s.Require().NoError(asContainer.Stop(ctx, &stopTimeout))

	config := secureConf
	if profile.usesTLS() {
		config = tlsSecureConf(host, mappedTLSPort, profile)
		s.copyServerCertificates(ctx, asContainer)
	}
	s.Require().NoError(asContainer.CopyToContainer(ctx, []byte(config), containerConfPath, 0o644))
	s.Require().NoError(asContainer.Start(ctx))

	mappedPlainPort, err := asContainer.MappedPort(ctx, plainPort)
	s.Require().NoError(err)
	adminSeed := dto.SeedNode{HostName: host, Port: dto.Port(mappedPlainPort.Num())}
	s.waitForLogin(adminSeed)

	seed := adminSeed
	if profile.usesTLS() {
		mapped, mapErr := asContainer.MappedPort(ctx, tlsPort)
		s.Require().NoError(mapErr)
		s.Require().Equal(mappedTLSPort, int(mapped.Num()), "TLS port mapping changed after restart")
		s.waitForTLS(host, mappedTLSPort, profile)
		seed = dto.SeedNode{HostName: host, Port: dto.Port(mappedTLSPort), TLSName: tlsName}
	}

	admin, err := newAdminClient(adminSeed)
	s.Require().NoError(err)

	return securedNode{Seed: seed, adminSeed: adminSeed, adminClient: admin}
}

func (s *SecuredClusterSuite) copyServerCertificates(ctx context.Context, asContainer *tcAerospike.Container) {
	ca, err := os.ReadFile(s.certs.CAFile)
	s.Require().NoError(err)
	s.Require().NoError(asContainer.CopyToContainer(ctx, ca, containerCAFile, 0o644))
	s.Require().NoError(asContainer.CopyToContainer(ctx, s.certs.serverCert, containerServerCert, 0o644))
	s.Require().NoError(asContainer.CopyToContainer(ctx, s.certs.serverKey, containerServerKey, 0o600))
}

// waitForLogin polls until the superuser can log in over the plaintext port.
func (s *SecuredClusterSuite) waitForLogin(seed dto.SeedNode) {
	var last error

	ok := s.eventually(nodeStartTimeout, nodePollInterval, func() bool {
		client, err := newAdminClient(seed)
		if err != nil {
			last = err
			return false
		}

		client.Close()

		return true
	})
	s.Require().True(ok, "admin login to %s:%d: %v", seed.HostName, seed.Port, last)
}

// waitForTLS polls until a TLS handshake with the node succeeds.
func (s *SecuredClusterSuite) waitForTLS(host string, port int, profile authProfile) {
	caPEM, err := os.ReadFile(s.certs.CAFile)
	s.Require().NoError(err)

	roots := x509.NewCertPool()
	s.Require().True(roots.AppendCertsFromPEM(caPEM), "append test CA")

	config := &tls.Config{
		RootCAs:    roots,
		ServerName: tlsName,
		MinVersion: tls.VersionTLS12,
	}
	if profile.requiresClientCert() {
		cert, loadErr := tls.LoadX509KeyPair(s.certs.Internal.CertFile, s.certs.Internal.keyFile)
		s.Require().NoError(loadErr)
		config.Certificates = []tls.Certificate{cert}
	}

	address := fmt.Sprintf("%s:%d", host, port)
	last := errors.New("no attempt made")

	ok := s.eventually(nodeStartTimeout, nodePollInterval, func() bool {
		dialer := tls.Dialer{Config: config}
		conn, dialErr := dialer.DialContext(s.T().Context(), "tcp", address)
		if dialErr != nil {
			last = dialErr
			return false
		}

		_ = conn.Close()

		return true
	})
	s.Require().True(ok, "TLS connection to %s: %v", address, last)
}

// newAdminClient connects as the default EE superuser over the plaintext port. Every
// profile keeps that port open so bootstrapping is identical regardless of the
// transport under test.
func newAdminClient(seed dto.SeedNode) (*as.Client, error) {
	policy := as.NewClientPolicy()
	policy.User = adminUser
	policy.Password = adminPassword
	policy.Timeout = 2 * time.Second
	policy.UseServicesAlternate = true

	return as.NewClientWithPolicyAndHost(policy, as.NewHost(seed.HostName, int(seed.Port)))
}

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
