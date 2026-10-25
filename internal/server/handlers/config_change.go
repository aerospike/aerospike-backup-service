package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// configWriteTimeout bounds the persist step of a configuration change: long enough for a cold
// cloud client, short enough that a stalled backend cannot hold the config lock indefinitely.
const configWriteTimeout = 60 * time.Second

// changeBackupConfig applies a mutation to the backup configuration DTO, validates and
// converts the full configuration, and persists the result before it becomes visible.
// The mutate function returns routine names that should be rescheduled and rescanned.
func (s *Service) changeBackupConfig(
	ctx context.Context,
	mutate func(*dto.Config) ([]string, error),
) error {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	dtoConfig := dto.NewConfigFromModel(s.config)
	routinesToInvalidate, err := mutate(dtoConfig)
	if err != nil {
		return err
	}

	// GET responses redact secrets as "[secret]". Before persisting a PUT, copy real secret
	// values from the stored config into the incoming payload wherever the sentinel appears,
	// so a GET-edit-PUT round trip does not overwrite secrets with the literal "[secret]".
	existingConfig := dto.NewConfigFromModel(s.config)
	if err := decoder.MergeSecrets(dtoConfig, existingConfig); err != nil {
		return errBadRequest(fmt.Errorf("failed to update configuration: %w", err))
	}

	if err := dtoConfig.Validate(); err != nil {
		return errBadRequest(fmt.Errorf("failed to update configuration: %w", err))
	}

	candidate, err := dtoConfig.ToModel()
	if err != nil {
		return errBadRequest(fmt.Errorf("failed to update configuration: %w", err))
	}
	if err := s.tlsProber.Probe(ctx, candidate); err != nil {
		return errBadRequest(fmt.Errorf("failed to update configuration: %w", err))
	}

	return s.commitConfig(ctx, candidate.BackupConfigCopy(), routinesToInvalidate)
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
