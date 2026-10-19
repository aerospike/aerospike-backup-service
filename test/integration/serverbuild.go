//go:build integration

package integration

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Environment variables naming the Aerospike server build ServerBuildSuite runs. Server-side
// backup is not in a published image yet, so the suite runs a locally built asd and skips
// when these are unset.
const (
	// envServerBinary is the path of the asd binary, built for Linux on the Docker host's
	// architecture.
	envServerBinary = "ABS_IT_ASD_BINARY"
	// envFeaturesConf is the path of the Enterprise feature key file asd starts with.
	envFeaturesConf = "ABS_IT_FEATURES_CONF"
)

const (
	// serverBuildImage provides the libraries asd links against: the image the server
	// is built in. It exists only where the server is built, so pullImages skips it.
	serverBuildImage = "aerospike-builder:latest"
	serverPort       = "3000/tcp"
	// serverStartTimeout bounds how long asd takes to format its device and serve.
	serverStartTimeout = time.Minute

	// destinationClusterName is the cluster serverBackupConfig registers to restore into.
	destinationClusterName = "destinationCluster"
	// serverNamespace is the namespace backed up and restored, defined in serverConf.
	serverNamespace = "bar"
	// serverJobTimeout bounds how long a server backup or restore job may take.
	serverJobTimeout = 2 * time.Minute
	// s3Profile names the MinIO credentials in the shared AWS credentials file. The server
	// takes no keys in its commands: each node reads this profile from its own
	// ~/.aws/credentials, and ABS reads it from the file SetupTest writes.
	s3Profile = "abs-minio"
)

// serverConf is the configuration of every node ServerBuildSuite starts.
//
//go:embed testdata/server_backup_aerospike.conf
var serverConf string

// ServerBuildSuite backs up from one single-node cluster running a locally built server and
// restores into another, through a MinIO bucket both the nodes and ABS reach. Both clusters
// and MinIO are started once for the suite.
type ServerBuildSuite struct {
	Suite

	// destinationNode is the node of the cluster restores write to.
	destinationNode dto.SeedNode
	// destinationClient is connected to destinationNode, for checking restored data.
	destinationClient *as.Client
	// s3Endpoint is the MinIO endpoint, reachable from this host and from containers.
	s3Endpoint string
}

// SetupSuite starts both clusters and MinIO, or skips the suite when the server build is not
// configured.
func (s *ServerBuildSuite) SetupSuite() {
	binary, featuresConf := os.Getenv(envServerBinary), os.Getenv(envFeaturesConf)
	if binary == "" || featuresConf == "" {
		s.T().Skipf("set %s and %s to run the server-side backup tests", envServerBinary, envFeaturesConf)
	}

	s.seedNode, s.client = s.startServerBuild("source", binary, featuresConf)
	s.destinationNode, s.destinationClient = s.startServerBuild("destination", binary, featuresConf)
	s.s3Endpoint = s.startSharedMinIO()
}

// SetupTest empties the namespace on both clusters and gives ABS the s3Profile credentials.
func (s *ServerBuildSuite) SetupTest() {
	credentialsFile := filepath.Join(s.T().TempDir(), "credentials")
	s.Require().NoError(os.WriteFile(credentialsFile, []byte(awsCredentials()), 0o600))
	s.T().Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsFile)

	s.Require().NoError(s.client.Truncate(nil, serverNamespace, "", nil))
	s.Require().NoError(s.destinationClient.Truncate(nil, serverNamespace, "", nil))
}

// serverBackupConfig switches ABS to server backup mode, from the source cluster's
// serverNamespace into the MinIO bucket, and registers the destination cluster.
func (s *ServerBuildSuite) serverBackupConfig(c *dto.Config) {
	c.ServiceConfig.Backup = &dto.BackupCommonConfig{BackupMode: dto.BackupModeServer}
	c.AerospikeClusters[clusterName].UseServicesAlternate = nil
	c.AerospikeClusters[destinationClusterName] = &dto.AerospikeCluster{
		SeedNodes: []dto.SeedNode{s.destinationNode},
	}
	c.Storage[storageName] = &dto.Storage{S3Storage: &dto.S3Storage{
		Bucket:             s3Bucket,
		S3Region:           s3Region,
		S3EndpointOverride: s.s3Endpoint,
		S3Profile:          s3Profile,
	}}
	c.BackupRoutines[routineName].Namespaces = ptr.Of([]dto.NamespaceName{serverNamespace})
}

// startServerBuild starts a single-node cluster from the asd binary and feature key file on
// this host, and connects a client to it.
func (s *ServerBuildSuite) startServerBuild(name, binary, featuresConf string) (dto.SeedNode, *as.Client) {
	ctx := s.T().Context()

	node, err := testcontainers.Run(ctx, serverBuildImage,
		testcontainers.WithExposedPorts(serverPort),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
			hostConfig.Binds = append(hostConfig.Binds,
				binary+":/usr/bin/asd:ro",
				featuresConf+":/etc/aerospike/features.conf:ro",
			)
		}),
		testcontainers.WithFiles(
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(serverConf),
				ContainerFilePath: "/etc/aerospike/aerospike.conf",
				FileMode:          0o644,
			},
			testcontainers.ContainerFile{
				Reader:            strings.NewReader(awsCredentials()),
				ContainerFilePath: "/root/.aws/credentials",
				FileMode:          0o600,
			},
		),
		testcontainers.WithEntrypoint("sh", "-c",
			"mkdir -p /opt/aerospike/data /opt/aerospike/smd /opt/aerospike/usr/udf/lua && "+
				"exec /usr/bin/asd --config-file /etc/aerospike/aerospike.conf --foreground"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("service ready").WithStartupTimeout(serverStartTimeout)),
	)
	s.Require().NoError(err)
	s.terminateOnCleanup(node, "aerospike-"+name)

	host, err := node.Host(ctx)
	s.Require().NoError(err)

	port, err := node.MappedPort(ctx, serverPort)
	s.Require().NoError(err)

	client, err := as.NewClient(host, int(port.Num()))
	s.Require().NoError(err)
	s.T().Cleanup(client.Close)

	return dto.SeedNode{HostName: host, Port: dto.Port(port.Num())}, client
}

// awsCredentials is a shared AWS credentials file holding the MinIO root user as s3Profile.
func awsCredentials() string {
	return fmt.Sprintf("[%s]\naws_access_key_id = %s\naws_secret_access_key = %s\n",
		s3Profile, minioRootUser, minioRootPassword)
}

// startSharedMinIO starts MinIO and returns its endpoint on this host's LAN address. The
// Aerospike nodes upload to it from inside Docker and ABS lists it from the host, and both
// share the one endpoint in the storage configuration, so neither localhost nor
// host.docker.internal works for both.
func (s *ServerBuildSuite) startSharedMinIO() string {
	endpoint, err := url.Parse(s.startMinIO())
	s.Require().NoError(err)

	lanAddress, err := hostLANAddress(s.T().Context())
	s.Require().NoError(err)

	endpoint.Host = net.JoinHostPort(lanAddress, endpoint.Port())

	return endpoint.String()
}

// hostLANAddress returns the address this host routes outbound traffic from. Dialing UDP
// sends nothing: it only picks the route.
func hostLANAddress(ctx context.Context) (string, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp4", "192.0.2.1:9")
	if err != nil {
		return "", fmt.Errorf("failed to find the LAN address: %w", err)
	}
	defer func() { _ = conn.Close() }()

	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}
