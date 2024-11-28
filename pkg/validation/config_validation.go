package validation

import (
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
)

// ConfigurationApplicable checks if new config changes are allowed as per dynamic consideration.
func ConfigurationApplicable(oldConf, newConf *dto.Config) error {
	if err := ValidateConfiguration(newConf); err != nil {
		return err
	}

	// currently, only ServiceConfig can not be changed dynamically.
	return oldConf.ServiceConfig.Compare(newConf.ServiceConfig)
}

func ValidateConfiguration(conf *dto.Config) error {
	clientManager := service.NewClientManager(&service.DefaultClientFactory{}, time.Second)
	namespaceValidator := service.NewNamespaceValidator(clientManager)
	_, err := conf.ToModel(namespaceValidator)
	return err
}

func ValidateRequest(request dto.RestoreRequest, conf *dto.Config) error {
	model, err := conf.ToModel(nil)
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

func ValidateRestoreTimestamp(request dto.RestoreTimestampRequest, conf *dto.Config) error {
	model, err := conf.ToModel(nil)
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
