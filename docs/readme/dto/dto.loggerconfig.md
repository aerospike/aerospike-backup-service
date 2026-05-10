## dto.LoggerConfig
LoggerConfig represents the backup service logger configuration.

| Field           | Description                                                                                | Default Value | Possible Values                                      |
|-----------------|--------------------------------------------------------------------------------------------|---------------|------------------------------------------------------|
| `file-writer`   | File writer logging configuration.<br>See: [dto.FileLoggerConfig](dto.fileloggerconfig.md) |               |                                                      |
| `format`        | Format is the logger format (PLAIN, JSON).                                                 | `PLAIN`       | `PLAIN`, `JSON`                                      |
| `level`         | Level is the logger level.                                                                 | `INFO`        | `TRACE`, `DEBUG`, `INFO`, `WARN`, `WARNING`, `ERROR` |
| `stdout-writer` | Whether to enable logging to the standard output.                                          | `true`        |                                                      |
