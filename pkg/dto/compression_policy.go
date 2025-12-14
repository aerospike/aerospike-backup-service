package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Compression modes.
const (
	CompressNone = "NONE"
	CompressZSTD = "ZSTD"
)

// CompressionPolicy contains backup compression information.
// @Description CompressionPolicy contains backup compression information.
type CompressionPolicy struct {
	// The compression mode to be used (default is NONE).
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" default:"NONE" enums:"NONE,ZSTD"`
	// The compression level to use.
	// Algorithm-specific; for zstd: from -1 (fastest) to 22 (best compression).
	// This field is ignored if the compression mode is NONE.
	// This field is ignored during restoration.
	Level int32 `yaml:"level,omitempty" json:"level,omitempty" default:"0" minimum:"-1" maximum:"22"`
}

// Validate validates the compression policy.
func (p *CompressionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.Mode != CompressNone && p.Mode != CompressZSTD {
		return errValidationInvalidValue("compression mode", p.Mode, []string{CompressNone, CompressZSTD})
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
		Mode:  p.Mode,
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
	p.Mode = m.Mode
	p.Level = m.Level
}

// RestoreCompressionPolicy contains restore compression information.
// @Description RestoreCompressionPolicy contains restore compression information.
type RestoreCompressionPolicy struct {
	// The compression mode to be used (default is NONE).
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" default:"NONE" enums:"NONE,ZSTD"`
}

// Validate validates the restore compression policy.
func (p *RestoreCompressionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.Mode != CompressNone && p.Mode != CompressZSTD {
		return errValidationInvalidValue("compression mode", p.Mode, []string{CompressNone, CompressZSTD})
	}
	return nil
}

func (p *RestoreCompressionPolicy) ToModel() *model.CompressionPolicy {
	if p == nil {
		return nil
	}

	return &model.CompressionPolicy{
		Mode: p.Mode,
	}
}
