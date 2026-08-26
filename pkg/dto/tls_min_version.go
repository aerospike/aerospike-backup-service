package dto

import "github.com/aerospike/aerospike-backup-service/v3/pkg/model"

// TLSMinVersion is the minimum accepted TLS protocol version.
// @Description TLSMinVersion is the minimum accepted TLS protocol version.
type TLSMinVersion string

const (
	// TLSMinVersion12 is TLS 1.2.
	TLSMinVersion12 TLSMinVersion = "1.2"
	// TLSMinVersion13 is TLS 1.3.
	TLSMinVersion13 TLSMinVersion = "1.3"
)

// Validate checks that the minimum TLS version is supported.
func (v TLSMinVersion) Validate() error {
	if _, err := v.ToModel(); err != nil {
		return errValidationInvalidValue("min-version", v, []string{string(TLSMinVersion12), string(TLSMinVersion13)})
	}

	return nil
}

// ToModel converts the DTO minimum TLS version to the model type.
func (v TLSMinVersion) ToModel() (model.TLSMinVersion, error) {
	switch v {
	case "", TLSMinVersion12:
		return model.TLSMinVersion12, nil
	case TLSMinVersion13:
		return model.TLSMinVersion13, nil
	default:
		return "", errValidationInvalidValue("min-version", v, []string{string(TLSMinVersion12), string(TLSMinVersion13)})
	}
}

// NewTLSMinVersionFromModel creates a DTO minimum TLS version from the model type.
func NewTLSMinVersionFromModel(m model.TLSMinVersion) TLSMinVersion {
	return TLSMinVersion(m)
}
