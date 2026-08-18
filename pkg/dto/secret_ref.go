package dto

import "github.com/aerospike/aerospike-backup-service/v3/pkg/model"

func validateSecretRef(value secret, agent *model.SecretAgent) error {
	if value == "" || !value.HasRefPrefix() {
		return nil
	}

	if !value.IsRef() {
		return errValidationSecretRefMalformed(value)
	}

	if agent == nil {
		return errValidationSecretRefNoAgent(value)
	}

	return nil
}
