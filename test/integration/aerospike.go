//go:build integration

package integration

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/testcontainers/testcontainers-go"
	tcAerospike "github.com/testcontainers/testcontainers-go/modules/aerospike"
)

const aerospikeImage = "aerospike/aerospike-server-enterprise:8.1"

// startAerospike starts a stock EE node with no security and connects the shared
// client to it.
func (s *Suite) startAerospike() {
	ctx := s.T().Context()

	asContainer, err := tcAerospike.Run(ctx, aerospikeImage,
		testcontainers.WithEnv(map[string]string{
			"REPL_FACTOR": "1",
		}),
	)
	s.Require().NoError(err)
	s.terminateOnCleanup(asContainer, "aerospike")

	host, err := asContainer.Host(ctx)
	s.Require().NoError(err)

	port, err := asContainer.MappedPort(ctx, "3000/tcp")
	s.Require().NoError(err)

	s.seedNode = dto.SeedNode{HostName: host, Port: dto.Port(port.Num())}

	client, err := as.NewClient(host, int(port.Num()))
	s.Require().NoError(err)
	s.T().Cleanup(client.Close)

	s.client = client
}
