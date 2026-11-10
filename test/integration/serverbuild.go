//go:build integration

package integration

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go/pkg/asinfo"
	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Environment variables naming the Aerospike server build ServerBuildSuite runs. Server-side
// backup is not in a published image yet, so the suite runs a locally built asd. Unset, they
// default to where the server repositories build it under ~/work; the suite skips when the
// files are not there.
const (
	// envServerBinary is the path of the asd binary, built for Linux on the Docker host's
	// architecture.
	envServerBinary = "ABS_IT_ASD_BINARY"
	// envFeaturesConf is the path of the Enterprise feature key file asd starts with.
	envFeaturesConf = "ABS_IT_FEATURES_CONF"
)

// linuxArch maps the Go architecture to the one the server build names its output after.
//
//nolint:gochecknoglobals // a table.
var linuxArch = map[string]string{"arm64": "aarch64", "amd64": "x86_64"}

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

// The gaps between server-side and scan backups, as the reasons the server tests skip the checks
// the scan tests make. README.md lists them.
const (
	gapRecordCount  = "server-side backup gap: a server backup's metadata records no record count"
	gapRestoreStats = "server-side backup gap: a server restore job reports no record or index counts"
	gapEmptyIncr    = "server-side backup gap: an incremental backup with no changes is written, listed " +
		"and counted as the last incremental"
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
	binary := s.serverBuildFile(envServerBinary,
		"work/aerospike-server/target/Linux-"+linuxArch[runtime.GOARCH]+"/bin/asd")
	featuresConf := s.serverBuildFile(envFeaturesConf, "work/aerospike-server-enterprise/etc/features.conf")

	s.seedNode, s.client = s.startServerBuild("source", binary, featuresConf)
	s.destinationNode, s.destinationClient = s.startServerBuild("destination", binary, featuresConf)
	s.s3Endpoint = s.startSharedMinIO()
}

// SetupTest empties serverNamespace of the source cluster and gives ABS the s3Profile credentials.
// The destination is left as it is: a truncate would filter out the restored records, which keep
// their backup times, and a test that needs it empty deletes its records.
func (s *ServerBuildSuite) SetupTest() {
	credentialsFile := filepath.Join(s.T().TempDir(), "credentials")
	s.Require().NoError(os.WriteFile(credentialsFile, []byte(awsCredentials()), 0o600))
	s.T().Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsFile)

	s.Require().NoError(s.client.Truncate(nil, serverNamespace, "", nil))
}

// serverBuildFile returns the path in env, or else defaultPath under the home directory. It
// skips the suite when the file does not exist.
func (s *ServerBuildSuite) serverBuildFile(env, defaultPath string) string {
	path := os.Getenv(env)
	if path == "" {
		home, err := os.UserHomeDir()
		s.Require().NoError(err)
		path = filepath.Join(home, defaultPath)
	}

	if _, err := os.Stat(path); err != nil { //nolint:gosec // the developer names the file to run
		s.T().Skipf("server build file %s not found (set %s to its path): %v", path, env, err)
	}

	return path
}

// serverBackupConfig switches ABS to server backup mode, from the source cluster's
// serverNamespace into a path of the MinIO bucket of the test's own, and registers the
// destination cluster.
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
		// The tests share the bucket: each lists only the backups it made.
		Path: dto.Path(strings.ReplaceAll(s.T().Name(), "/", "-")),
	}}
	c.BackupRoutines[routineName].Namespaces = new([]dto.NamespaceName{serverNamespace})
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
				"exec /usr/bin/asd --config-file /etc/aerospike/aerospike.conf --foreground "+
				"--preview backup-restore"),
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

// gap runs check as a subtest named name that is skipped for reason: what the scan tests check
// and server-side backup does not do yet.
func (s *ServerBuildSuite) gap(name, reason string, check func()) {
	s.Run(name, func() {
		s.T().Skip(reason)
		check()
	})
}

// waitForFinishedBackup waits until ABS reports the backup of backupType triggered last as
// succeeded, and returns the newest backup of that type listed. A server backup is listed as
// soon as the server commits its metadata.json, a few seconds before its job ends.
func (s *ServerBuildSuite) waitForFinishedBackup(e *env, backupType model.BackupType) dto.BackupDetails {
	previousSuccesses := e.successesBeforeTrigger[backupType]
	previousFailures := e.failuresBeforeTrigger[backupType]

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
		s.Require().Equal(previousFailures, int(s.metricBackupEventCount(e, backupType, outcomeFailure)),
			"%s backup failed", backupType)

		return s.metricBackupSuccessEventCount(e, backupType) > previousSuccesses
	})
	s.Require().True(ok, "%s backup did not succeed within %s", backupType, serverJobTimeout)

	backups := s.getBackups(e, backupType)
	s.Require().NotEmpty(backups, "the %s backup succeeded but no %s backup is listed", backupType, backupType)

	return backups[len(backups)-1] // listings are sorted by creation time
}

