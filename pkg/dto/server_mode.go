package dto

import (
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// serverOption is an option of the scan backup mode, and whether it is set.
type serverOption struct {
	name string
	set  bool
}

// rejectServerUnsupported fails when subject sets any of options: the server has no equivalent
// for them, so the server backup mode would ignore them.
func rejectServerUnsupported(subject string, options ...serverOption) error {
	var set []string
	for _, option := range options {
		if option.set {
			set = append(set, option.name)
		}
	}
	if len(set) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s sets %s, which backup-mode %q does not support",
		errInvalidValue, subject, strings.Join(set, ", "), model.BackupModeServer)
}

func encrypts(policy *model.EncryptionPolicy) bool {
	return policy != nil && policy.Mode != model.EncryptionModeNone
}

func compresses(policy *model.CompressionPolicy) bool {
	return policy != nil && policy.Mode != model.CompressionModeNone
}
