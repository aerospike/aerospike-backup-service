package model

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/require"
)

func TestAerospikeCluster_Hash(t *testing.T) {
	t.Parallel()

	base := &AerospikeCluster{
		ClusterLabel: "cluster-a",
		SeedNodes: []SeedNode{
			{HostName: "host-b", Port: 3000},
			{HostName: "host-a", Port: 3000},
		},
		ConnTimeout:          ptr.Of(5 * time.Second),
		UseServicesAlternate: ptr.Of(true),
		Credentials: &Credentials{
			User:     "user",
			Password: "password",
		},
		TLS: &TLS{
			CAPath:          "/etc/certs",
			KeyfilePassword: "tls-password",
		},
		MaxParallelScans: ptr.Of(10),
		PreferRacks:      []int{1, 2},
	}

	t.Run("stable for identical config", func(t *testing.T) {
		t.Parallel()

		other := *base
		other.SeedNodes = []SeedNode{
			{HostName: "host-a", Port: 3000},
			{HostName: "host-b", Port: 3000},
		}

		require.Equal(t, base.Hash(), other.Hash())
	})

	t.Run("different password produces different hash", func(t *testing.T) {
		t.Parallel()

		other := *base
		other.Credentials = &Credentials{
			User:     base.Credentials.User,
			Password: "other-password",
		}

		require.NotEqual(t, base.Hash(), other.Hash())
	})

	t.Run("nil cluster", func(t *testing.T) {
		t.Parallel()

		var cluster *AerospikeCluster
		require.Zero(t, cluster.Hash())
	})
}

func TestCredentials_Hash(t *testing.T) {
	t.Parallel()

	t.Run("nil credentials", func(t *testing.T) {
		t.Parallel()

		var creds *Credentials
		require.Zero(t, creds.Hash())
	})

	t.Run("nil auth mode differs from explicit", func(t *testing.T) {
		t.Parallel()

		withDefault := (&Credentials{User: "user"}).Hash()
		withInternal := (&Credentials{User: "user", AuthMode: ptr.Of(AuthModeInternal)}).Hash()

		require.NotEqual(t, withDefault, withInternal)
	})
}
