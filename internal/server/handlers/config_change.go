package handlers

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// changeBackupConfig applies a mutation to the backup configuration DTO, validates and
// converts the full configuration, and persists the result.
// The mutate function returns routine names that should be rescheduled and rescanned.
//
// Every change is followed by the advisory preflight check, which probes whatever the
// change added or altered. No handler decides whether its change is worth validating:
// a change that reaches nothing new produces an empty delta and probes nothing.
func (s *Service) changeBackupConfig(
	ctx context.Context,
	mutate func(*dto.Config) ([]string, error),
) error {
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

	previous := configSnapshot(s.config)
	s.config.SetBackupConfig(modelConfig.BackupConfigCopy())
	s.config.InvalidateRoutines(routinesToInvalidate)

	s.checker.CheckChanges(ctx, previous, s.config) // validate under the lock

	if err = s.configurationManager.Write(ctx, s.config); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	if err = s.configApplier.ApplyNewConfig(); err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}

// configSnapshot captures the backup configuration as it stands, so that a change can be
// compared against what it replaced. Every change round trips through the DTO layer, which
// allocates fresh entities, so the entities a snapshot points at are never the ones a later
// change mutates.
func configSnapshot(config *model.Config) *model.Config {
	snapshot := model.NewConfig()
	snapshot.SetBackupConfig(config.BackupConfigCopy())

	return snapshot
}

func routinesUsingStorage(config *dto.Config, storageName string) []string {
	var names []string
	for name, routine := range config.BackupRoutines {
		if routine != nil && routine.Storage == storageName {
			names = append(names, name)
		}
	}
	return names
}

func routinesUsingCluster(config *dto.Config, clusterName string) []string {
	var names []string
	for name, routine := range config.BackupRoutines {
		if routine != nil && routine.SourceCluster == clusterName {
			names = append(names, name)
		}
	}
	return names
}

func routinesUsingPolicy(config *dto.Config, policyName string) []string {
	var names []string
	for name, routine := range config.BackupRoutines {
		if routine != nil && routine.BackupPolicy == policyName {
			names = append(names, name)
		}
	}
	return names
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
