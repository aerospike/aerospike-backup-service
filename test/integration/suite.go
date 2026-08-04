//go:build integration

// Package integration holds the end-to-end tests for the backup service.
//
// Files without a _test.go suffix are the harness: the suite, the service fixture and the API
// client. The _test.go files hold the tests themselves. To add a test, write a method on
// Suite in a _test.go file; it will pick up the shared container automatically.
package integration

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcAerospike "github.com/testcontainers/testcontainers-go/modules/aerospike"
)

const (
	aerospikeImage = "aerospike/aerospike-server:8.1"

	clusterName = "testCluster"
	storageName = "local"
	policyName  = "defaultPolicy"
	routineName = "integrationRoutine"
	namespace   = "test"
	setName     = "filteredSet"
)

// Suite runs every test in this package against a single Aerospike container.
type Suite struct {
	suite.Suite

	seedNode dto.SeedNode
	client   *as.Client
}

// SetupSuite starts the Aerospike container and connects the shared client.
//
// Cleanup is registered with T().Cleanup instead of TearDownSuite because testify only registers
// its TearDownSuite defer after SetupSuite returns; a failure part way through here would
// otherwise leave the container running.
func (s *Suite) SetupSuite() {
	t := s.T()
	ctx := context.Background()

	container, err := tcAerospike.Run(ctx, aerospikeImage,
		testcontainers.WithEnv(map[string]string{
			"REPL_FACTOR": "1",
		}),
	)
	s.Require().NoError(err)
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate Aerospike container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	s.Require().NoError(err)

	port, err := container.MappedPort(ctx, "3000/tcp")
	s.Require().NoError(err)

	s.seedNode = dto.SeedNode{HostName: host, Port: dto.Port(port.Num())}

	client, asErr := as.NewClient(host, int(port.Num()))
	s.Require().NoError(asErr)
	t.Cleanup(client.Close)

	s.client = client
}

// SetupTest hands every test an empty namespace. All tests share this one namespace, so they must
// not call T().Parallel().
func (s *Suite) SetupTest() {
	s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))
}
