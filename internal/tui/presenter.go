package tui

import (
	"fmt"
	"time"

	m "github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

func presentClusterLine(cfg *m.Config, routine string) string {
	return fmt.Sprintf("description for %s", routine)
}

// ---------- Backup rows ----------

func presentBackupLine(b m.BackupDetails) string {
	t := b.Created.UTC().Format("2006-01-02 15:04:01")
	typ := "Full"
	if !b.IsFull() {
		typ = "Incr"
	}
	src := "Manual"
	if b.Created.UnixMilli()%1000 == 0 {
		src = "Scheduled"
	}
	size := humanSize(b.ByteCount)

	return fmt.Sprintf("BD %s  %s  (%s)   %s", t, typ, src, size)
}

func presentRunningMeta(bkp any, eta time.Duration) string {
	b := bkp.(m.BackupDetails)
	if eta > 0 {
		return fmt.Sprintf("ETA %s  Started: %s", eta.Truncate(time.Second).String(), b.Created)
	}
	return fmt.Sprintf("Started: %s", b.Created)
}

func buildRestoreRequest(routine string, b *m.BackupDetails) m.RestoreTimestampRequest {
	var req m.RestoreTimestampRequest
	req.RoutineName = routine
	req.Time = b.Created
	return req
}

// ---------- Human size ----------

func humanSize(n uint64) string {
	const (
		Ki = 1 << 10
		Mi = 1 << 20
		Gi = 1 << 30
		Ti = 1 << 40
	)
	switch {
	case n >= Ti:
		return fmt.Sprintf("%.1f TiB", float64(n)/Ti)
	case n >= Gi:
		return fmt.Sprintf("%.1f GiB", float64(n)/Gi)
	case n >= Mi:
		return fmt.Sprintf("%.1f MiB", float64(n)/Mi)
	case n >= Ki:
		return fmt.Sprintf("%.1f KiB", float64(n)/Ki)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
