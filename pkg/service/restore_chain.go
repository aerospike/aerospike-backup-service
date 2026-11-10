package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestoreChain is the backups that restore a namespace to a point in time: its latest full
// backup up to that time and the incremental backups after it, oldest first.
type RestoreChain struct {
	Full         model.BackupDetails
	Incrementals []model.BackupDetails
}

// Backups returns the backups of the chain, the full backup first.
func (c RestoreChain) Backups() []model.BackupDetails {
	return append([]model.BackupDetails{c.Full}, c.Incrementals...)
}

// FindRestoreChains returns the restore chain of each namespace of the request's routine, from
// the backups reader lists.
func FindRestoreChains(
	ctx context.Context, reader BackupReader, request *model.RestoreTimestampRequest,
) (map[string]RestoreChain, error) {
	fullBackups, err := reader.GetBackups(ctx, // find all full backups completed by given restore request time.
		newRoutineBackupFilter(request.RoutineName, request.Storage, model.BackupTypeFull).
			WithToTime(request.Time),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read full backups: %w", err)
	}

	if len(fullBackups) == 0 {
		return nil, errors.New("no full backups found")
	}

	// map namespace -> latest full backup for this ns
	latestFullByNamespace := latestBackupsByNamespace(fullBackups)
	// earliest full backup among all
	earliestSelectedFull := earliestSelectedFullBackup(latestFullByNamespace)

	// Strategy:
	// 1) choose the latest full backup per namespace,
	// 2) read incrementals once from the earliest of those selected full backups,
	// 3) then filter incrementals per namespace against that namespace's selected full.
	// This keeps storage reads batched while still producing correct per-namespace chains.
	incrementalBackups, err := reader.GetBackups(ctx,
		newRoutineBackupFilter(request.RoutineName, request.Storage, model.BackupTypeIncremental).
			WithFromTime(earliestSelectedFull.Created). // lower bound for a single batched incremental scan
			WithToTime(request.Time))
	if err != nil {
		return nil, fmt.Errorf("failed to find incremental backups: %w", err)
	}

	incrementalsByNamespace := splitByNamespace(incrementalBackups)

	return buildRestoreChainsByNamespace(latestFullByNamespace, incrementalsByNamespace), nil
}

// latestBackupsByNamespace picks the newest backup per namespace from the provided list.
func latestBackupsByNamespace(backups []model.BackupDetails) map[string]model.BackupDetails {
	latestByNamespace := make(map[string]model.BackupDetails, len(backups))
	for _, b := range backups {
		current, exists := latestByNamespace[b.Namespace]
		if !exists || current.Created.Before(b.Created) {
			latestByNamespace[b.Namespace] = b
		}
	}

	return latestByNamespace
}

// earliestSelectedFullBackup returns the oldest backup from selected per-namespace full backups.
func earliestSelectedFullBackup(selectedFullByNamespace map[string]model.BackupDetails) model.BackupDetails {
	backups := slices.Collect(maps.Values(selectedFullByNamespace))
	return slices.MinFunc(backups, func(a, b model.BackupDetails) int {
		return a.Created.Compare(b.Created)
	})
}

// buildRestoreChainsByNamespace constructs the restore chain of each namespace: its selected
// full backup and the eligible incrementals after it, in chronological order.
func buildRestoreChainsByNamespace(
	latestFullByNamespace map[string]model.BackupDetails,
	incrementalsByNamespace map[string][]model.BackupDetails,
) map[string]RestoreChain {
	chains := make(map[string]RestoreChain, len(latestFullByNamespace))
	for ns, full := range latestFullByNamespace {
		chains[ns] = RestoreChain{
			Full:         full,
			Incrementals: filterIncrementals(full, incrementalsByNamespace[ns]),
		}
	}

	return chains
}

// AllBackups returns the backups of every chain.
func AllBackups(chains map[string]RestoreChain) []model.BackupDetails {
	var backups []model.BackupDetails
	for _, chain := range chains {
		backups = append(backups, chain.Backups()...)
	}

	return backups
}

func filterIncrementals(
	full model.BackupDetails, incrementals []model.BackupDetails,
) []model.BackupDetails {
	// Sort incrementals descending by Created
	slices.SortFunc(incrementals, func(a, b model.BackupDetails) int {
		return b.Created.Compare(a.Created)
	})

	filteredIncr := make([]model.BackupDetails, 0, len(incrementals))
	var coveredUntil time.Time

	for _, incr := range incrementals {
		// Apply only incrementals strictly newer than the selected full backup.
		if !incr.Created.After(full.Created) {
			continue
		}

		// If this incremental is fully covered by newer ones, skip it.
		if !coveredUntil.IsZero() && !incr.From.Before(coveredUntil) {
			continue
		}

		filteredIncr = append(filteredIncr, incr)
		coveredUntil = updateCoveredUntil(coveredUntil, incr.From)
	}

	// Append filtered incrementals in chronological order
	slices.Reverse(filteredIncr)

	return filteredIncr
}

func updateCoveredUntil(current, incrFrom time.Time) time.Time {
	if incrFrom.IsZero() {
		return current
	}
	if current.IsZero() || incrFrom.Before(current) {
		return incrFrom
	}
	return current
}

func splitByNamespace(backups []model.BackupDetails) map[string][]model.BackupDetails {
	backupsByNamespace := make(map[string][]model.BackupDetails)
	for _, b := range backups {
		backupsByNamespace[b.Namespace] = append(backupsByNamespace[b.Namespace], b)
	}

	return backupsByNamespace
}

// LogChainGaps warns about every gap between a full backup and the incremental backups after it.
func LogChainGaps(chains map[string]RestoreChain, logger *slog.Logger) {
	for ns, chain := range chains {
		backups := chain.Backups()

		for i := 0; i < len(backups)-1; i++ {
			prev := backups[i]
			next := backups[i+1]

			if prev.Created.Before(next.From) {
				msg := "Gap detected in incremental backup chain"
				if i == 0 {
					msg = "Gap detected between full backup and first incremental"
				}

				logger.Warn(msg,
					slog.Time("gapStart", prev.Created),
					slog.Time("gapEnd", next.From),
					slog.String("namespace", ns))
			}
		}
	}
}
