## dto.ServiceConfig
ServiceConfig represents the backup service configuration properties.

| Field    | Description                                                                                                             |
|----------|-------------------------------------------------------------------------------------------------------------------------|
| `backup` | Backup contains service-level backup settings.<br>See: [dto.BackupCommonConfig](dto.backupcommonconfig.md)              |
| `http`   | ServerHTTP is the backup service HTTP server configuration.<br>See: [dto.ServerConfigHTTP](dto.serverconfighttp.md)     |
| `https`  | ServerHTTPS is the backup service HTTPS server configuration.<br>See: [dto.ServerConfigHTTPS](dto.serverconfighttps.md) |
| `logger` | Logger is the backup service logger configuration.<br>See: [dto.LoggerConfig](dto.loggerconfig.md)                      |
