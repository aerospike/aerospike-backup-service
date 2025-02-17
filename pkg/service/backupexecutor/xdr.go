package backupexecutor

import (
	"context"
	"fmt"
	"log/slog"
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

	slog.Info("start XDR backup", slog.Any("config", *xdrConfig))
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
	port, err := getFreePortInRange(5000, 6000)
	if err != nil {
		return nil, fmt.Errorf("failed to find free port: %w", err)
	}

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

// getFreePortInRange finds a free TCP port within the specified range and listens on all interfaces.
func getFreePortInRange(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port)) // Bind to all interfaces
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free ports available in range %d-%d", start, end)
}

var (
	dcCounter    atomic.Int32
	limit              = 1000
	dcUpperBound int32 = 10_000
)

func generateUniqueDCName(client backup.AerospikeClient) (string, error) {
	existingDCs, err := aerospike.GetDCNames(client)
	if err != nil {
		return "", fmt.Errorf("failed to get existing DC names: %w", err)
	}
	slog.Info("XDR: existing DC", slog.Any("dc", existingDCs))

	// Try a reasonable number of times to avoid infinite loop
	for i := 0; i < limit; i++ {
		name := fmt.Sprintf("abs_dc%d", dcCounter.Add(1)%dcUpperBound) // each time generate a different name
		if !slices.Contains(existingDCs, name) {
			slog.Info("XDR: Create unique DC", slog.String("dc", name))
			return name, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique DC name after %d attempts", limit)
}

// getRewind calculates the rewind value based on FromTime.
// The returned value is the string representation of seconds since bounds.FromTime rounded up.
func getRewind(bounds model.TimeBounds) string {
	if bounds.FromTime == nil {
		return "all"
	}
	seconds := int(time.Since(*bounds.FromTime).Seconds()) + 1

	return fmt.Sprintf("%d", seconds)
}
