package dto

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Port represents a network port.
// @Description Port is a single network port.
type Port int

// Validate checks if the port number is within the valid range (1-65535).
func (p *Port) Validate() error {
	if p == nil {
		return nil
	}
	if *p < 1 || *p > 65535 {
		return fmt.Errorf("port number %d invalid: must be between 1 and 65535", *p)
	}
	return nil
}

func (p *Port) ToModel() *model.Port {
	if p == nil {
		return nil
	}
	port := model.Port(*p)
	return &port
}

// NewPortFromModel creates a DTO Port from a model Port.
func NewPortFromModel(m *model.Port) *Port {
	if m == nil {
		return nil
	}

	port := Port(*m)
	return &port
}

// PortRange represents a range of network ports.
// @Description PortRange is a range of ports (inclusive).
type PortRange struct {
	// @Description Start port of the range (inclusive).
	Start Port `json:"start" yaml:"start"`

	// @Description End port of the range (inclusive).
	End Port `json:"end" yaml:"end"`
}

func (p *PortRange) Validate() error {
	if p == nil {
		return nil
	}
	if err := p.Start.Validate(); err != nil {
		return fmt.Errorf("invalid start port: %w", err)
	}
	if err := p.End.Validate(); err != nil {
		return fmt.Errorf("invalid end port: %w", err)
	}
	if p.Start > p.End {
		return errors.New("start port must be less than or equal to end port")
	}

	return nil
}

func (p *PortRange) ToModel() *model.PortRange {
	if p == nil {
		return nil
	}

	return &model.PortRange{
		Start: model.Port(p.Start),
		End:   model.Port(p.End),
	}
}

func newPortRangeFromModel(m *model.PortRange) *PortRange {
	if m == nil {
		return nil
	}

	return &PortRange{
		Start: Port(m.Start),
		End:   Port(m.End),
	}
}
