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
	"github.com/aerospike/backup-go"
)

// runXDRBackup performs an XDR-based backup.
func runXDRBackup(
	ctx context.Context,
	client aerospike.Backuper,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	writer backup.Writer,
) (BackupHandler, error) {
	xdrConfig, err := makeXDRConfig(ctx, namespace, routine, timeBounds, client.InfoClient())
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
	ctx context.Context,
	namespace string,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	infoGetter backup.InfoGetter,
) (*backup.ConfigBackupXDR, error) {
	// Every xdr requests starts a server instance that listens for connections from other nodes in the cluster.
	// It needs unique dc name and port.
	dc, err := generateUniqueDCName(ctx, infoGetter)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique DC name: %w", err)
	}
	port, err := getFreePortInRange(routine.BackupPolicy.XDRConfig.PortRange)
	if err != nil {
		return nil, fmt.Errorf("failed to find free port: %w", err)
	}

	policy := routine.BackupPolicy
	xdrConfig := policy.XDRConfig

	return &backup.ConfigBackupXDR{
		EncryptionPolicy:  makeEncryptionPolicy(policy),
		CompressionPolicy: makeCompressionPolicy(policy),
		SecretAgentConfig: routine.SecretAgent.ToSecretAgentConfig(),
		EncoderType:       backup.EncoderTypeASBX,
		FileLimit:         uint64(policy.GetFileLimitOrDefault()) * megabyte,
		ParallelWrite:     policy.GetParallelOrDefault(),
		DC:                dc,
		LocalAddress:      xdrConfig.LocalHost,
		LocalPort:         int(port),
		Namespace:         namespace,
		Rewind:            getRewind(timeBounds),
		TLSConfig:         nil,
		ReadTimeout:       xdrConfig.GetReadTimeoutOrDefault(),
		WriteTimeout:      xdrConfig.GetWriteTimeoutOrDefault(),
		ResultQueueSize:   xdrConfig.GetResultQueueSizeOrDefault(),
		AckQueueSize:      xdrConfig.GetAckQueueSizeOrDefault(),
		MaxConnections:    xdrConfig.GetMaxConnsOrDefault(),
		InfoPollingPeriod: xdrConfig.GetPollingPeriodOrDefault(),
		StartTimeout:      xdrConfig.GetStartTimeoutOrDefault(),
	}, nil
}

// getFreePortInRange finds a free TCP port within the specified range and listens on all interfaces.
func getFreePortInRange(r *model.PortRange) (model.Port, error) {
	if r == nil {
		return getFreePort()
	}

	for port := r.Start; port <= r.End; port++ {
		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free ports available in range %d-%d", r.Start, r.End)
}

// isPortAvailable checks if the port is available.
func isPortAvailable(port model.Port) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

// getFreePort finds a free TCP port on localhost.
func getFreePort() (model.Port, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	return model.Port(port), nil
}

var (
	dcCounter    atomic.Int32
	limit              = 1000
	dcUpperBound int32 = 10_000
)

func generateUniqueDCName(ctx context.Context, infoGetter backup.InfoGetter) (string, error) {
	existingDCs, err := infoGetter.GetDCsList(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get existing DC names: %w", err)
	}

	// Try a reasonable number of times to avoid infinite loop
	for i := 0; i < limit; i++ {
		name := fmt.Sprintf("abs_dc%d", dcCounter.Add(1)%dcUpperBound) // each time generate a different name
		if !slices.Contains(existingDCs, name) {
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
