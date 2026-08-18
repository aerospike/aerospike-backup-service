package dto

import "github.com/aerospike/aerospike-backup-service/v3/pkg/model"

func validateSecretRef(value secret, agent *model.SecretAgent) error {
	if !value.IsRef() {
		return nil
	}

	if agent == nil {
		return errValidationSecretRefNoAgent(value)
	}

	return nil
}
