package validation

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/aerospike"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
)

var nsValidator = &aerospike.NoopNamespaceValidator{}

// ValidateStaticFieldChanges checks if new config changes are allowed as per dynamic consideration.
func ValidateStaticFieldChanges(oldConf, newConf *dto.Config) error {
	// currently, only ServiceConfig can not be changed dynamically.
	return oldConf.ServiceConfig.Compare(newConf.ServiceConfig)
}

func ValidateConfiguration(conf *dto.Config) error {
	_, err := conf.ToModel(nsValidator)
	return err
}

func ValidateRestoreRequest(request dto.RestoreRequest, conf *dto.Config) error {
	model, err := conf.ToModel(nsValidator)
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

func ValidateRestoreTimestampRequest(request dto.RestoreTimestampRequest, conf *dto.Config) error {
	model, err := conf.ToModel(nsValidator)
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
