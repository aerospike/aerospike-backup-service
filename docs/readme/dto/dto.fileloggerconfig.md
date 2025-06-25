## dto.FileLoggerConfig
FileLoggerConfig represents the configuration for the file logger writer.

| Field           | Description                                                                                                                                                                   | Default Value |
|-----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 🔴 `filename`    | Filename is the file to write logs to.                                                                                                                                        |               |
| `compress`      | Compress determines if the rotated log files should be compressed<br>using gzip. The default is not to perform compression.                                                   | `false`       |
| `maxage`        | MaxAge is the maximum number of days to retain old log files based on the<br>timestamp encoded in their filename. The default is not to remove old log files<br>based on age. |               |
| `maxbackups`    | MaxBackups is the maximum number of old log files to retain. The default<br>is to retain all old log files.                                                                   |               |
| `maxsize`       | MaxSize is the maximum size in megabytes of the log file before it gets rotated.                                                                                              |               |

🔴 = Required field