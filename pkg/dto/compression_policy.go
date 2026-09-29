package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// CompressionMode identifies the compression algorithm used for backup files.
// @Description CompressionMode identifies the compression algorithm used for backup files.
type CompressionMode string

const (
	CompressionModeNone CompressionMode = "NONE"
	CompressionModeZSTD CompressionMode = "ZSTD"
)

var compressionModes = []CompressionMode{CompressionModeNone, CompressionModeZSTD}

// Validate checks that the compression mode is supported.
func (m CompressionMode) Validate() error {
	if _, ok := canonicalEnum(m, compressionModes); ok {
		return nil
	}

	return errValidationInvalidValue("compression mode", m, compressionModes)
}

// ToModel converts the DTO compression mode to the model type.
func (m CompressionMode) ToModel() model.CompressionMode {
	c, _ := canonicalEnum(m, compressionModes)
	return model.CompressionMode(c)
}

// NewCompressionModeFromModel creates a DTO compression mode from the model type.
func NewCompressionModeFromModel(m model.CompressionMode) CompressionMode {
	return CompressionMode(m)
}

// CompressionPolicy contains backup compression information.
// @Description CompressionPolicy contains backup compression information.
type CompressionPolicy struct {
	// The compression mode to be used. Required.
	Mode CompressionMode `yaml:"mode,omitempty" json:"mode,omitempty" validate:"required"`
	// The compression level to use, from -1 to 22.
	// A higher value gives better compression at the cost of speed,
	// but not every step changes the result: neighboring levels may compress the same way.
	// Required for ZSTD; must not be set if the compression mode is NONE.
	Level *int32 `yaml:"level,omitempty" json:"level,omitempty" minimum:"-1" maximum:"22" extensions:"x-nullable"`
}

// Validate validates the compression policy.
func (p *CompressionPolicy) Validate() error {
	if p == nil {
		return nil
	}

	mode, ok := canonicalEnum(p.Mode, compressionModes)
	if mode == "" {
		return errValidationEmptyField("mode")
	}
	if !ok {
		return errValidationInvalidValue("mode", p.Mode, compressionModes)
	}

	if mode == CompressionModeNone {
		if p.Level != nil {
			return errValidationMutuallyExclusive("level", "mode = NONE")
		}

		return nil // no more validation for mode = NONE
	}

	if p.Level == nil {
		return errValidationRequires("mode = ZSTD", "level")
	}
	if *p.Level < -1 || *p.Level > 22 {
		return errValidationInvalidValue("level", *p.Level, "-1 to 22")
	}

	return nil
}

func (p *CompressionPolicy) ToModel() *model.CompressionPolicy {
	if p == nil {
		return nil
	}

	return &model.CompressionPolicy{
		Mode:  p.Mode.ToModel(),
		Level: ptr.ValueOrZero(p.Level),
	}
}

func newCompressionPolicyFromModel(m *model.CompressionPolicy) *CompressionPolicy {
	if m == nil {
		return nil
	}

	c := &CompressionPolicy{}
	c.fromModel(m)

	return c
}

func (p *CompressionPolicy) fromModel(m *model.CompressionPolicy) {
	p.Mode = NewCompressionModeFromModel(m.Mode)
	if m.Mode != model.CompressionModeNone {
		p.Level = ptr.Of(m.Level)
	}
}

// RestoreCompressionPolicy contains restore compression information.
// @Description RestoreCompressionPolicy contains restore compression information.
type RestoreCompressionPolicy struct {
	// The compression mode to be used. Required.
	Mode CompressionMode `yaml:"mode,omitempty" json:"mode,omitempty" validate:"required"`
}

// Validate validates the restore compression policy.
func (p *RestoreCompressionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	mode, ok := canonicalEnum(p.Mode, compressionModes)
	if mode == "" {
		return errValidationEmptyField("mode")
	}
	if !ok {
		return errValidationInvalidValue("mode", p.Mode, compressionModes)
	}

	return nil
}

func (p *RestoreCompressionPolicy) ToModel() *model.CompressionPolicy {
	if p == nil {
		return nil
	}

	return &model.CompressionPolicy{
		Mode: p.Mode.ToModel(),
	}
}
