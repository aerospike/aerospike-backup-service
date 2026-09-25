//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/api/option"
)

const (
	fakeGCSImage = "fsouza/fake-gcs-server:1.56.1"
	fakeGCSPort  = "4443/tcp"
	gcpBucket    = "abs-integration-test"

	gcpKeySecretName = "gcp-key-json"
)

// TestGCP exercises every way ABS can authenticate to a GCS-compatible endpoint, against
// one shared fake-gcs-server container: the no-auth Endpoint-only path, and real
// service-account key auth (both key-file and key-JSON, literal and Secret Agent) with
// the credential exchange redirected at a local fake OAuth token endpoint instead of
// Google's real one - see fakeServiceAccountKey. ADC (Application Default Credentials,
// the path taken when neither KeyFile nor Key is set and Endpoint is empty) is not
// covered: it can only resolve against a real GCP environment (metadata server, gcloud
// config, or GOOGLE_APPLICATION_CREDENTIALS), which an emulator cannot stand in for.
func (s *StorageSuite) TestGCP() {
	endpoint := s.startFakeGCS()

	s.Run("endpoint, no authentication", func() {
		s.storageBackup(&dto.Storage{GcpStorage: &dto.GcpStorage{
			BucketName: gcpBucket,
			Endpoint:   endpoint,
		}})
	})

	s.Run("key file, literal", func() {
		tokenServer := s.startFakeGCPTokenServer()
		keyFile := filepath.Join(s.T().TempDir(), "key.json")
		s.Require().NoError(os.WriteFile(keyFile, []byte(s.fakeServiceAccountKey(tokenServer.URL)), 0o600))

		s.storageBackup(&dto.Storage{GcpStorage: &dto.GcpStorage{
			BucketName: gcpBucket,
			Endpoint:   endpoint,
			KeyFile:    dto.Path(keyFile),
		}})
	})

	s.Run("key json, literal", func() {
		tokenServer := s.startFakeGCPTokenServer()

		s.storageBackup(&dto.Storage{GcpStorage: &dto.GcpStorage{
			BucketName: gcpBucket,
			Endpoint:   endpoint,
			Key:        redact.Secret(s.fakeServiceAccountKey(tokenServer.URL)),
		}})
	})

	s.Run("key json from secret agent", func() {
		tokenServer := s.startFakeGCPTokenServer()
		agent := s.startSecretAgentWithKeys(map[string]string{
			gcpKeySecretName: s.fakeServiceAccountKey(tokenServer.URL),
		})

		s.storageBackup(&dto.Storage{GcpStorage: &dto.GcpStorage{
			SecretAgentConfig: dto.SecretAgentConfig{SecretAgent: agent},
			BucketName:        gcpBucket,
			Endpoint:          endpoint,
			Key:               redact.Secret(secretRefKey(gcpKeySecretName)),
		}})
	})
}

// startFakeGCS starts a fake-gcs-server container and creates the bucket the GCP tests
// write to. It returns the endpoint ABS should reach it at (dto.GcpStorage.Endpoint).
func (s *StorageSuite) startFakeGCS() string {
	ctx := s.T().Context()

	// fake-gcs-server signs self-referential download URLs (mediaLink) using whatever
	// -public-host it was started with; those URLs are what ABS's object reads actually
	// hit. The host port Docker would otherwise assign at random isn't known until after
	// the container starts, so -public-host would be wrong - reserve a free port first and
	// bind the container to it, so -public-host can name the real externally-reachable
	// address up front. Same trick as auth_suite.go's availableHostPort/TLS port pinning.
	hostPort := availableHostPort(ctx, &s.Suite)
	publicHost := fmt.Sprintf("127.0.0.1:%d", hostPort)

	gcsContainer, err := testcontainers.Run(ctx, fakeGCSImage,
		testcontainers.WithExposedPorts(fakeGCSPort),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
			if hostConfig.PortBindings == nil {
				hostConfig.PortBindings = network.PortMap{}
			}
			hostConfig.PortBindings[network.MustParsePort(fakeGCSPort)] = []network.PortBinding{{
				HostIP:   netip.MustParseAddr("127.0.0.1"),
				HostPort: strconv.Itoa(hostPort),
			}}
		}),
		testcontainers.WithCmd("-scheme", "http", "-port", "4443", "-public-host", publicHost),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(fakeGCSPort)),
	)
	s.Require().NoError(err)
	s.cleanupContainer(gcsContainer)

	endpoint := fmt.Sprintf("http://%s/storage/v1/", publicHost)
	s.createGCPBucket(ctx, endpoint)

	return endpoint
}

// createGCPBucket creates the bucket the GCP tests share, using the same no-auth path
// as the "endpoint, no authentication" sub-test - independent of whichever auth mode a
// given sub-test exercises.
func (s *StorageSuite) createGCPBucket(ctx context.Context, endpoint string) {
	client, err := storage.NewClient(ctx, option.WithEndpoint(endpoint), option.WithoutAuthentication())
	s.Require().NoError(err)
	defer func() { _ = client.Close() }()

	s.Require().NoError(client.Bucket(gcpBucket).Create(ctx, "abs-integration-test-project", nil))
}

// startFakeGCPTokenServer stands in for Google's OAuth2 token endpoint. ABS's
// service-account auth paths (KeyFile/Key) sign a real JWT client assertion locally and
// POST it to the key's token_uri to exchange it for an access token; fake-gcs-server
// itself does not check that token at all, so this only needs to hand back a
// syntactically valid OAuth2 response for the client library to accept.
func (s *StorageSuite) startFakeGCPTokenServer() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	s.T().Cleanup(server.Close)

	return server
}

// fakeServiceAccountKey builds a syntactically valid GCP service-account JSON key whose
// token_uri points at a local fake OAuth token endpoint instead of Google's real one, so
// the real credential-exchange code path (google.golang.org/api/option.WithAuthCredentialsFile/
// WithAuthCredentialsJSON) can be exercised end-to-end against fake-gcs-server without
// talking to the internet or owning a real GCP service account.
func (s *StorageSuite) fakeServiceAccountKey(tokenURL string) string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	s.Require().NoError(err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	data, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "abs-integration-test",
		"private_key_id": "test-key-id",
		"private_key":    string(keyPEM),
		"client_email":   "abs-integration-test@abs-integration-test.iam.gserviceaccount.com",
		"token_uri":      tokenURL,
	})
	s.Require().NoError(err)

	return string(data)
}
