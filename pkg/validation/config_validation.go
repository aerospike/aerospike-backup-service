package validation

import (
	"errors"
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
		return errors.New("config is nil")
	}

	if err := conf.Validate(); err != nil {
		return err
	}

	_, err := conf.ToModel()
	return err
}

func ValidateRestoreRequest(request *dto.RestoreRequest, conf *dto.Config) error {
	if conf == nil {
		return errors.New("config is nil")
	}
	if request == nil {
		return errors.New("restore request is nil")
	}

	if err := conf.Validate(); err != nil {
		return fmt.Errorf("config invalid: %w", err)
	}

	model, err := conf.ToModel()
	if err != nil {
		return fmt.Errorf("config invalid: %w", err)
	}

	err = request.Validate()
	if err != nil {
		return err
	}

	_, err = request.ToModel(model)

	return err
}

func ValidateRestoreTimestampRequest(request *dto.RestoreTimestampRequest, conf *dto.Config) error {
	if conf == nil {
		return errors.New("config is nil")
	}
	if request == nil {
		return errors.New("restore request is nil")
	}

	if err := conf.Validate(); err != nil {
		return fmt.Errorf("config invalid: %w", err)
	}

	model, err := conf.ToModel()
	if err != nil {
		return fmt.Errorf("config invalid: %w", err)
	}

	err = request.Validate()
	if err != nil {
		return err
	}

	_, err = request.ToModel(model)

	return err
}
