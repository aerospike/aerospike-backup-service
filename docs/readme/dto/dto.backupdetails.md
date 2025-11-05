## dto.BackupDetails
BackupDetails contains information about a finished backup.

| Field                   | Description                                                                                                                                        |
|-------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| `byte-count`            | The size of the backup in bytes.                                                                                                                   |
| `compression`           | Compression specifies the compression mode used for the backup (ZSTD or NONE).                                                                     |
| `created`               | The backup time in the ISO 8601 format.                                                                                                            |
| `duration`              | Duration represents the elapsed time taken by the backup process in seconds.                                                                       |
| `encryption`            | Encryption specifies the encryption mode used for the backup (NONE, AES128, AES256).                                                               |
| `file-count`            | The number of backup files created.                                                                                                                |
| `finished`              | The time the backup operation completed.                                                                                                           |
| `from`                  | The lower time bound of backup entities in the ISO 8601 format (for incremental backups only).                                                     |
| `key`                   | Key is the path to the backup files within the configured storage location.<br>This value can be used as `backup-data-path` in dto.RestoreRequest. |
| `namespace`             | The namespace of a backup.                                                                                                                         |
| `record-count`          | The total number of records backed up.                                                                                                             |
| `secondary-index-count` | The number of secondary indexes backed up.                                                                                                         |
| `storage`               | Storage specifies the details of the storage location where the backup is stored.<br>See: [dto.Storage](dto.storage.md)                            |
| `timestamp`             | The backup time in epoch millis.                                                                                                                   |
| `udf-count`             | The number of UDF files backed up.                                                                                                                 |
