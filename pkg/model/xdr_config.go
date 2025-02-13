package model

type XDRConfig struct {
	LocalHost     string
	Rewind        string
	MaxConns      *int
	ReadTimeout   *int64
	WriteTimeout  *int64
	StartTimeout  *int64
	PollingPeriod *int64
}

// GetMaxConnsOrDefault returns MaxConns value or default.
func (c *XDRConfig) GetMaxConnsOrDefault() int {
	if c.MaxConns != nil {
		return *c.MaxConns
	}
	return *defaultConfig.xdrConfig.MaxConns
}

// GetReadTimeoutOrDefault returns ReadTimeout value or default.
func (c *XDRConfig) GetReadTimeoutOrDefault() int64 {
	if c.ReadTimeout != nil {
		return *c.ReadTimeout
	}
	return *defaultConfig.xdrConfig.ReadTimeout
}

// GetWriteTimeoutOrDefault returns WriteTimeout value or default.
func (c *XDRConfig) GetWriteTimeoutOrDefault() int64 {
	if c.WriteTimeout != nil {
		return *c.WriteTimeout
	}
	return *defaultConfig.xdrConfig.WriteTimeout
}

// GetStartTimeoutOrDefault returns StartTimeout value or default.
func (c *XDRConfig) GetStartTimeoutOrDefault() int64 {
	if c.StartTimeout != nil {
		return *c.StartTimeout
	}
	return *defaultConfig.xdrConfig.StartTimeout
}

// GetPollingPeriodOrDefault returns PollingPeriod value or default.
func (c *XDRConfig) GetPollingPeriodOrDefault() int64 {
	if c.PollingPeriod != nil {
		return *c.PollingPeriod
	}
	return *defaultConfig.xdrConfig.PollingPeriod
}
