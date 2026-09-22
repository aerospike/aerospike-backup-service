package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
)

// configWriteTimeout bounds the persist step of a configuration change: long enough for a cold
// cloud client, short enough that a stalled backend cannot hold the config lock indefinitely.
const configWriteTimeout = 60 * time.Second

type backupConfigChangeOptions struct {
	validateNamespaces bool
}

// changeBackupConfig applies a mutation to the backup configuration DTO, validates and
// converts the full configuration, and persists the result.
// The mutate function returns routine names that should be rescheduled and rescanned.
func (s *Service) changeBackupConfig(
	ctx context.Context,
	mutate func(*dto.Config) ([]string, error),
	opts ...func(*backupConfigChangeOptions),
) error {
	options := backupConfigChangeOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	dtoConfig := dto.NewConfigFromModel(s.config)
	routinesToInvalidate, err := mutate(dtoConfig)
	if err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	// GET responses redact secrets as "[secret]". Before persisting a PUT, copy real secret
	// values from the stored config into the incoming payload wherever the sentinel appears,
	// so a GET-edit-PUT round trip does not overwrite secrets with the literal "[secret]".
	existingConfig := dto.NewConfigFromModel(s.config)
	if err := decoder.MergeSecrets(dtoConfig, existingConfig); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	if err := dtoConfig.Validate(); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	modelConfig, err := dtoConfig.ToModel()
	if err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}
	if err := s.tlsProber.Probe(ctx, modelConfig); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	s.config.SetBackupConfig(modelConfig.BackupConfigCopy())
	s.config.InvalidateRoutines(routinesToInvalidate)

	if options.validateNamespaces {
		s.nsValidator.Validate(ctx, s.config)
	}

	// A write the client can cancel would leave memory, file and scheduler describing different
	// configurations, so the persist outlives the request and its own timeout bounds it instead.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configWriteTimeout)
	defer cancel()

	if err = s.configurationManager.Write(writeCtx, s.config); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	if err = s.configApplier.ApplyNewConfig(); err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}

func withNamespaceValidation(opts *backupConfigChangeOptions) {
	opts.validateNamespaces = true
}
