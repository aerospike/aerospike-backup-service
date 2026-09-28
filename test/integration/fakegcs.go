//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"

	"cloud.google.com/go/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/api/option"
)

// gcpBucket is the bucket startFakeGCS creates.
const gcpBucket = "abs-integration-test"

const (
	fakeGCSImage = "fsouza/fake-gcs-server:1.56.1"
	fakeGCSPort  = "4443/tcp"
	gcpProject   = "abs-integration-test"
)

// startFakeGCS starts a fake-gcs-server container and creates gcpBucket in it. It
// returns the endpoint ABS reaches it at (dto.GcpStorage.Endpoint).
func (s *Suite) startFakeGCS() string {
	ctx := s.T().Context()

	// fake-gcs-server signs self-referential download URLs (mediaLink) with whatever
	// -public-host it was started with, and those URLs are what ABS's object reads
	// hit. So -public-host must name the real host port, known before the start.
	hostPort, pin := s.pinnedHostPort(fakeGCSPort)
	publicHost := fmt.Sprintf("127.0.0.1:%d", hostPort)

	gcs, err := testcontainers.Run(ctx, fakeGCSImage,
		testcontainers.WithExposedPorts(fakeGCSPort),
		pin,
		testcontainers.WithCmd("-scheme", "http", "-port", "4443", "-public-host", publicHost),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(fakeGCSPort)),
	)
	s.Require().NoError(err)
	s.terminateOnCleanup(gcs, "fake-gcs-server")

	endpoint := fmt.Sprintf("http://%s/storage/v1/", publicHost)
	s.createGCPBucket(ctx, endpoint)

	return endpoint
}

// startFakeGCPTokenServer stands in for Google's OAuth2 token endpoint and returns
// its URL. ABS's service-account auth (KeyFile/Key) signs a real JWT client
// assertion locally and POSTs it to the key's token_uri to exchange it for an access
// token. fake-gcs-server does not check that token at all, so this only hands back a
// syntactically valid OAuth2 response for the client library to accept.
func (s *Suite) startFakeGCPTokenServer() string {
	response, err := decoder.Marshal(map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	}, decoder.JSON, false)
	s.Require().NoError(err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	s.T().Cleanup(server.Close)

	return server.URL
}

// fakeServiceAccountKey builds a syntactically valid GCP service-account JSON key
// whose token_uri is tokenURL rather than Google's real endpoint, so the real
// credential exchange runs end to end without the internet or a GCP account.
func (s *Suite) fakeServiceAccountKey(tokenURL string) string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	s.Require().NoError(err)

	data, err := decoder.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     gcpProject,
		"private_key_id": "test-key-id",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   gcpProject + "@" + gcpProject + ".iam.gserviceaccount.com",
		"token_uri":      tokenURL,
	}, decoder.JSON, false)
	s.Require().NoError(err)

	return string(data)
}

// createGCPBucket uses the unauthenticated endpoint path, independent of whichever
// auth mode a test gives ABS.
func (s *Suite) createGCPBucket(ctx context.Context, endpoint string) {
	client, err := storage.NewClient(ctx, option.WithEndpoint(endpoint), option.WithoutAuthentication())
	s.Require().NoError(err)
	defer func() { _ = client.Close() }()

	s.Require().NoError(client.Bucket(gcpBucket).Create(ctx, gcpProject, nil))
}
