package model

// Port represents a network port.
type Port int

func NewPort(port int) *Port {
	p := Port(port)
	return &p
}

// PortRange represents a range of network ports.
type PortRange struct {
	Start Port
	End   Port
}
