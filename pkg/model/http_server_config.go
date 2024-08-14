package model

import (
	"fmt"
	"strings"
)

// HTTPServerConfig represents the service's HTTP server configuration.
// @Description HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	// The address to listen on.
	Address *string `yaml:"address,omitempty" json:"address,omitempty" default:"0.0.0.0" example:"0.0.0.0"`
	// The port to listen on.
	Port *int `yaml:"port,omitempty" json:"port,omitempty" default:"8080" example:"8080"`
	// HTTP rate limiter configuration.
	Rate *RateLimiterConfig `yaml:"rate,omitempty" json:"rate,omitempty"`
	// ContextPath customizes path for the API endpoints.
	ContextPath *string `yaml:"context-path,omitempty" json:"context-path,omitempty" default:"/"`
	// Timeout for http server operations in milliseconds.
	Timeout *int `yaml:"timeout,omitempty" json:"timeout,omitempty" default:"5000"`
}

// GetAddressOrDefault returns the value of the Address property.
// If the property is not set, it returns the default value.
func (s *HTTPServerConfig) GetAddressOrDefault() string {
	if s.Address != nil {
		return *s.Address
	}
	return *defaultConfig.http.Address
}

// GetPortOrDefault returns the value of the Port property.
// If the property is not set, it returns the default value.
func (s *HTTPServerConfig) GetPortOrDefault() int {
	if s.Port != nil {
		return *s.Port
	}
	return *defaultConfig.http.Port
}

// GetTimeout returns the value of the Timeout property.
// If the property is not set, it returns the default value = 5s.
func (s *HTTPServerConfig) GetTimeout() int {
	if s.Timeout != nil {
		return *s.Timeout
	}
	return *defaultConfig.http.Timeout
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
	if s.ContextPath != nil {
		return *s.ContextPath
	}
	return *defaultConfig.http.ContextPath
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
// @Description RateLimiterConfig is the HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	// Rate limiter tokens per second threshold.
	Tps *int `yaml:"tps,omitempty" json:"tps,omitempty" default:"1024" example:"1024"`
	// Rate limiter token bucket size (bursts threshold).
	Size *int `yaml:"size,omitempty" json:"size,omitempty" default:"1024" example:"1024"`
	// The list of ips to whitelist in rate limiting.
	WhiteList []string `yaml:"white-list,omitempty" json:"white-list,omitempty" default:""`
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

// Validate validates the HTTP server configuration.
func (s *HTTPServerConfig) Validate() error {
	if s.ContextPath != nil && !strings.HasPrefix(*s.ContextPath, "/") {
		return fmt.Errorf("context-path must start with a slash: %s", *s.ContextPath)
	}
	return nil
}
