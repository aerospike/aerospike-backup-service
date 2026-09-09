package secrets

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestKeyfilePasswordResolverSkipsResolverWhenEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := NewMockResolver(ctrl) // no EXPECT(): Resolve must not be called

	input := model.TLS{ClientTLS: model.ClientTLS{CAFile: "ca.pem"}}
	result, err := NewKeyfilePasswordResolver(resolver).Resolve(t.Context(), input, nil)
	require.NoError(t, err)
	require.Equal(t, input, result)
}

func TestKeyfilePasswordResolverResolvesThroughAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	agent := &model.SecretAgent{Address: "127.0.0.1"}
	resolver := NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), agent, "secrets:agent:key").
		Return("real-password", nil)

	result, err := NewKeyfilePasswordResolver(resolver).Resolve(
		t.Context(), model.TLS{KeyfilePassword: "secrets:agent:key"}, agent,
	)
	require.NoError(t, err)
	require.Equal(t, "real-password", result.KeyfilePassword)
}

func TestKeyfilePasswordResolverPropagatesResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", errors.New("secret agent unreachable"))

	_, err := NewKeyfilePasswordResolver(resolver).Resolve(
		t.Context(), model.TLS{KeyfilePassword: "secrets:agent:key"}, nil,
	)
	require.Error(t, err)
}

func TestKeyfilePasswordResolverPreservesOtherFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	resolver := NewMockResolver(ctrl)
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("resolved", nil)

	input := model.TLS{
		ClientTLS: model.ClientTLS{
			CAFile:   "ca.pem",
			Certfile: "cert.pem",
			Keyfile:  "key.pem",
			Name:     "tls-name",
		},
		Protocols:       "TLSv1.2",
		KeyfilePassword: "secrets:agent:key",
	}

	result, err := NewKeyfilePasswordResolver(resolver).Resolve(t.Context(), input, nil)
	require.NoError(t, err)
	require.Equal(t, "ca.pem", result.CAFile)
	require.Equal(t, "cert.pem", result.Certfile)
	require.Equal(t, "key.pem", result.Keyfile)
	require.Equal(t, "tls-name", result.Name)
	require.Equal(t, "TLSv1.2", result.Protocols)
	require.Equal(t, "resolved", result.KeyfilePassword)
}
