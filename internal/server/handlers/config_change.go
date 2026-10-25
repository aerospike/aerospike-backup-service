package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/validation"
)

// configWriteTimeout bounds the persist step of a configuration change: long enough for a cold
// cloud client, short enough that a stalled backend cannot hold the config lock indefinitely.
const configWriteTimeout = 60 * time.Second

// changeBackupConfig applies a mutation to the backup configuration DTO, validates and
// converts the full configuration, and persists the result before it becomes visible.
// The mutate function returns routine names that should be rescheduled and rescanned.
// It is the one path for every configuration change, whether it edits an entity or, as
// PUT /v1/config does, replaces the whole document.
func (s *Service) changeBackupConfig(
	ctx context.Context,
	mutate func(*dto.Config) ([]string, error),
) error {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	// Two conversions of the live configuration: mutate edits the candidate in place, and the
	// secret merge and the static-field check need the untouched one to compare against.
	existing := dto.NewConfigFromModel(s.config)
	candidate := dto.NewConfigFromModel(s.config)

	routinesToInvalidate, err := mutate(candidate)
	if err != nil {
		return err
	}

	// GET responses redact secrets as "[secret]". Before comparing or persisting, copy real secret
	// values from the stored config into the incoming payload wherever the sentinel appears,
	// so a GET-edit-PUT round trip does not overwrite secrets with the literal "[secret]".
	if err := decoder.MergeSecrets(candidate, existing); err != nil {
		return errBadRequest(err)
	}

	// The service section cannot change at runtime. Checked after the merge, so a redacted
	// secret does not count as a change; a no-op for an entity change, which keeps the section.
	if err := validation.ValidateStaticFieldChanges(existing, candidate); err != nil {
		return errBadRequest(fmt.Errorf("static configuration has changed: %w", err))
	}

	if err := candidate.Validate(); err != nil {
		return errBadRequest(err)
	}

	next, err := candidate.ToModel()
	if err != nil {
		return errBadRequest(err)
	}
	if err := s.tlsProber.Probe(ctx, next); err != nil {
		return errBadRequest(err)
	}

	return s.commitConfig(ctx, next.BackupConfigCopy(), routinesToInvalidate)
}

// commitConfig persists next and only then makes it the live backup configuration and
// reschedules the named routines. A change that did not reach the configuration source never
// becomes visible, so memory, file and scheduler cannot come to describe different
// configurations. The service configuration is static, so the live one is what gets persisted.
//
// The caller holds changeConfigLock.
func (s *Service) commitConfig(ctx context.Context, next *model.BackupConfig, routinesToInvalidate []string) error {
	// A write the client can cancel would leave memory, file and scheduler describing different
	// configurations, so the persist outlives the request and its own timeout bounds it instead.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configWriteTimeout)
	defer cancel()

	if err := s.configurationManager.Write(writeCtx, s.config.WithBackupConfig(next)); err != nil {
		return errNotPersisted(err)
	}

	s.config.SetBackupConfig(next)
	s.config.InvalidateRoutines(routinesToInvalidate)

	if err := s.configApplier.ApplyNewConfig(ctx); err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}
