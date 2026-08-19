package model

import "time"

// HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	// The address to listen on.
	Address string
	// The port to listen on.
	Port *Port
	// HTTP rate limiter configuration.
	Rate *RateLimiterConfig
	// ContextPath customizes path for the API endpoints.
	ContextPath string
	// Timeout for reading HTTP request headers (http.Server.ReadHeaderTimeout).
	Timeout *time.Duration
	// ReadTimeout is the maximum duration for reading the entire request, including the body.
	ReadTimeout *time.Duration
	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout *time.Duration
	// IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled.
	IdleTimeout *time.Duration
}

// GetAddressOrDefault returns the value of the Address property.
// If the property is not set, it returns the default value.
func (s *HTTPServerConfig) GetAddressOrDefault() string {
	if s.Address != "" {
		return s.Address
	}
	return defaultConfig.http.Address
}

// GetPortOrDefault returns the value of the Port property.
// If the property is not set, it returns the default value.
func (s *HTTPServerConfig) GetPortOrDefault() Port {
	if s.Port != nil {
		return *s.Port
	}
	return *defaultConfig.http.Port
}

// GetTimeoutOrDefault returns the value of the Timeout property.
// If the property is not set, it returns the default value = 5s.
func (s *HTTPServerConfig) GetTimeoutOrDefault() time.Duration {
	if s.Timeout != nil {
		return *s.Timeout
	}
	return *defaultConfig.http.Timeout
}

// GetReadTimeoutOrDefault returns the value of the ReadTimeout property.
// If the property is not set, it returns the default value = 30s.
func (s *HTTPServerConfig) GetReadTimeoutOrDefault() time.Duration {
	if s.ReadTimeout != nil {
		return *s.ReadTimeout
	}
	return *defaultConfig.http.ReadTimeout
}

// GetWriteTimeoutOrDefault returns the value of the WriteTimeout property.
// If the property is not set, it returns the default value = 60s.
func (s *HTTPServerConfig) GetWriteTimeoutOrDefault() time.Duration {
	if s.WriteTimeout != nil {
		return *s.WriteTimeout
	}
	return *defaultConfig.http.WriteTimeout
}

// GetIdleTimeoutOrDefault returns the value of the IdleTimeout property.
// If the property is not set, it returns the default value = 120s.
func (s *HTTPServerConfig) GetIdleTimeoutOrDefault() time.Duration {
	if s.IdleTimeout != nil {
		return *s.IdleTimeout
	}
	return *defaultConfig.http.IdleTimeout
}

// GetRateOrDefault returns the value of the Rate property.
// If the property is not set, it returns the default value.
func (s *HTTPServerConfig) GetRateOrDefault() *RateLimiterConfig {
	if s.Rate != nil {
		return s.Rate
	}
	return defaultConfig.http.Rate
}

// GetContextPathOrDefault returns the value of the ContextPath property.
// If the property is not set, it returns the default value.
func (s *HTTPServerConfig) GetContextPathOrDefault() string {
	if s.ContextPath != "" {
		return s.ContextPath
	}
	return defaultConfig.http.ContextPath
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	// Rate limiter tokens per second threshold.
	Tps *int
	// Rate limiter token bucket size (bursts threshold).
	Size *int
	// The list of ips to whitelist in rate limiting.
	WhiteList []string
}

// GetTpsOrDefault returns the value of the Tps property.
// If the property is not set, it returns the default value.
func (r *RateLimiterConfig) GetTpsOrDefault() int {
	if r.Tps != nil {
		return *r.Tps
	}
	return *defaultConfig.http.Rate.Tps
}

// GetSizeOrDefault returns the value of the Size property.
// If the property is not set, it returns the default value.
func (r *RateLimiterConfig) GetSizeOrDefault() int {
	if r.Size != nil {
		return *r.Size
	}
	return *defaultConfig.http.Rate.Size
}

// GetWhiteListOrDefault returns the value of the WhiteList property.
// If the property is not set, it returns the default value.
func (r *RateLimiterConfig) GetWhiteListOrDefault() []string {
	if r.WhiteList != nil {
		return r.WhiteList
	}
	return defaultConfig.http.Rate.WhiteList
}
