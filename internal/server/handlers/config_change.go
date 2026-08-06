package handlers

import (
	"context"
	"fmt"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/validation"
)

type BackupConfigChangeOptions struct {
	validateNamespaces bool
}

// ConfigManagerImpl applies mutations to the configuration.
type ConfigManagerImpl struct {
	config               *model.Config
	changeConfigLock     *sync.Mutex
	nsValidator          aerospike.NamespaceValidator
	configurationManager configuration.Manager
	configApplier        service.ConfigApplier
	sysCtx               context.Context //nolint:containedctx
}

// NewConfigManagerImpl creates a new configuration manager.
func NewConfigManagerImpl(
	sysCtx context.Context,
	config *model.Config,
	changeConfigLock *sync.Mutex,
	nsValidator aerospike.NamespaceValidator,
	configurationManager configuration.Manager,
	configApplier service.ConfigApplier,
) *ConfigManagerImpl {
	return &ConfigManagerImpl{
		config:               config,
		changeConfigLock:     changeConfigLock,
		nsValidator:          nsValidator,
		configurationManager: configurationManager,
		configApplier:        configApplier,
		sysCtx:               sysCtx,
	}
}

// ChangeBackupConfig applies a mutation to the backup configuration DTO, validates the
// full configuration via ToModel, and persists the result.
// The mutate function returns routine names that should be rescheduled and rescanned.
func (s *ConfigManagerImpl) ChangeBackupConfig(
	ctx context.Context,
	action string,
	resourceID string,
	mutate func(*dto.Config) ([]string, error),
	opts ...func(*BackupConfigChangeOptions),
) error {
	options := BackupConfigChangeOptions{}
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

	modelConfig, err := dtoConfig.ToModel(dto.ValidationSkipTLSFiles)
	if err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	s.config.SetBackupConfig(modelConfig.BackupConfigCopy())
	s.config.InvalidateRoutines(routinesToInvalidate)

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

func WithNamespaceValidation(opts *BackupConfigChangeOptions) {
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

// UpdateConfig updates the configuration for the service.
func (s *ConfigManagerImpl) UpdateConfig(ctx context.Context, newConfig *dto.Config) error {
	// validate static fields.
	oldConfig := dto.NewConfigFromModel(s.config)
	if err := validation.ValidateStaticFieldChanges(oldConfig, newConfig); err != nil {
		return fmt.Errorf("static configuration has changed: %w", err)
	}

	newConfigModel, err := newConfig.ToModel(dto.ValidationSkipTLSFiles) // matching what UpdateConfig did
	if err != nil {
		return err
	}

	return s.changeConfig(ctx, func(config *model.Config) error {
		config.SetBackupConfig(newConfigModel.BackupConfigCopy())
		config.InvalidateAllRoutines()
		s.nsValidator.Validate(ctx, config) // validate under the lock
		return nil
	})
}

// ApplyConfig reloads the configuration from the config file.
func (s *ConfigManagerImpl) ApplyConfig(ctx context.Context) error {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	config, err := s.configurationManager.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %w", err)
	}

	newConfig := dto.NewConfigFromModel(s.config)
	oldConfig := dto.NewConfigFromModel(config)
	if err := validation.ValidateStaticFieldChanges(oldConfig, newConfig); err != nil {
		return fmt.Errorf("static configuration has changed: %w", err)
	}

	s.config.SetBackupConfig(config.BackupConfigCopy())
	s.config.InvalidateAllRoutines()
	err = s.configApplier.ApplyNewConfig(s.sysCtx)

	return err
}

func (s *ConfigManagerImpl) changeConfig(ctx context.Context, updateFunc func(*model.Config) error) error {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()

	err := updateFunc(s.config)
	if err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	err = s.configurationManager.Write(ctx, s.config)
	if err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	err = s.configApplier.ApplyNewConfig(s.sysCtx)
	if err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}
