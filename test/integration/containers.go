//go:build integration

package integration

import (
	"context"
	"io"
	"net"
	"net/netip"
	"strconv"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
)

// cleanupContext is for work registered with T().Cleanup: T().Context() is already
// canceled by the time cleanups run, which would make every call in them fail.
func (s *Suite) cleanupContext() context.Context {
	return context.WithoutCancel(s.T().Context())
}

// terminateOnCleanup kills the container when the current test ends, first dumping
// its logs if the test failed.
//
// Nothing in a test container is worth a graceful shutdown, and some images
// (Azurite) ignore SIGTERM, so every stop would wait out Docker's 10s default.
//
// It is registered with T().Cleanup rather than done in a TearDown method because
// testify only registers TearDownSuite after SetupSuite returns: a failure part way
// through setup would otherwise leave the container running.
func (s *Suite) terminateOnCleanup(c testcontainers.Container, name string) {
	t := s.T()
	ctx := s.cleanupContext()

	t.Cleanup(func() {
		if t.Failed() {
			s.logContainer(ctx, c, name)
		}

		if err := c.Terminate(ctx, testcontainers.StopTimeout(0)); err != nil {
			t.Logf("failed to terminate %s container: %v", name, err)
		}
	})
}

func (s *Suite) logContainer(ctx context.Context, c testcontainers.Container, name string) {
	t := s.T()

	logs, err := c.Logs(ctx)
	if err != nil {
		t.Logf("%s logs unavailable: %v", name, err)
		return
	}
	defer func() { _ = logs.Close() }()

	b, _ := io.ReadAll(logs)
	t.Logf("%s logs:\n%s", name, b)
}

// pinnedHostPort reserves a free host port and returns it with a customizer that
// binds containerPort to it.
//
// Docker otherwise picks the host port at start, and picks a new one on every
// restart. Some containers must be told their externally reachable address before
// they start (Aerospike's tls-alternate-access-port, fake-gcs-server's
// -public-host), which is only possible when the port is chosen up front.
func (s *Suite) pinnedHostPort(containerPort string) (int, testcontainers.ContainerCustomizer) {
	hostPort := s.freeHostPort()

	return hostPort, testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) {
		if hostConfig.PortBindings == nil {
			hostConfig.PortBindings = network.PortMap{}
		}

		hostConfig.PortBindings[network.MustParsePort(containerPort)] = []network.PortBinding{{
			HostIP:   netip.MustParseAddr("127.0.0.1"),
			HostPort: strconv.Itoa(hostPort),
		}}
	})
}

// freeHostPort returns a loopback port that was free a moment ago.
func (s *Suite) freeHostPort() int {
	listener, err := (&net.ListenConfig{}).Listen(s.T().Context(), "tcp4", "127.0.0.1:0")
	s.Require().NoError(err)
	defer func() { _ = listener.Close() }()

	return listener.Addr().(*net.TCPAddr).Port
}
