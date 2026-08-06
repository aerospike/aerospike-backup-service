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

func (s *ConfigManagerImpl) ReadConfig(_ context.Context) *dto.Config {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	return dto.NewConfigFromModel(s.config)
}

func (s *ConfigManagerImpl) ReadAerospikeClusters(_ context.Context) map[string]*dto.AerospikeCluster {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	backupConfig := s.config.BackupConfigCopy()
	return dto.ConvertModelMapToDTO(
		backupConfig.AerospikeClusters,
		func(m *model.AerospikeCluster) *dto.AerospikeCluster {
			return dto.NewClusterFromModel(m, backupConfig)
		})
}

func (s *ConfigManagerImpl) ReadAerospikeCluster(_ context.Context, name string) (*dto.AerospikeCluster, error) {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	backupConfig := s.config.BackupConfigCopy()
	cluster, ok := backupConfig.AerospikeClusters[name]
	if !ok {
		return nil, fmt.Errorf("read Aerospike cluster %q: %w", name, model.ErrNotFound)
	}
	return dto.NewClusterFromModel(cluster, backupConfig), nil
}

func (s *ConfigManagerImpl) ReadPolicies(_ context.Context) map[string]*dto.BackupPolicy {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	return dto.ConvertModelMapToDTO(s.config.BackupConfigCopy().BackupPolicies, dto.NewBackupPolicyFromModel)
}

func (s *ConfigManagerImpl) ReadPolicy(_ context.Context, name string) (*dto.BackupPolicy, error) {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	policy, ok := s.config.BackupConfigCopy().BackupPolicies[name]
	if !ok {
		return nil, fmt.Errorf("read backup policy %q: %w", name, model.ErrNotFound)
	}
	return dto.NewBackupPolicyFromModel(policy), nil
}

func (s *ConfigManagerImpl) ReadRoutines(_ context.Context) map[string]*dto.BackupRoutine {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	return dto.ConvertModelMapToDTO(s.config.Routines(), func(m *model.BackupRoutine) *dto.BackupRoutine {
		return dto.NewRoutineFromModel(m, s.config)
	})
}

func (s *ConfigManagerImpl) ReadRoutine(_ context.Context, name string) (*dto.BackupRoutine, error) {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	routine, found := s.config.Routine(name)
	if !found {
		return nil, fmt.Errorf("read backup routine %q: %w", name, model.ErrNotFound)
	}
	return dto.NewRoutineFromModel(routine, s.config), nil
}

func (s *ConfigManagerImpl) ReadAllStorage(_ context.Context) map[string]*dto.Storage {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	backupConfig := s.config.BackupConfigCopy()
	return dto.ConvertStorageMapToDTO(backupConfig.Storage, backupConfig)
}

func (s *ConfigManagerImpl) ReadStorage(_ context.Context, name string) (*dto.Storage, error) {
	s.changeConfigLock.Lock()
	defer s.changeConfigLock.Unlock()
	backupConfig := s.config.BackupConfigCopy()
	storage, ok := backupConfig.Storage[name]
	if !ok {
		return nil, fmt.Errorf("read storage %q: %w", name, model.ErrNotFound)
	}
	return dto.NewStorageFromModel(storage, backupConfig), nil
}

func (s *ConfigManagerImpl) AddAerospikeCluster(
	ctx context.Context, name string, newCluster *dto.AerospikeCluster,
) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.AerospikeClusters[name]; exists {
			return nil, fmt.Errorf("add Aerospike cluster %q: %w", name, model.ErrAlreadyExists)
		}
		config.AerospikeClusters[name] = newCluster
		return nil, nil
	})
}

func (s *ConfigManagerImpl) UpdateAerospikeCluster(
	ctx context.Context, name string, updatedCluster *dto.AerospikeCluster,
) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.AerospikeClusters[name]; !exists {
			return nil, fmt.Errorf("update Aerospike cluster %q: %w", name, model.ErrNotFound)
		}
		config.AerospikeClusters[name] = updatedCluster
		return nil, nil
	}, WithNamespaceValidation)
}

func (s *ConfigManagerImpl) DeleteAerospikeCluster(ctx context.Context, name string) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.AerospikeClusters[name]; !exists {
			return nil, fmt.Errorf("delete Aerospike cluster %q: %w", name, model.ErrNotFound)
		}
		if err := ensureClusterNotInUse(config, name); err != nil {
			return nil, err
		}
		delete(config.AerospikeClusters, name)
		return nil, nil
	})
}

