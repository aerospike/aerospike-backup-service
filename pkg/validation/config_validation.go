package validation

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

// ValidateStaticFieldChanges checks if new config changes are allowed as per dynamic consideration.
func ValidateStaticFieldChanges(oldConf, newConf *dto.Config) error {
	// currently, only ServiceConfig can not be changed dynamically.
	return oldConf.ServiceConfig.Compare(newConf.ServiceConfig)
}

func ValidateConfiguration(conf *dto.Config) error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}

	_, err := conf.ToModel(dto.ValidationSkipTLSFiles)
	return err
}

func ValidateRestoreRequest(request *dto.RestoreRequest, conf *dto.Config) error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}
	if request == nil {
		return fmt.Errorf("restore request is nil")
	}

	model, err := conf.ToModel(dto.ValidationSkipTLSFiles)
	if err != nil {
		return fmt.Errorf("config invalid: %w", err)
	}

	err = request.Validate(dto.ValidationSkipTLSFiles)
	if err != nil {
		return err
	}

	_, err = request.ToModel(model)

	return err
}

func ValidateRestoreTimestampRequest(request *dto.RestoreTimestampRequest, conf *dto.Config) error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}
	if request == nil {
		return fmt.Errorf("restore request is nil")
	}

	model, err := conf.ToModel(dto.ValidationSkipTLSFiles)
	if err != nil {
		return fmt.Errorf("config invalid: %w", err)
	}

	err = request.Validate(dto.ValidationSkipTLSFiles)
	if err != nil {
		return err
	}

	_, err = request.ToModel(model)

	return err
}
