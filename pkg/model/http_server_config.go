package model

import (
	"errors"
	"fmt"
	"reflect"
)

// HTTPServerConfig represents the service's HTTP server configuration.
// @Description HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	// The address to listen on.
	Address *string
	// The port to listen on.
	Port *int
	// HTTP rate limiter configuration.
	Rate *RateLimiterConfig
	// ContextPath customizes path for the API endpoints.
	ContextPath *string
	// Timeout for http server operations in milliseconds.
	Timeout *int
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

// Compare HTTPServerConfig object with another and return detailed errors.
func (s *HTTPServerConfig) Compare(other *HTTPServerConfig) error {
	if s == nil && other == nil {
		return nil
	}
	if s == nil {
		return fmt.Errorf("HTTPServer added")
	}
	if other == nil {
		return fmt.Errorf("HTTPServer removed")
	}

	var err error

	if e := comparePointers("Address", s.Address, other.Address); e != nil {
		err = errors.Join(err, e)
	}
	if e := comparePointers("Port", s.Port, other.Port); e != nil {
		err = errors.Join(err, e)
	}
	if e := comparePointers("ContextPath", s.ContextPath, other.ContextPath); e != nil {
		err = errors.Join(err, e)
	}
	if e := comparePointers("Timeout", s.Timeout, other.Timeout); e != nil {
		err = errors.Join(err, e)
	}
	if e := s.Rate.Compare(other.Rate); e != nil {
		err = errors.Join(err, fmt.Errorf("rate changes: %w", e))
	}

	return err
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
// @Description RateLimiterConfig is the HTTP server rate limiter configuration.
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

// Compare RateLimiterConfig object with another and return detailed errors.
func (r *RateLimiterConfig) Compare(other *RateLimiterConfig) error {
	if r == nil && other == nil {
		return nil
	}
	if r == nil {
		return fmt.Errorf("RateLimiter added")
	}
	if other == nil {
		return fmt.Errorf("RateLimiter removed")
	}

	var err error

	if e := comparePointers("Tps", r.Tps, other.Tps); e != nil {
		err = errors.Join(err, e)
	}
	if e := comparePointers("Size", r.Size, other.Size); e != nil {
		err = errors.Join(err, e)
	}
	if !reflect.DeepEqual(r.WhiteList, other.WhiteList) {
		err = errors.Join(err, fmt.Errorf("WhiteList changed: %v -> %v", r.WhiteList, other.WhiteList))
	}

	return err
}
