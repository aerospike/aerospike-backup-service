## dto.FileLoggerConfig
FileLoggerConfig represents the configuration for the file logger writer.

| Field           | Description                                                                                                                                                     | Default Value |
|-----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 📍 `filename`    | Filename is the file to write logs to.                                                                                                                          |               |
| `compress`      | Determines if the rotated log files should be compressed<br>using gzip. The default is not to perform compression.                                              | `false`       |
| `maxage`        | Maximum number of days to retain old log files based on the<br>timestamp encoded in their filename. The default is not to remove old log files<br>based on age. | `7`           |
| `maxbackups`    | Maximum number of old log files to retain. The default<br>is to retain all old log files.                                                                       | `3`           |
| `maxsize`       | Maximum size in megabytes of the log file before it gets rotated.                                                                                               | `100`         |

📍 = Required field