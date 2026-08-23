## dto.ServiceConfig
ServiceConfig represents the backup service configuration properties.

| Field    | Description                                                                                                             |
|----------|-------------------------------------------------------------------------------------------------------------------------|
| `backup` | Backup contains service-level backup settings.<br>See: [dto.BackupCommonConfig](dto.backupcommonconfig.md)              |
| `http`   | HTTPServer is the backup service HTTP server configuration.<br>See: [dto.HTTPServerConfig](dto.httpserverconfig.md)     |
| `https`  | HTTPSServer is the backup service HTTPS server configuration.<br>See: [dto.HTTPSServerConfig](dto.httpsserverconfig.md) |
| `logger` | Logger is the backup service logger configuration.<br>See: [dto.LoggerConfig](dto.loggerconfig.md)                      |
