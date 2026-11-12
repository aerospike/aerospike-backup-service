package dto

import (
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Why a set of options is rejected. Each reason completes the sentence
// "<subject> sets <options>, which <reason>".
const (
	// unsupportedInServerMode rejects options of the scan backup mode that the server has no
	// equivalent for. The server backup mode would otherwise ignore them.
	unsupportedInServerMode = `backup-mode "server" does not support`
	// onlyInServerMode rejects options of the server backup mode in the scan backup mode.
	onlyInServerMode = `only backup-mode "server" supports`
	// unsupportedInStrictMode rejects options that apply only to a fuzzy server restore.
	unsupportedInStrictMode = `mode "strict" does not support`
)

// option is an option of a routine or a policy, and whether it is set.
type option struct {
	name string
	set  bool
}

// rejectOptions fails when subject sets any of options, and names all of the set ones. reason
// says why they are not allowed.
func rejectOptions(subject, reason string, options ...option) error {
	var set []string
	for _, o := range options {
		if o.set {
			set = append(set, o.name)
		}
	}
	if len(set) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s sets %s, which %s", errInvalidValue, subject, strings.Join(set, ", "), reason)
}

func encrypts(policy *model.EncryptionPolicy) bool {
	return policy != nil && policy.Mode != model.EncryptionModeNone
}

func compresses(policy *model.CompressionPolicy) bool {
	return policy != nil && policy.Mode != model.CompressionModeNone
}
