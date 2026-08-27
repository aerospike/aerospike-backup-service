package model

// Port represents a network port.
type Port int

func NewPort(port int) *Port {
	p := Port(port)
	return &p
}
