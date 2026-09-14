package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// validateSecret checks a value that may be a Secret Agent reference and reports the result
// against the field it came from. redact.Secret is shared with pkg/model, so this cannot be
// a method on it; it belongs here with the rest of this package's field validators.
//
// Neither message carries a literal credential. A well-formed reference is not sensitive,
// which is exactly what DisplayString returns unchanged. A malformed one is reported without
// its value, because a literal password that happens to start with "secrets:" lands there.
func validateSecret(field string, secret redact.Secret, withAgent bool) error {
	switch {
	case secret == "":
		return nil

	case secret.IsMalformedRef():
		return errValidationSecret(field, errors.New("must be in the form secrets:<resource>:<key>"))

	case secret.IsRef() && !withAgent:
		return errValidationSecret(field, fmt.Errorf(
			"%q requires secret agent configuration (secret-agent or secret-agent-name)", secret.DisplayString()))

	default:
		return nil
	}
}
