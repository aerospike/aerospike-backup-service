package dto

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// HTTPServerConfig represents the service's HTTP server configuration.
// @Description HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	ListenerConfig `yaml:",inline"`
	// The port to listen on.
	Port *Port `yaml:"port,omitempty" json:"port,omitempty" default:"8080" example:"8080"`
}

// Validate validates the HTTP server configuration.
func (s *HTTPServerConfig) Validate() error {
	if s == nil {
		return nil
	}

	if err := s.Port.Validate(); err != nil {
		return err
	}
	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	return s.ListenerConfig.validate()
}

func (s *HTTPServerConfig) ToModel() *model.ServerConfigHTTP {
	if s == nil {
		return nil
	}

	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	return &model.ServerConfigHTTP{
		ListenerConfig: s.ListenerConfig.toModel(),
		Port:           s.Port.ToModel(),
	}
}

func (s *HTTPServerConfig) fromModel(m *model.ServerConfigHTTP) {
	if m == nil {
		return
	}
	s.ListenerConfig = newListenerFromModel(m.ListenerConfig)
	s.Port = NewPortFromModel(m.Port)
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

	return errors.Join(
		//nolint:staticcheck // We want to call embedded methods with embedded struct name.
		s.ListenerConfig.compare(other.ListenerConfig),
		comparePointers("Port", s.Port, other.Port),
	)
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