func (s *ConfigManagerImpl) AddPolicy(ctx context.Context, name string, newPolicy *dto.BackupPolicy) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupPolicies[name]; exists {
			return nil, fmt.Errorf("add backup policy %q: %w", name, model.ErrAlreadyExists)
		}
		config.BackupPolicies[name] = newPolicy
		return nil, nil
	})
}

func (s *ConfigManagerImpl) UpdatePolicy(ctx context.Context, name string, updatedPolicy *dto.BackupPolicy) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupPolicies[name]; !exists {
			return nil, fmt.Errorf("update backup policy %q: %w", name, model.ErrNotFound)
		}
		config.BackupPolicies[name] = updatedPolicy
		return nil, nil
	})
}

func (s *ConfigManagerImpl) DeletePolicy(ctx context.Context, name string) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupPolicies[name]; !exists {
			return nil, fmt.Errorf("delete backup policy %q: %w", name, model.ErrNotFound)
		}
		if err := ensurePolicyNotInUse(config, name); err != nil {
			return nil, err
		}
		delete(config.BackupPolicies, name)
		return nil, nil
	})
}

func (s *ConfigManagerImpl) AddRoutine(ctx context.Context, name string, newRoutine *dto.BackupRoutine) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupRoutines[name]; exists {
			return nil, fmt.Errorf("add backup routine %q: %w", name, model.ErrAlreadyExists)
		}
		config.BackupRoutines[name] = newRoutine
		return []string{name}, nil
	}, WithNamespaceValidation)
}

func (s *ConfigManagerImpl) UpdateRoutine(ctx context.Context, name string, updatedRoutine *dto.BackupRoutine) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupRoutines[name]; !exists {
			return nil, fmt.Errorf("update backup routine %q: %w", name, model.ErrNotFound)
		}
		config.BackupRoutines[name] = updatedRoutine
		return []string{name}, nil
	}, WithNamespaceValidation)
}

func (s *ConfigManagerImpl) DeleteRoutine(ctx context.Context, name string) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.BackupRoutines[name]; !exists {
			return nil, fmt.Errorf("delete backup routine %q: %w", name, model.ErrNotFound)
		}
		delete(config.BackupRoutines, name)
		return []string{name}, nil
	})
}

func (s *ConfigManagerImpl) EnableRoutine(ctx context.Context, name string) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		routine, exists := config.BackupRoutines[name]
		if !exists {
			return nil, fmt.Errorf("toggle disable for backup routine %q: %w", name, model.ErrNotFound)
		}
		routine.Disabled = false
		return []string{name}, nil
	})
}

func (s *ConfigManagerImpl) DisableRoutine(ctx context.Context, name string) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		routine, exists := config.BackupRoutines[name]
		if !exists {
			return nil, fmt.Errorf("toggle disable for backup routine %q: %w", name, model.ErrNotFound)
		}
		routine.Disabled = true
		return []string{name}, nil
	})
}

func (s *ConfigManagerImpl) AddStorage(ctx context.Context, name string, newStorage *dto.Storage) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.Storage[name]; exists {
			return nil, fmt.Errorf("add storage %q: %w", name, model.ErrAlreadyExists)
		}
		config.Storage[name] = newStorage
		return nil, nil
	})
}

func (s *ConfigManagerImpl) UpdateStorage(ctx context.Context, name string, updatedStorage *dto.Storage) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.Storage[name]; !exists {
			return nil, fmt.Errorf("update storage %q: %w", name, model.ErrNotFound)
		}
		config.Storage[name] = updatedStorage
		return routinesUsingStorage(config, name), nil
	})
}

func (s *ConfigManagerImpl) DeleteStorage(ctx context.Context, name string) error {
	return s.changeConfigInternal(ctx, func(config *dto.Config) ([]string, error) {
		if _, exists := config.Storage[name]; !exists {
			return nil, fmt.Errorf("delete storage %q: %w", name, model.ErrNotFound)
		}
		if err := ensureStorageNotInUse(config, name); err != nil {
			return nil, err
		}
		delete(config.Storage, name)
		return nil, nil
	})
}

func (s *ConfigManagerImpl) changeConfigInternal(
	ctx context.Context,
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
