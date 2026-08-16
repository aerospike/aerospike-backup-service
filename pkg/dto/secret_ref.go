package dto

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const secretRefPrefix = "secrets:"

var (
	errSecretRefNoAgent = errors.New(
		"secret reference requires secret agent configuration (secret-agent or secret-agent-name)",
	)
	errSecretRefMalformed = errors.New("invalid secret reference format, must be secrets:<resource>:<secret>")
)

func validateSecretRef(value string, agent *model.SecretAgent) error {
	if value == "" || !strings.HasPrefix(value, secretRefPrefix) {
		return nil
	}

	if agent == nil {
		return fmt.Errorf("%q: %w", value, errSecretRefNoAgent)
	}

	if len(strings.Split(value, ":")) != 3 {
		return fmt.Errorf("%q: %w", value, errSecretRefMalformed)
	}

	return nil
}
