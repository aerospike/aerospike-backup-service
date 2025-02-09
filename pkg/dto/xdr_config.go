package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// XDRConfig represents the configuration for XDR backups.
// @Description XDRConfig represents the configuration for XDR backups.
type XDRConfig struct {
	DC            string `yaml:"dc,omitempty" json:"dc,omitempty" example:"us-west"`
	LocalHost     string `yaml:"local-host,omitempty" json:"local-host,omitempty" example:"127.0.0.1"`
	LocalPort     int    `yaml:"local-port,omitempty" json:"local-port,omitempty" example:"4000"`
	Rewind        string `yaml:"rewind,omitempty" json:"rewind,omitempty" example:"2023-01-01T00:00:00Z"`
	MaxConns      *int   `yaml:"max-conns,omitempty" json:"max-conns,omitempty" example:"100"`
	ReadTimeout   *int64 `yaml:"read-timeout,omitempty" json:"read-timeout,omitempty" example:"5000"`
	WriteTimeout  *int64 `yaml:"write-timeout,omitempty" json:"write-timeout,omitempty" example:"5000"`
	StartTimeout  *int64 `yaml:"start-timeout,omitempty" json:"start-timeout,omitempty" example:"3000"`
	PollingPeriod *int64 `yaml:"polling-period,omitempty" json:"polling-period,omitempty" example:"60000"`
}

func (x *XDRConfig) Validate() error {
	if x == nil {
		return nil
	}

	if x.DC == "" {
		return errValidationEmptyField("dc")
	}

	if x.LocalHost == "" {
		return errValidationEmptyField("local-host")
	}

	if x.LocalPort <= 0 {
		return errValidationNonPositive("local-port", x.LocalPort)
	}

	if x.MaxConns != nil && *x.MaxConns <= 0 {
		return errValidationNonPositive("max-conns", *x.MaxConns)
	}

	if x.ReadTimeout != nil && *x.ReadTimeout < 0 {
		return errValidationNegative("read-timeout", *x.ReadTimeout)
	}

	if x.WriteTimeout != nil && *x.WriteTimeout < 0 {
		return errValidationNegative("write-timeout", *x.WriteTimeout)
	}

	if x.StartTimeout != nil && *x.StartTimeout < 0 {
		return errValidationNegative("start-timeout", *x.StartTimeout)
	}

	if x.PollingPeriod != nil && *x.PollingPeriod < 0 {
		return errValidationNegative("polling-period", *x.PollingPeriod)
	}

	return nil
}

func (x *XDRConfig) ToModel() *model.XDRConfig {
	if x == nil {
		return nil
	}

	return &model.XDRConfig{
		DC:            x.DC,
		LocalHost:     x.LocalHost,
		LocalPort:     x.LocalPort,
		Rewind:        x.Rewind,
		MaxConns:      util.ValueOrZero(x.MaxConns),
		ReadTimeout:   util.ValueOrZero(x.ReadTimeout),
		WriteTimeout:  util.ValueOrZero(x.WriteTimeout),
		StartTimeout:  util.ValueOrZero(x.StartTimeout),
		PollingPeriod: util.ValueOrZero(x.PollingPeriod),
	}
}

func newXDRConfigFromModel(m *model.XDRConfig) *XDRConfig {
	if m == nil {
		return nil
	}

	return &XDRConfig{
		DC:            m.DC,
		LocalHost:     m.LocalHost,
		LocalPort:     m.LocalPort,
		Rewind:        m.Rewind,
		MaxConns:      &m.MaxConns,
		ReadTimeout:   &m.ReadTimeout,
		WriteTimeout:  &m.WriteTimeout,
		StartTimeout:  &m.StartTimeout,
		PollingPeriod: &m.PollingPeriod,
	}
}
