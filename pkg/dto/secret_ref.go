package dto

import (
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const secretRefPrefix = "secrets:"

func validateSecretRef(value string, agent *model.SecretAgent) error {
	if value == "" || !strings.HasPrefix(value, secretRefPrefix) {
		return nil
	}

	if agent == nil {
		return errValidationSecretRefNoAgent(value)
	}

	if len(strings.Split(value, ":")) != 3 {
		return errValidationSecretRefMalformed(value)
	}

	return nil
}
