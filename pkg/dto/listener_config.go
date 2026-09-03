package dto

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ListenerConfig represents the listen settings shared by the HTTP and HTTPS servers.
type ListenerConfig struct {
	// Disabled controls whether the listener is disabled.
	Disabled bool `yaml:"disabled,omitempty" json:"disabled,omitempty" default:"false"`
	// The address to listen on.
	Address string `yaml:"address,omitempty" json:"address,omitempty" default:"0.0.0.0" example:"0.0.0.0"`
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

func (l *ListenerConfig) validate() error {
	if l.ContextPath != "" && !strings.HasPrefix(l.ContextPath, "/") {
		return fmt.Errorf("context-path must start with a slash: %s", l.ContextPath)
	}
	if l.Timeout != nil && *l.Timeout < 0 {
		return errValidationNegative("timeout", *l.Timeout)
	}
	if l.ReadTimeout != nil && *l.ReadTimeout < 0 {
		return errValidationNegative("read-timeout", *l.ReadTimeout)
	}
	if l.WriteTimeout != nil && *l.WriteTimeout < 0 {
		return errValidationNegative("write-timeout", *l.WriteTimeout)
	}
	if l.IdleTimeout != nil && *l.IdleTimeout < 0 {
		return errValidationNegative("idle-timeout", *l.IdleTimeout)
	}
	if err := l.Rate.Validate(); err != nil {
		return fmt.Errorf("rate-limiter validation error: %w", err)
	}

	return nil
}

func (l *ListenerConfig) compare(other ListenerConfig) error {
	err := errors.Join(
		compareValues("Disabled", l.Disabled, other.Disabled),
		compareValues("Address", l.Address, other.Address),
		compareValues("ContextPath", l.ContextPath, other.ContextPath),
		comparePointers("Timeout", l.Timeout, other.Timeout),
		comparePointers("ReadTimeout", l.ReadTimeout, other.ReadTimeout),
		comparePointers("WriteTimeout", l.WriteTimeout, other.WriteTimeout),
		comparePointers("IdleTimeout", l.IdleTimeout, other.IdleTimeout),
	)
	if e := l.Rate.Compare(other.Rate); e != nil {
		err = errors.Join(err, fmt.Errorf("rate changes: %w", e))
	}

	return err
}

func (l *ListenerConfig) toModel() model.ListenerConfig {
	return model.ListenerConfig{
		Disabled:     l.Disabled,
		Address:      l.Address,
		Rate:         l.Rate.ToModel(),
		ContextPath:  l.ContextPath,
		Timeout:      millisToDuration(l.Timeout),
		ReadTimeout:  millisToDuration(l.ReadTimeout),
		WriteTimeout: millisToDuration(l.WriteTimeout),
		IdleTimeout:  millisToDuration(l.IdleTimeout),
	}
}

func newListenerFromModel(m model.ListenerConfig) ListenerConfig {
	listener := ListenerConfig{
		Disabled:     m.Disabled,
		Address:      m.Address,
		ContextPath:  m.ContextPath,
		Timeout:      durationToMillis(m.Timeout),
		ReadTimeout:  durationToMillis(m.ReadTimeout),
		WriteTimeout: durationToMillis(m.WriteTimeout),
		IdleTimeout:  durationToMillis(m.IdleTimeout),
	}
	if m.Rate != nil {
		listener.Rate = &RateLimiterConfig{}
		listener.Rate.fromModel(m.Rate)
	}

	return listener
}
