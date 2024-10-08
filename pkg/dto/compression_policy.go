package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// Compression modes
const (
	CompressNone = "NONE"
	CompressZSTD = "ZSTD"
)

// CompressionPolicy contains backup compression information.
// @Description CompressionPolicy contains backup compression information.
type CompressionPolicy struct {
	// The compression mode to be used (default is NONE).
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" default:"NONE" enums:"NONE,ZSTD"`
	// The compression level to use (or -1 if unspecified).
	Level int32 `yaml:"level,omitempty" json:"level,omitempty"`
}

// Validate validates the compression policy.
func (p *CompressionPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.Mode != CompressNone && p.Mode != CompressZSTD {
		return fmt.Errorf("invalid compression mode: %s", p.Mode)
	}
	if p.Level == 0 {
		p.Level = -1
	}
	if p.Level < -1 {
		return fmt.Errorf("invalid compression level: %d", p.Level)
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

func (p *CompressionPolicy) fromModel(m *model.CompressionPolicy) {
	p.Mode = m.Mode
	p.Level = m.Level
}
