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
	tlsName           = "test-tls"

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

// Stock EE docker conf plus a security stanza. The image entrypoint overwrites
// /etc/aerospike/aerospike.conf on first start, so this is copied in after that.
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

type authCluster struct {
	seed      dto.SeedNode
	adminSeed dto.SeedNode
	admin     *as.Client
}

type certificateFiles struct {
	caCert       string
	serverCert   []byte
	serverKey    []byte
	internalCert string
	internalKey  string
	pkiCert      string
	pkiKey       string
}

// AuthSuite runs connection smoke tests against secured EE nodes. Only one
// Aerospike container is kept running at a time; each Test* method starts the
// profile it needs and testcontainers cleanup stops it when that test returns.
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

func (s *AuthSuite) startAndProvision(withTLS, requireClientCert, withPKI bool) authCluster {
	t := s.T()
	cluster := s.startSecureAerospike(context.Background(), withTLS, requireClientCert)

	s.Require().NoError(cluster.admin.CreateUser(nil, intUser, intPassword, []string{readWriteRole}))
	s.Require().NoError(cluster.admin.GrantRoles(
		nil, adminUser, []string{"sys-admin", "truncate", readWriteRole},
	))
	if withPKI {
		s.Require().NoError(cluster.admin.CreatePKIUser(nil, pkiUser, []string{readWriteRole}))
	}

	cluster.admin.Close()
	admin, err := newAdminClient(cluster.adminSeed)
	s.Require().NoError(err)
	t.Cleanup(admin.Close)
	cluster.admin = admin

	return cluster
}

//nolint:funlen // Container lifecycle is clearer when setup remains in one place.
func (s *AuthSuite) startSecureAerospike(
	ctx context.Context,
	withTLS bool,
	requireClientCert bool,
) authCluster {
	t := s.T()

	var options []testcontainers.ContainerCustomizer
	if withTLS {
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

	container, err := tcAerospike.Run(ctx, aerospikeImage, options...)
	s.Require().NoError(err)
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate Aerospike container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	s.Require().NoError(err)

	var mappedTLSPort int
	if withTLS {
		mapped, mapErr := container.MappedPort(ctx, tlsPort)
		s.Require().NoError(mapErr)
		mappedTLSPort = int(mapped.Num())
	}

	stopTimeout := 10 * time.Second
	s.Require().NoError(container.Stop(ctx, &stopTimeout))

	config := secureConf
	if withTLS {
		config = tlsSecureConf(host, mappedTLSPort, requireClientCert)
		s.copyServerCertificates(ctx, container)
	}
	s.Require().NoError(container.CopyToContainer(ctx, []byte(config), containerConfPath, 0o644))
	s.Require().NoError(container.Start(ctx))

	// Start() can return after the module sees logs from the first boot. Poll a
	// real authenticated connection before proceeding.
	plainPort, err := container.MappedPort(ctx, "3000/tcp")
	s.Require().NoError(err)
	adminSeed := dto.SeedNode{HostName: host, Port: dto.Port(plainPort.Num())}
	s.Require().NoError(waitForAdmin(ctx, adminSeed))

	seed := adminSeed
	if withTLS {
		mapped, mapErr := container.MappedPort(ctx, tlsPort)
		s.Require().NoError(mapErr)
		s.Equal(mappedTLSPort, int(mapped.Num()), "TLS port mapping changed after restart")
		s.Require().NoError(s.waitForTLS(ctx, host, mappedTLSPort, requireClientCert))
		seed = dto.SeedNode{
			HostName: host,
			Port:     dto.Port(mapped.Num()),
			TLSName:  tlsName,
		}
	}

	admin, err := newAdminClient(adminSeed)
	s.Require().NoError(err)

	return authCluster{seed: seed, adminSeed: adminSeed, admin: admin}
}

//nolint:funlen // The embedded Aerospike configuration is intentionally kept together.
func tlsSecureConf(host string, mappedTLSPort int, requireClientCert bool) string {
	clientAuthentication := "false"
	if requireClientCert {
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

func (s *AuthSuite) waitForTLS(ctx context.Context, host string, port int, withClientCert bool) error {
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
	if withClientCert {
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
		SerialNumber:          randomSerial(s),
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

	caFile := filepath.Join(dir, "ca.crt")
	s.Require().NoError(os.WriteFile(caFile, caPEM, 0o600))

	serverCert, serverKey := issueCertificate(
		s,
		caCert,
		caKey,
		pkix.Name{CommonName: tlsName},
		[]string{tlsName},
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	)
	internalCert, internalKey := issueCertificate(
		s,
		caCert,
		caKey,
		pkix.Name{CommonName: intUser},
		nil,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	)
	pkiCert, pkiKey := issueCertificate(
		s,
		caCert,
		caKey,
		pkix.Name{CommonName: pkiUser},
		nil,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	)

	return certificateFiles{
		caCert:       caFile,
		serverCert:   serverCert,
		serverKey:    serverKey,
		internalCert: writeCertificateFile(s, dir, "internal.crt", internalCert),
		internalKey:  writeCertificateFile(s, dir, "internal.key", internalKey),
		pkiCert:      writeCertificateFile(s, dir, "pki.crt", pkiCert),
		pkiKey:       writeCertificateFile(s, dir, "pki.key", pkiKey),
	}
}

func issueCertificate(
	s *AuthSuite,
	ca *x509.Certificate,
	caKey *rsa.PrivateKey,
	subject pkix.Name,
	dnsNames []string,
	usage []x509.ExtKeyUsage,
) ([]byte, []byte) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	template := &x509.Certificate{
		SerialNumber: randomSerial(s),
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

func randomSerial(s *AuthSuite) *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	s.Require().NoError(err)
	return serial
}

func writeCertificateFile(s *AuthSuite, dir, name string, content []byte) string {
	path := filepath.Join(dir, name)
	s.Require().NoError(os.WriteFile(path, content, 0o600))
	return path
}

func waitForAdmin(ctx context.Context, seed dto.SeedNode) error {
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

func newAdminClient(seed dto.SeedNode) (*as.Client, error) {
	policy := as.NewClientPolicy()
	policy.User = adminUser
	policy.Password = adminPassword
	policy.Timeout = 2 * time.Second
	policy.UseServicesAlternate = true

	return as.NewClientWithPolicyAndHost(policy, as.NewHost(seed.HostName, int(seed.Port)))
}
