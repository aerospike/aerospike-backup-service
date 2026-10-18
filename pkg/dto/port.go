package dto

import (
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
