package secrets

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

// TestResolverLiteral verifies that nil agents pass through literal values unchanged.
func TestResolverLiteral(t *testing.T) {
	resolver := NewResolver()
	result, err := resolver.Resolve(t.Context(), nil, "secret-value")
	require.NoError(t, err)
	require.Equal(t, "secret-value", result)
}

// TestResolverNoCache verifies that repeated Secret Agent resolution is not cached;
// each call independently reaches the agent and gets a different response.
func TestResolverNoCache(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	connCount := atomic.Int32{}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			n := int(connCount.Add(1))
			go respondWithSecret(conn, n)
		}
	}()

	resolver := NewResolver()
	agent := &model.SecretAgent{
		Address:        "127.0.0.1",
		Port:           ptr.Of(model.Port(addr.Port)),
		ConnectionType: "tcp",
		Timeout:        ptr.Of(1000),
	}

	result1, err := resolver.Resolve(t.Context(), agent, "secrets:agent:key")
	require.NoError(t, err)
	require.Equal(t, "value-1", result1)

	// Second call should NOT be cached; agent returns different value
	result2, err := resolver.Resolve(t.Context(), agent, "secrets:agent:key")
	require.NoError(t, err)
	require.Equal(t, "value-2", result2)

	// Both calls made separate connections
	require.Equal(t, int32(2), connCount.Load())
}

// TestResolverFailClosed verifies that resolution errors propagate without stale-secret fallback.
func TestResolverFailClosed(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := listener.Addr().(*net.TCPAddr)
	reqCount := atomic.Int32{}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if reqCount.Add(1) == 1 {
				go respondWithSecret(conn, 1)
			} else {
				conn.Close()
			}
		}
	}()

	resolver := NewResolver()
	agent := &model.SecretAgent{
		Address:        "127.0.0.1",
		Port:           ptr.Of(model.Port(addr.Port)),
		ConnectionType: "tcp",
		Timeout:        ptr.Of(1000),
	}

	result, err := resolver.Resolve(t.Context(), agent, "secrets:agent:key")
	require.NoError(t, err)
	require.Equal(t, "value-1", result)

	listener.Close()

	// Second call should fail, not return cached "value-1"
	_, err = resolver.Resolve(t.Context(), agent, "secrets:agent:key")
	require.Error(t, err)
}

// respondWithSecret implements Secret Agent protocol for testing.
func respondWithSecret(conn net.Conn, num int) {
	defer conn.Close()

	header := make([]byte, 8)
	if _, err := conn.Read(header); err != nil {
		return
	}

	if binary.BigEndian.Uint32(header[0:4]) != 0x51dec1cc {
		return
	}

	length := binary.BigEndian.Uint32(header[4:8])
	if _, err := conn.Read(make([]byte, length)); err != nil {
		return
	}

	var response []byte
	response = append(response, `{"SecretValue":"value-`...)
	response = fmt.Appendf(response, `%d"}`, num)

	respHeader := make([]byte, 8)
	binary.BigEndian.PutUint32(respHeader[0:4], 0x51dec1cc)
	binary.BigEndian.PutUint32(respHeader[4:8], uint32(len(response)))

	conn.Write(respHeader)
	conn.Write(response)
}
