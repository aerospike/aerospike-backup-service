package model

// HTTPServerConfig represents the service's HTTP server configuration.
// @Description HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	Address string            `yaml:"address,omitempty" json:"address,omitempty" default:"0.0.0.0" example:"0.0.0.0"`
	Port    int               `yaml:"port,omitempty" json:"port,omitempty" default:"8080" example:"8080"`
	Rate    RateLimiterConfig `yaml:"rate,omitempty" json:"rate,omitempty"`
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
// @Description RateLimiterConfig is the HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	Tps       int      `yaml:"tps,omitempty" json:"tps,omitempty" default:"1024" example:"1024"`
	Size      int      `yaml:"size,omitempty" json:"size,omitempty" default:"1024" example:"1024"`
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
	}
}
