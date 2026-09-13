package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLS_Hash(t *testing.T) {
	t.Parallel()

	t.Run("nil tls", func(t *testing.T) {
		t.Parallel()

		var tls *TLS
		require.Zero(t, tls.Hash())
	})

	t.Run("key file password change changes hash", func(t *testing.T) {
		t.Parallel()

		first := (&TLS{CAPath: "/ca", KeyfilePassword: "first"}).Hash()
		second := (&TLS{CAPath: "/ca", KeyfilePassword: "second"}).Hash()

		require.NotEqual(t, first, second)
	})
}
