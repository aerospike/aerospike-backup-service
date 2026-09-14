package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// secretRef validates a redact.Secret as a Secret Agent reference. redact.Secret itself is
// shared with pkg/model and stays focused purely on redaction (String, GoString, LogValue,
// Reveal, Hash); this wrapper keeps validation — an input-format concern — in the DTO layer,
// alongside every other field validator in this package.
type secretRef struct {
	redact.Secret
}

// Validate checks that the value, if it looks like a Secret Agent reference, is well-formed,
// and that a Secret Agent is configured to resolve it. It returns only the reason: callers
// wrap the result with errValidationSecret, which supplies the field name.
func (s secretRef) Validate(withAgent bool) error {
	if s.Secret == "" {
		return nil
	}

	if s.IsMalformedRef() {
		return fmt.Errorf("%q must be in the form secrets:<resource>:<key>", string(s.Secret))
	}

	if s.IsRef() && !withAgent {
		return fmt.Errorf("%q requires secret agent configuration (secret-agent or secret-agent-name)", string(s.Secret))
	}

	return nil
}
