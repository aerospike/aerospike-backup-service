package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// validateSecret checks a value that may be a Secret Agent reference and reports the result
// against the field it came from: validateSecretForm, then validateSecretAgent. redact.Secret is
// shared with pkg/model, so these cannot be methods on it; they belong here with the rest of this
// package's field validators.
//
// Neither message carries a literal credential. A well-formed reference is not sensitive,
// which is exactly what DisplayString returns unchanged. A malformed one is reported without
// its value, because a literal password that happens to start with "secrets:" lands there.
func validateSecret(field string, secret redact.Secret, hasAgent bool) error {
	if err := validateSecretForm(field, secret); err != nil {
		return err
	}

	return validateSecretAgent(field, secret, hasAgent)
}

// validateSecretForm checks that a value that looks like a reference is a well-formed one.
// It is the whole check for a DTO that does not own the Secret Agent its secrets are resolved with.
func validateSecretForm(field string, secret redact.Secret) error {
	if secret.IsMalformedRef() {
		return errValidationSecret(field, errors.New("must be in the form secrets:<resource>:<key>"))
	}

	return nil
}

// validateSecretAgent checks that a reference has a Secret Agent to be resolved with.
func validateSecretAgent(field string, secret redact.Secret, hasAgent bool) error {
	if secret.IsRef() && !hasAgent {
		return errValidationSecret(field, fmt.Errorf("%q requires a secret agent", secret.DisplayString()))
	}

	return nil
}
