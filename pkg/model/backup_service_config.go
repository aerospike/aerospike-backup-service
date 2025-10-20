package model

// BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig
	// Logger is the backup service logger configuration.
	Logger       *LoggerConfig
	DateEncoding *string
}

func (c BackupServiceConfig) GetHTTPServerOrDefault() *HTTPServerConfig {
	if c.HTTPServer != nil {
		return c.HTTPServer
	}

	return &defaultConfig.http
}

func (c BackupServiceConfig) GetLoggerOrDefault() *LoggerConfig {
	if c.Logger != nil {
		return c.Logger
	}

	return &defaultConfig.logger
}

var DateFormatPresets = map[string]string{
	"ISO": "2006-01-02T15-04-05",
	"US":  "Jan-02-2006-15-04-05",
	"EU":  "02-Jan-2006-15-04-05",
}
