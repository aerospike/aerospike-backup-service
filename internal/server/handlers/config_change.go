package handlers

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type backupConfigChangeOptions struct {
	validateNamespaces bool
}

// changeBackupConfig applies a mutation to the backup configuration DTO, validates the
// full configuration via ToModel, and persists the result.
func (s *Service) changeBackupConfig(
	ctx context.Context,
	mutate func(*dto.Config) error,
	opts ...func(*backupConfigChangeOptions),
) error {
	options := backupConfigChangeOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	dtoConfig := dto.NewConfigFromModel(s.config)
	if err := mutate(dtoConfig); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	modelConfig, err := dtoConfig.ToModel(dto.ValidationSkipTLSFiles)
	if err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	s.config.SetBackupConfig(modelConfig.BackupConfigCopy())

	if options.validateNamespaces {
		s.nsValidator.Validate(ctx, s.config)
	}

	if err = s.configurationManager.Write(ctx, s.config); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	if err = s.configApplier.ApplyNewConfig(s.sysCtx); err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}

func withNamespaceValidation(opts *backupConfigChangeOptions) {
	opts.validateNamespaces = true
}

func ensurePolicyNotInUse(config *dto.Config, policyName string) error {
	for routineName, routine := range config.BackupRoutines {
		if routine != nil && routine.BackupPolicy == policyName {
			return fmt.Errorf("delete backup policy %q: %w: it is used in routine %q", policyName, model.ErrInUse, routineName)
		}
	}
	return nil
}

func ensureClusterNotInUse(config *dto.Config, clusterName string) error {
	for routineName, routine := range config.BackupRoutines {
		if routine != nil && routine.SourceCluster == clusterName {
			return fmt.Errorf(
				"delete Aerospike cluster %q: %w: it is used in routine %q", clusterName, model.ErrInUse, routineName)
		}
	}
	return nil
}

func ensureStorageNotInUse(config *dto.Config, storageName string) error {
	for routineName, routine := range config.BackupRoutines {
		if routine != nil && routine.Storage == storageName {
			return fmt.Errorf("delete storage %q: %w: it is used in routine %q", storageName, model.ErrInUse, routineName)
		}
	}
	return nil
}
