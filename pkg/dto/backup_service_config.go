package dto

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// BackupServiceConfig represents the backup service configuration properties.
// @Description BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig `yaml:"http,omitempty" json:"http,omitempty"`
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig `yaml:"logger,omitempty" json:"logger,omitempty"`
	// Encoding for backup date in human-readable format
	DateEncoding *string `yaml:"date-encoding,omitempty" json:"date-encoding,omitempty" enums:"ISO,US,EU" extensions:"x-nullable"` //nolint:lll
}

func (b *BackupServiceConfig) Validate() error {
	if err := b.HTTPServer.Validate(); err != nil {
		return fmt.Errorf("http validation error: %w", err)
	}
	if err := b.Logger.Validate(); err != nil {
		return fmt.Errorf("logger validation error: %w", err)
	}
	if b.DateEncoding != nil {
		if _, ok := model.DateEncodingPresets[model.DateFormat(*b.DateEncoding)]; !ok {
			allowed := slices.Collect(maps.Keys(model.DateEncodingPresets))
			return errValidationInvalidValue("date-encoding", *b.DateEncoding, allowed)
		}
	}

	return nil
}

func (b *BackupServiceConfig) ToModel() *model.BackupServiceConfig {
	var dateEncoding *model.DateFormat
	if b.DateEncoding != nil {
		val := model.DateFormat(*b.DateEncoding)
		dateEncoding = &val
	}
	return &model.BackupServiceConfig{
		HTTPServer: b.HTTPServer.ToModel(),
		Logger:     b.Logger.ToModel(),
		DateFormat: dateEncoding,
	}
}

func (b *BackupServiceConfig) fromModel(m *model.BackupServiceConfig) {
	if m == nil {
		return
	}

	if m.HTTPServer != nil {
		b.HTTPServer = &HTTPServerConfig{}
		b.HTTPServer.fromModel(m.HTTPServer)
	}

	if m.Logger != nil {
		b.Logger = &LoggerConfig{}
		b.Logger.fromModel(m.Logger)
	}

	if m.DateFormat != nil {
		val := string(*m.DateFormat)
		b.DateEncoding = &val
	}
}

// Compare BackupServiceConfig with another and return detailed errors.
func (b *BackupServiceConfig) Compare(other BackupServiceConfig) error {
	var err error

	if e := b.HTTPServer.Compare(other.HTTPServer); e != nil {
		err = errors.Join(err, fmt.Errorf("HTTPServer changes: %w", e))
	}

	if e := b.Logger.Compare(other.Logger); e != nil {
		err = errors.Join(err, fmt.Errorf("logger changes: %w", e))
	}

	if b.DateEncoding == nil && other.DateEncoding != nil {
		err = errors.Join(err, errors.New("date encoding added"))
	}
	if b.DateEncoding != nil && other.DateEncoding == nil {
		err = errors.Join(err, errors.New("date encoding removed"))
	}
	if ptr.ValueOrZero(b.DateEncoding) != ptr.ValueOrZero(other.DateEncoding) {
		err = errors.Join(err, errors.New("date encoding changed"))
	}

	return err
}
