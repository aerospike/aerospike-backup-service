## dto.BackupCommonConfig
BackupCommonConfig represents service-level backup settings.

| Field              | Description                                                                                                                                                                                                       | Default Value | Possible Values   |
|--------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|-------------------|
| `backup-mode`      | Backup mode for the entire service instance: scan (default) or server.                                                                                                                                            | `scan`        | `scan`, `server`  |
| `timestamp-format` | Encoding for backup date in human-readable format in backup file paths (optional).<br>Allowed values:<br>* ISO (e.g. 2006-01-02T15-04-05)<br>* EU (e.g. 02-Jan-2006-15-04-05)<br>* US (e.g. Jan-02-2006-15-04-05) |               | `ISO`, `US`, `EU` |
