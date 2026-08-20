package dto

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// HTTPServerConfig represents the service's HTTP server configuration.
// @Description HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	// The address to listen on.
	Address string `yaml:"address,omitempty" json:"address,omitempty" default:"0.0.0.0" example:"0.0.0.0"`
	// The port to listen on.
	Port *Port `yaml:"port,omitempty" json:"port,omitempty" default:"8080" example:"8080"`
	// HTTP rate limiter configuration.
	Rate *RateLimiterConfig `yaml:"rate,omitempty" json:"rate,omitempty"`
	// ContextPath customizes path for the API endpoints.
	ContextPath string `yaml:"context-path,omitempty" json:"context-path,omitempty" default:"/"`
	// Timeout for reading HTTP request headers in milliseconds (http.Server.ReadHeaderTimeout).
	Timeout *int64 `yaml:"timeout,omitempty" json:"timeout,omitempty" default:"5000"`
	// ReadTimeout is the maximum duration in milliseconds for reading the entire request,
	// including the body (http.Server.ReadTimeout).
	ReadTimeout *int64 `yaml:"read-timeout,omitempty" json:"read-timeout,omitempty" default:"30000"`
	// WriteTimeout is the maximum duration in milliseconds before timing out writes of the response
	// (http.Server.WriteTimeout).
	WriteTimeout *int64 `yaml:"write-timeout,omitempty" json:"write-timeout,omitempty" default:"60000"`
	// IdleTimeout is the maximum amount of time in milliseconds to wait for the next request
	// when keep-alives are enabled (http.Server.IdleTimeout).
	IdleTimeout *int64 `yaml:"idle-timeout,omitempty" json:"idle-timeout,omitempty" default:"120000"`
}

// Validate validates the HTTP server configuration.
func (s *HTTPServerConfig) Validate() error {
	if s == nil {
		return nil
	}

	if s.ContextPath != "" && !strings.HasPrefix(s.ContextPath, "/") {
		return fmt.Errorf("context-path must start with a slash: %s", s.ContextPath)
	}
	if s.Timeout != nil && *s.Timeout < 0 {
		return errValidationNegative("timeout", *s.Timeout)
	}
	if s.ReadTimeout != nil && *s.ReadTimeout < 0 {
		return errValidationNegative("read-timeout", *s.ReadTimeout)
	}
	if s.WriteTimeout != nil && *s.WriteTimeout < 0 {
		return errValidationNegative("write-timeout", *s.WriteTimeout)
	}
	if s.IdleTimeout != nil && *s.IdleTimeout < 0 {
		return errValidationNegative("idle-timeout", *s.IdleTimeout)
	}

	if err := s.Rate.Validate(); err != nil {
		return fmt.Errorf("rate-limiter validation error: %w", err)
	}

	return nil
}

func (s *HTTPServerConfig) ToModel() *model.HTTPServerConfig {
	if s == nil {
		return nil
	}

	return &model.HTTPServerConfig{
		Address:      s.Address,
		Port:         s.Port.ToModel(),
		Rate:         s.Rate.ToModel(),
		ContextPath:  s.ContextPath,
		Timeout:      millisToDuration(s.Timeout),
		ReadTimeout:  millisToDuration(s.ReadTimeout),
		WriteTimeout: millisToDuration(s.WriteTimeout),
		IdleTimeout:  millisToDuration(s.IdleTimeout),
	}
}

func (s *HTTPServerConfig) fromModel(m *model.HTTPServerConfig) {
	if m == nil {
		return
	}
	s.Address = m.Address
	s.Port = NewPortFromModel(m.Port)
	if m.Rate != nil {
		s.Rate = &RateLimiterConfig{}
		s.Rate.fromModel(m.Rate)
	}
	s.ContextPath = m.ContextPath
	s.Timeout = durationToMillis(m.Timeout)
	s.ReadTimeout = durationToMillis(m.ReadTimeout)
	s.WriteTimeout = durationToMillis(m.WriteTimeout)
	s.IdleTimeout = durationToMillis(m.IdleTimeout)
}

// Compare HTTPServerConfig object with another and return detailed errors.
func (s *HTTPServerConfig) Compare(other *HTTPServerConfig) error {
	if s == nil && other == nil {
		return nil
	}
	if s == nil {
		return errors.New("HTTPServer added")
	}
	if other == nil {
		return errors.New("HTTPServer removed")
	}

	var err = errors.Join(
		compareValues("Address", s.Address, other.Address),
		comparePointers("Port", s.Port, other.Port),
		compareValues("ContextPath", s.ContextPath, other.ContextPath),
		comparePointers("Timeout", s.Timeout, other.Timeout),
		comparePointers("ReadTimeout", s.ReadTimeout, other.ReadTimeout),
		comparePointers("WriteTimeout", s.WriteTimeout, other.WriteTimeout),
		comparePointers("IdleTimeout", s.IdleTimeout, other.IdleTimeout),
	)

	if e := s.Rate.Compare(other.Rate); e != nil {
		err = errors.Join(err, fmt.Errorf("rate changes: %w", e))
	}

	return err
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
// @Description RateLimiterConfig is the HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	// Rate limiter tokens per second threshold.
	Tps *int `yaml:"tps,omitempty" json:"tps,omitempty" default:"1024" example:"1024"`
	// Rate limiter token bucket size (bursts threshold).
	Size *int `yaml:"size,omitempty" json:"size,omitempty" default:"1024" example:"1024"`
	// The list of ips to exempt from rate limiting (optional).
	// Default: empty list, so rate limiting applies to all clients.
	// Use "0.0.0.0/0" to exempt all clients and effectively disable rate limiting.
	WhiteList []string `yaml:"white-list,omitempty" json:"white-list,omitempty" extensions:"x-nullable"`
}

// Validate validates the rate limiter configuration.
func (r *RateLimiterConfig) Validate() error {
	if r == nil {
		return nil
	}
	if duplicates := collections.CheckDuplicates(r.WhiteList); len(duplicates) > 0 {
		return errValidationDuplicate("white-list", duplicates)
	}
	for _, entry := range r.WhiteList {
		if _, err := netip.ParsePrefix(entry); err == nil {
			continue
		}
		if _, err := netip.ParseAddr(entry); err == nil {
			continue
		}
		return fmt.Errorf("white-list contains invalid ip or cidr: %s", entry)
	}

	return nil
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

// Compare RateLimiterConfig object with another and return detailed errors.
func (r *RateLimiterConfig) Compare(other *RateLimiterConfig) error {
	if r == nil && other == nil {
		return nil
	}
	if r == nil {
		return errors.New("RateLimiter added")
	}
	if other == nil {
		return errors.New("RateLimiter removed")
	}

	return errors.Join(
		comparePointers("Tps", r.Tps, other.Tps),
		comparePointers("Size", r.Size, other.Size),
		compareSlices("WhiteList", r.WhiteList, other.WhiteList),
	)
}
