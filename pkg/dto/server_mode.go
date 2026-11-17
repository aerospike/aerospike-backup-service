package dto

import (
	"errors"
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

// infoSeparators are the characters an Aerospike info command cannot carry in a value. Server-side
// backup sends routine, policy and storage values in info commands, where ";" separates
// parameters, "|" separates the values the server keeps with a job and a line break ends the
// command.
const infoSeparators = ";|\r\n"

// infoListSeparator separates the entries of a list in an info command, so an entry that holds it
// would reach the server as two.
const infoListSeparator = ","

// validateInfoValue fails when an info command cannot carry the value of field.
func validateInfoValue(field, value string) error {
	if strings.ContainsAny(value, infoSeparators) {
		return errValidationInvalidName(field, value, errors.New(`must not contain ";", "|" or a line break`))
	}

	return nil
}

// validateS3InfoValues fails when an info command cannot carry a part of an S3 location.
func validateS3InfoValues(path, bucket, region, profile, endpoint string) error {
	return errors.Join(
		validateInfoValue("path", path),
		validateInfoValue("bucket", bucket),
		validateInfoValue("s3-region", region),
		validateInfoValue("s3-profile", profile),
		validateInfoValue("s3-endpoint-override", endpoint),
	)
}

// validateInfoList fails on the first entry of the list in field that an info command cannot
// carry as one entry.
func validateInfoList(field string, entries []string) error {
	for _, entry := range entries {
		if strings.ContainsAny(entry, infoSeparators+infoListSeparator) {
			return errValidationInvalidName(field+" entry", entry,
				errors.New(`must not contain ",", ";", "|" or a line break`))
		}
	}

	return nil
}
