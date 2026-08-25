## dto.LoggerConfig
LoggerConfig represents the backup service logger configuration.

| Field           | Description                                                                                | Default Value |
|-----------------|--------------------------------------------------------------------------------------------|---------------|
| `file-writer`   | File writer logging configuration.<br>See: [dto.FileLoggerConfig](dto.fileloggerconfig.md) |               |
| `format`        | Format is the logger format (PLAIN, JSON).                                                 | `PLAIN`       |
| `level`         | Level is the logger level.                                                                 | `INFO`        |
| `stdout-writer` | Whether to enable logging to the standard output.                                          | `true`        |
