package model

// HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	Address string            `yaml:"address,omitempty" json:"address,omitempty"`
	Port    int               `yaml:"port,omitempty" json:"port,omitempty"`
	Rate    RateLimiterConfig `yaml:"rate,omitempty" json:"rate,omitempty"`
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	Tps       int      `yaml:"tps,omitempty" json:"tps,omitempty"`
	Size      int      `yaml:"size,omitempty" json:"size,omitempty"`
	WhiteList []string `yaml:"white-list,omitempty" json:"white-list,omitempty"`
}

// NewHttpServerWithDefaultValues returns a new HTTPServerConfig with default values.
func NewHttpServerWithDefaultValues() *HTTPServerConfig {
	return &HTTPServerConfig{
		Address: "0.0.0.0",
		Port:    8080,
		Rate: RateLimiterConfig{
			Tps:       1024,
			Size:      1024,
			WhiteList: []string{},
		},
	}
}
