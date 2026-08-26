package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerHTTPConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ServerConfigHTTP
		wantErr string
	}{
		{
			name: "valid config",
			cfg: &ServerConfigHTTP{
				ListenerConfig: ListenerConfig{
					ContextPath:  "/",
					Timeout:      ptr.Of(int64(5000)),
					ReadTimeout:  ptr.Of(int64(30000)),
					WriteTimeout: ptr.Of(int64(60000)),
					IdleTimeout:  ptr.Of(int64(120000)),
				},
			},
		},
		{
			name: "negative timeout",
			cfg: &ServerConfigHTTP{
				ListenerConfig: ListenerConfig{Timeout: ptr.Of(int64(-1))},
			},
			wantErr: "timeout",
		},
		{
			name: "negative read-timeout",
			cfg: &ServerConfigHTTP{
				ListenerConfig: ListenerConfig{ReadTimeout: ptr.Of(int64(-1))},
			},
			wantErr: "read-timeout",
		},
		{
			name: "negative write-timeout",
			cfg: &ServerConfigHTTP{
				ListenerConfig: ListenerConfig{WriteTimeout: ptr.Of(int64(-1))},
			},
			wantErr: "write-timeout",
		},
		{
			name: "negative idle-timeout",
			cfg: &ServerConfigHTTP{
				ListenerConfig: ListenerConfig{IdleTimeout: ptr.Of(int64(-1))},
			},
			wantErr: "idle-timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestServerHTTPConfig_ToModelAndFromModel(t *testing.T) {
	dtoCfg := &ServerConfigHTTP{
		ListenerConfig: ListenerConfig{
			Address:      "127.0.0.1",
			ContextPath:  "/api",
			Timeout:      ptr.Of(int64(5000)),
			ReadTimeout:  ptr.Of(int64(30000)),
			WriteTimeout: ptr.Of(int64(60000)),
			IdleTimeout:  ptr.Of(int64(120000)),
		},
		Port: ptr.Of(Port(9090)),
	}

	modelCfg := dtoCfg.ToModel()
	require.NotNil(t, modelCfg)
	require.Equal(t, int64(5000), modelCfg.Timeout.Milliseconds())
	require.Equal(t, int64(30000), modelCfg.ReadTimeout.Milliseconds())
	require.Equal(t, int64(60000), modelCfg.WriteTimeout.Milliseconds())
	require.Equal(t, int64(120000), modelCfg.IdleTimeout.Milliseconds())

	roundTrip := &ServerConfigHTTP{}
	roundTrip.fromModel(modelCfg)
	require.Equal(t, dtoCfg, roundTrip)
}
