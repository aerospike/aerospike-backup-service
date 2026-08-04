//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/testcontainers/testcontainers-go"
	tcAerospike "github.com/testcontainers/testcontainers-go/modules/aerospike"
)

const aerospikeImage = "aerospike/aerospike-server:8.1"

// One Aerospike container and one client are shared by every test in the package.
var (
	asSeedNode dto.SeedNode
	asClient   *as.Client
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests owns the shared container and client so that deferred cleanup runs before os.Exit.
func runTests(m *testing.M) int {
	ctx := context.Background()

	container, err := tcAerospike.Run(ctx, aerospikeImage,
		testcontainers.WithEnv(map[string]string{
			"REPL_FACTOR": "1",
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start aerospike container: %v\n", err)
		return 1
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate aerospike container: %v\n", err)
		}
	}()

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get aerospike host: %v\n", err)
		return 1
	}

	port, err := container.MappedPort(ctx, "3000/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get aerospike port: %v\n", err)
		return 1
	}

	asSeedNode = dto.SeedNode{HostName: host, Port: dto.Port(port.Num())}

	client, asErr := as.NewClient(asSeedNode.HostName, int(asSeedNode.Port))
	if asErr != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to aerospike: %v\n", asErr)
		return 1
	}
	defer client.Close()

	asClient = client

	return m.Run()
}
