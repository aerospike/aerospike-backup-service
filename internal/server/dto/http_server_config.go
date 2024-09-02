package dto

import (
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
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

// Validate validates the HTTP server configuration.
func (s *HTTPServerConfig) Validate() error {
	if s == nil {
		return nil
	}

	if s.ContextPath != nil && !strings.HasPrefix(*s.ContextPath, "/") {
		return fmt.Errorf("context-path must start with a slash: %s", *s.ContextPath)
	}
	if s.Timeout != nil && *s.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative: %d", *s.Timeout)
	}
	return nil
}

func (s *HTTPServerConfig) ToModel() *model.HTTPServerConfig {
	if s == nil {
		return nil
	}

	return &model.HTTPServerConfig{
		Address:     s.Address,
		Port:        s.Port,
		Rate:        s.Rate.ToModel(),
		ContextPath: s.ContextPath,
		Timeout:     s.Timeout,
	}
}

func (s *HTTPServerConfig) fromModel(m *model.HTTPServerConfig) {
	if m == nil {
		return
	}
	s.Address = m.Address
	s.Port = m.Port
	s.Rate = &RateLimiterConfig{}
	s.Rate.fromModel(m.Rate)
	s.ContextPath = m.ContextPath
	s.Timeout = m.Timeout
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

func (r *RateLimiterConfig) ToModel() *model.RateLimiterConfig {
	if r == nil {
		return nil
	}

	return &model.RateLimiterConfig{
		Tps:       r.Tps,
		Size:      r.Size,
		WhiteList: r.WhiteList,
	}
}

func (r *RateLimiterConfig) fromModel(m *model.RateLimiterConfig) {
	if m == nil {
		return
	}
	r.Tps = m.Tps
	r.Size = m.Size
	r.WhiteList = m.WhiteList
}
