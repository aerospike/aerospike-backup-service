package dto

import (
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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
	if m == "" || slices.Contains(compressionModes, m) {
		return nil
	}

	return errValidationInvalidValue("compression mode", m, compressionModes)
}

// ToModel converts the DTO compression mode to the model type.
func (m CompressionMode) ToModel() model.CompressionMode {
	return model.CompressionMode(m)
}

// NewCompressionModeFromModel creates a DTO compression mode from the model type.
func NewCompressionModeFromModel(m model.CompressionMode) CompressionMode {
	return CompressionMode(m)
}

// CompressionPolicy contains backup compression information.
// @Description CompressionPolicy contains backup compression information.
type CompressionPolicy struct {
	// The compression mode to be used (default is NONE).
	Mode CompressionMode `yaml:"mode,omitempty" json:"mode,omitempty" default:"NONE"`
	// The compression level to use.
	// Algorithm-specific; for zstd: from -1 (fastest) to 22 (best compression).
	// This field is ignored if the compression mode is NONE.
	Level int32 `yaml:"level,omitempty" json:"level,omitempty" default:"0" minimum:"-1" maximum:"22"`
}

// Validate validates the compression policy.
func (p *CompressionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if err := p.Mode.Validate(); err != nil {
		return err
	}
	if p.Level < -1 || p.Level > 22 {
		return errValidationInvalidValue("compression level", p.Level, "-1 to 22")
	}

	return nil
}

func (p *CompressionPolicy) ToModel() *model.CompressionPolicy {
	if p == nil {
		return nil
	}

	return &model.CompressionPolicy{
		Mode:  p.Mode.ToModel(),
		Level: p.Level,
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
	p.Level = m.Level
}

// RestoreCompressionPolicy contains restore compression information.
// @Description RestoreCompressionPolicy contains restore compression information.
type RestoreCompressionPolicy struct {
	// The compression mode to be used (default is NONE).
	Mode CompressionMode `yaml:"mode,omitempty" json:"mode,omitempty" default:"NONE"`
}

// Validate validates the restore compression policy.
func (p *RestoreCompressionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	return p.Mode.Validate()
}

func (p *RestoreCompressionPolicy) ToModel() *model.CompressionPolicy {
	if p == nil {
		return nil
	}

	return &model.CompressionPolicy{
		Mode: p.Mode.ToModel(),
	}
}
