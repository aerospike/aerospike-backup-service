package backupexecutor

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// runXDRBackup performs an XDR-based backup.
func runXDRBackup(
	ctx context.Context,
	client *backup.Client,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	writer backup.Writer,
) (BackupHandler, error) {
	xdrConfig, err := makeXDRConfig(namespace, routine, timeBounds, client.AerospikeClient())
	if err != nil {
		return nil, fmt.Errorf("failed to build XDR configuration: %w", err)
	}

	handler, err := client.BackupXDR(ctx, xdrConfig, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to start XDR backup: %w", err)
	}

	return handler, nil
}

func makeXDRConfig(
	namespace string,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	client backup.AerospikeClient,
) (*backup.ConfigBackupXDR, error) {
	// Every xdr requests starts a server instance that listens for connections from other nodes in the cluster.
	// It needs unique dc name and port.
	dc, err := generateUniqueDCName(client)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique DC name: %w", err)
	}
	port := GetFreePort()

	policy := routine.BackupPolicy
	xdrConfig := policy.XDRConfig
	return &backup.ConfigBackupXDR{
		InfoPolicy:                   as.NewInfoPolicy(),
		EncryptionPolicy:             makeEncryptionPolicy(policy),
		CompressionPolicy:            makeCompressionPolicy(policy),
		SecretAgentConfig:            routine.SecretAgent.ToSecretAgentConfig(),
		EncoderType:                  backup.EncoderTypeASBX,
		FileLimit:                    int64(policy.GetFileLimitOrDefault()) * 1_048_576,
		ParallelWrite:                policy.GetParallelOrDefault(),
		DC:                           dc,
		LocalAddress:                 xdrConfig.LocalHost,
		LocalPort:                    port,
		Namespace:                    namespace,
		Rewind:                       getRewind(timeBounds),
		TLSConfig:                    nil,
		ReadTimeoutMilliseconds:      xdrConfig.GetReadTimeoutOrDefault(),
		WriteTimeoutMilliseconds:     xdrConfig.GetWriteTimeoutOrDefault(),
		ResultQueueSize:              xdrConfig.GetResultQueueSizeOrDefault(),
		AckQueueSize:                 xdrConfig.GetAckQueueSizeOrDefault(),
		MaxConnections:               xdrConfig.GetMaxConnsOrDefault(),
		InfoPolingPeriodMilliseconds: xdrConfig.GetPollingPeriodOrDefault(),
		StartTimeoutMilliseconds:     xdrConfig.GetStartTimeoutOrDefault(),
		InfoRetryPolicy:              xdrConfig.GetInfoRetryPolicyOrDefault(),
	}, nil
}

func GetFreePort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		return 0
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

var dcCounter atomic.Int64

func generateUniqueDCName(client backup.AerospikeClient) (string, error) {
	existingDCs, err := aerospike.GetDCNames(client)
	if err != nil {
		return "", fmt.Errorf("failed to get existing DC names: %w", err)
	}

	// Try a reasonable number of times to avoid infinite loop
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("dc%d", dcCounter.Add(1)) // each time generate a different name
		if !slices.Contains(existingDCs, name) {
			return name, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique DC name after 1000 attempts")
}

// getRewind calculates the rewind value based on FromTime.
func getRewind(bounds model.TimeBounds) string {
	if bounds.FromTime == nil {
		return "all"
	}
	seconds := int(time.Since(*bounds.FromTime).Seconds()) + 1

	return fmt.Sprintf("%d", seconds)
}