// waitForLastBackup waits until ABS reports want as the routine's last backup of backupType.
// ABS reads it from storage after the backup succeeds, in the background, and the next
// incremental backup takes its lower bound from it.
func (s *ServerBuildSuite) waitForLastBackup(e *env, backupType model.BackupType, want time.Time) {
	var last *time.Time

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
		state := s.routineState(e)

		last = state.LastFull
		if backupType == model.BackupTypeIncremental {
			last = state.LastIncremental
		}

		return last != nil && last.Equal(want)
	})
	s.Require().True(ok, "last %s backup is %v, want %s", backupType, last, want)
}

// routineState reads the state of the routine baseConfig creates through
// GET /v1/backups/currentBackup.
func (s *ServerBuildSuite) routineState(e *env) dto.RoutineState {
	status, body := s.do(e, http.MethodGet, fmt.Sprintf("%s/v1/backups/currentBackup/%s", e.baseURL, routineName), nil)
	s.Require().Equal(http.StatusOK, status, "routine state: %s", body)

	var state dto.RoutineState
	s.Require().NoError(decoder.Deserialize(&state, bytes.NewReader(body), decoder.JSON))

	return state
}

// assertServerBackupListed checks that the listing of backupType holds want unchanged.
func (s *ServerBuildSuite) assertServerBackupListed(e *env, backupType model.BackupType, want dto.BackupDetails) {
	s.T().Helper()

	for _, backup := range s.getBackups(e, backupType) {
		if backup.Key == want.Key {
			s.Equal(want, backup)
			return
		}
	}

	s.Failf("backup not listed", "%s backup %s is not listed", backupType, want.Key)
}

// restoreIntoDestination restores the backup at key into the destination cluster via
// POST /v1/restore/full, and waits for the job to succeed.
func (s *ServerBuildSuite) restoreIntoDestination(e *env, key string) dto.RestoreJobStatus {
	return s.restoreByPathInto(e, key, destinationClusterName)
}

// restoreByPathInto restores the backup at key into cluster via POST /v1/restore/full, and waits
// for the job to succeed.
func (s *ServerBuildSuite) restoreByPathInto(e *env, key, cluster string) dto.RestoreJobStatus {
	request := defaultRestoreRequest(key)
	request.DestinationClusterConfig.Name = cluster

	return s.restoreWithin(e, e.baseURL+"/v1/restore/full", request, serverJobTimeout)
}

// restoreIntoDestinationByTimestamp restores the routine's backups up to timestamp into the
// destination cluster via POST /v1/restore/timestamp, and waits for the job to succeed.
func (s *ServerBuildSuite) restoreIntoDestinationByTimestamp(e *env, timestamp time.Time) dto.RestoreJobStatus {
	return s.restoreWithin(e, e.baseURL+"/v1/restore/timestamp",
		timestampRestoreRequest(timestamp, destinationClusterName), serverJobTimeout)
}

// timestampRestoreRequest is a request to restore the routine's backups up to timestamp into
// cluster.
func timestampRestoreRequest(timestamp time.Time, cluster string) dto.RestoreTimestampRequest {
	return dto.RestoreTimestampRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: cluster,
		},
		Policy:  &dto.TimestampRestorePolicy{},
		Time:    timestamp.UnixMilli(),
		Routine: routineName,
	}
}

// emptyNamespace deletes every record of ns. Unlike a truncate it leaves no truncate state, which
// would filter out the records restored into ns.
func (s *ServerBuildSuite) emptyNamespace(client *as.Client, ns string) {
	recordset, err := client.ScanAll(nil, ns, "")
	s.Require().NoError(err)

	for result := range recordset.Results() {
		s.Require().NoError(result.Err)

		_, err := client.Delete(nil, result.Record.Key)
		s.Require().NoError(err)
	}
}

// info sends an info command to a node of the cluster client is connected to.
func (s *ServerBuildSuite) info(client *as.Client, command string) string {
	node, err := client.Cluster().GetRandomNode()
	s.Require().NoError(err)

	response, err := node.RequestInfo(as.NewInfoPolicy(), command)
	s.Require().NoError(err)

	return response[command]
}

// assertServerBackupDetails checks the fields a full server backup of serverNamespace reports
// from the server's metadata: it was created at or after createdAfter, is keyed by its job id,
// is not incremental, and is stored uncompressed and unencrypted in the MinIO bucket.
func (s *ServerBuildSuite) assertServerBackupDetails(backup dto.BackupDetails, createdAfter time.Time) {
	s.T().Helper()

	s.True(asinfo.IsJobID(backup.Key), "key %q is not a server job id", backup.Key)
	s.Equal(serverNamespace, backup.Namespace)

	// The job id holds whole seconds.
	s.False(backup.Created.Before(createdAfter.Truncate(time.Second)),
		"created %s before the backup was triggered at %s", backup.Created, createdAfter)
	s.Equal(backup.Created.UnixMilli(), backup.Timestamp)
	s.True(backup.From.IsZero(), "a full backup has no lower bound, got %s", backup.From)

	s.Require().NotNil(backup.Storage)
	s.Require().NotNil(backup.Storage.S3Storage)
	s.Equal(s3Bucket, backup.Storage.S3Storage.Bucket)
	s.Equal("NONE", backup.Compression)
	s.Equal("NONE", backup.Encryption)
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
