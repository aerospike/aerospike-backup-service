package model

// HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	ListenerConfig
	// The port to listen on.
	Port *Port
}

// GetPortOrDefault returns the value of the Port property.
// If the property is not set, it returns the default value.
func (s *HTTPServerConfig) GetPortOrDefault() Port {
	if s == nil || s.Port == nil {
		return *defaultConfig.http.Port
	}
	return *s.Port
}
