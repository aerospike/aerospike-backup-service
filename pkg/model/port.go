package model

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// Port represents a network port
type Port int

func NewPort(port int) *Port {
	p := Port(port)
	return &p
}

// PortRange represents a range of network ports
type PortRange struct {
	Start Port
	End   Port
}

func (p *Port) IntValue() *int {
	if p == nil {
		return nil
	}

	return util.Ptr(int(*p))
}
