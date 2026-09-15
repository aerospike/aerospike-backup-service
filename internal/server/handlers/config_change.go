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

type backupConfigChangeOptions struct {
	validateNamespaces bool
}

// changeBackupConfig applies a mutation to the backup configuration DTO, validates and
// converts the full configuration, and persists the result before it becomes visible.
// The mutate function returns routine names that should be rescheduled and rescanned.
//
// The returned error carries the status code the client should see: a rejected change is a
// bad request, a change that could not be persisted is not.
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
		return errBadRequest(fmt.Errorf("failed to update configuration: %w", err))
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

	if options.validateNamespaces {
		s.nsValidator.Validate(ctx, candidate)
	}

	return s.commitConfig(ctx, candidate, routinesToInvalidate)
}

// commitConfig persists candidate and only then makes it the live configuration and
// reschedules the named routines. A change that did not reach the configuration file never
// becomes visible, so memory, file and scheduler cannot come to describe different
// configurations.
//
// The caller holds changeConfigLock.
func (s *Service) commitConfig(ctx context.Context, candidate *model.Config, routinesToInvalidate []string) error {
	// A write the client can cancel would leave memory, file and scheduler describing different
	// configurations, so the persist outlives the request and its own timeout bounds it instead.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configWriteTimeout)
	defer cancel()

	if err := s.configurationManager.Write(writeCtx, candidate); err != nil {
		return errStorageUnavailable(fmt.Errorf("failed to write configuration: %w", err))
	}

	s.config.SetBackupConfig(candidate.BackupConfigCopy())
	s.config.InvalidateRoutines(routinesToInvalidate)

	if err := s.configApplier.ApplyNewConfig(); err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}

func withNamespaceValidation(opts *backupConfigChangeOptions) {
	opts.validateNamespaces = true
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
