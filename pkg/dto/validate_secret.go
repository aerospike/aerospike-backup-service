package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// validateSecretForm checks that a value that looks like a Secret Agent reference is a
// well-formed one. It is the whole check Validate makes on a secret; whether a reference has an
// agent is checked by validateSecretAgent in the ToModel that resolves that agent. redact.Secret
// is shared with pkg/model, so these cannot be methods on it; they belong here with the rest of
// this package's field validators.
//
// Neither message carries a literal credential. A well-formed reference is not sensitive,
// which is exactly what DisplayString returns unchanged. A malformed one is reported without
// its value, because a literal password that happens to start with "secrets:" lands here.
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

// validateEncryptionKeyAgent checks that a policy's key-secret reference has the agent it is
// resolved with: a policy owns none, so the ToModel of the routine or request that runs it calls this.
func validateEncryptionKeyAgent(policy *model.EncryptionPolicy, agent *model.SecretAgent) error {
	if policy == nil {
		return nil
	}

	return validateSecretAgent("key-secret", policy.KeySecret, agent != nil)
}
