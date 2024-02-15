package model

import (
	"fmt"
	"strings"
)

// HTTPServerConfig represents the service's HTTP server configuration.
// @Description HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	// The address to listen on.
	Address string `yaml:"address,omitempty" json:"address,omitempty" default:"0.0.0.0"`
	// The port to listen on.
	Port int `yaml:"port,omitempty" json:"port,omitempty" default:"8080"`
	// HTTP rate limiter configuration.
	Rate RateLimiterConfig `yaml:"rate,omitempty" json:"rate,omitempty"`
	// ContextPath customizes path for the API endpoints.
	ContextPath string `yaml:"context-path,omitempty" json:"context-path,omitempty" default:"/"`
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
// @Description RateLimiterConfig is the HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	// Rate limiter tokens per second threshold.
	Tps int `yaml:"tps,omitempty" json:"tps,omitempty" default:"1024"`
	// Rate limiter token bucket size (bursts threshold).
	Size int `yaml:"size,omitempty" json:"size,omitempty" default:"1024"`
	// The list of ips to whitelist in rate limiting.
	WhiteList []string `yaml:"white-list,omitempty" json:"white-list,omitempty" default:""`
}

// NewHTTPServerWithDefaultValues returns a new HTTPServerConfig with default values.
func NewHTTPServerWithDefaultValues() *HTTPServerConfig {
	return &HTTPServerConfig{
		Address: "0.0.0.0",
		Port:    8080,
		Rate: RateLimiterConfig{
			Tps:       1024,
			Size:      1024,
			WhiteList: []string{},
		},
		ContextPath: "/",
	}
}

// Validate validates the HTTP server configuration.
func (s *HTTPServerConfig) Validate() error {
	if !strings.HasPrefix(s.ContextPath, "/") {
		return fmt.Errorf("context-path must start with a slash: %s", s.ContextPath)
	}
	return nil
}
