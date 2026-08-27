package model

import "time"

// ListenerConfig represents the listen settings shared by the HTTP and HTTPS servers.
type ListenerConfig struct {
	// Disabled controls whether the listener is disabled.
	Disabled bool
	// The address to listen on.
	Address string
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

// GetDisabledOrDefault returns whether the listener is disabled.
func (l *ListenerConfig) GetDisabledOrDefault() bool {
	if l == nil {
		return defaultListener.Disabled
	}
	return l.Disabled
}

// GetAddressOrDefault returns the value of the Address property.
// If the property is not set, it returns the default value.
func (l *ListenerConfig) GetAddressOrDefault() string {
	if l == nil || l.Address == "" {
		return defaultListener.Address
	}
	return l.Address
}

// GetRateOrDefault returns the value of the Rate property.
// If the property is not set, it returns the default value.
func (l *ListenerConfig) GetRateOrDefault() *RateLimiterConfig {
	if l == nil || l.Rate == nil {
		return defaultListener.Rate
	}
	return l.Rate
}

// GetContextPathOrDefault returns the value of the ContextPath property.
// If the property is not set, it returns the default value.
func (l *ListenerConfig) GetContextPathOrDefault() string {
	if l == nil || l.ContextPath == "" {
		return defaultListener.ContextPath
	}
	return l.ContextPath
}

// GetTimeoutOrDefault returns the value of the Timeout property.
// If the property is not set, it returns the default value = 5s.
func (l *ListenerConfig) GetTimeoutOrDefault() time.Duration {
	if l == nil || l.Timeout == nil {
		return *defaultListener.Timeout
	}
	return *l.Timeout
}

// GetReadTimeoutOrDefault returns the value of the ReadTimeout property.
// If the property is not set, it returns the default value = 30s.
func (l *ListenerConfig) GetReadTimeoutOrDefault() time.Duration {
	if l == nil || l.ReadTimeout == nil {
		return *defaultListener.ReadTimeout
	}
	return *l.ReadTimeout
}

// GetWriteTimeoutOrDefault returns the value of the WriteTimeout property.
// If the property is not set, it returns the default value = 60s.
func (l *ListenerConfig) GetWriteTimeoutOrDefault() time.Duration {
	if l == nil || l.WriteTimeout == nil {
		return *defaultListener.WriteTimeout
	}
	return *l.WriteTimeout
}

// GetIdleTimeoutOrDefault returns the value of the IdleTimeout property.
// If the property is not set, it returns the default value = 120s.
func (l *ListenerConfig) GetIdleTimeoutOrDefault() time.Duration {
	if l == nil || l.IdleTimeout == nil {
		return *defaultListener.IdleTimeout
	}
	return *l.IdleTimeout
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
	return *defaultListener.Rate.Tps
}

// GetSizeOrDefault returns the value of the Size property.
// If the property is not set, it returns the default value.
func (r *RateLimiterConfig) GetSizeOrDefault() int {
	if r.Size != nil {
		return *r.Size
	}
	return *defaultListener.Rate.Size
}

// GetWhiteListOrDefault returns the value of the WhiteList property.
// If the property is not set, it returns the default value.
func (r *RateLimiterConfig) GetWhiteListOrDefault() []string {
	if r.WhiteList != nil {
		return r.WhiteList
	}
	return defaultListener.Rate.WhiteList
}
