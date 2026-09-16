package model

// ServerConfigHTTP represents the service's HTTP server configuration.
type ServerConfigHTTP struct {
	ListenerConfig
	// The port to listen on.
	Port *Port
}

// GetPortOrDefault returns the value of the Port property.
// If the property is not set, it returns the default value.
func (s *ServerConfigHTTP) GetPortOrDefault() Port {
	if s == nil || s.Port == nil {
		return *defaultConfig.http.Port
	}
	return *s.Port
}
