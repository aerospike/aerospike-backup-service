## dto.GcpStorage

| Field                                         | Description                                                                                                                          | Default Value |
|-----------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|---------------|
| `bucket-name`<span style="color:red">*</span> | GCP storage bucket name.                                                                                                             |               |
| `endpoint`                                    | Alternative url.<br>It is not recommended to use an alternate URL in a production environment.                                       |               |
| `key`                                         | Key is the service account key in JSON format.<br>This is sensitive information. Can be a path in secret agent or an actual value.   |               |
| `key-file-path`                               | Path to the file containing the service account key in JSON format.                                                                  |               |
| `min-part-size`                               | The minimum size in bytes of individual GCP storage chunks.                                                                          | `5.24288e+06` |
| `path`                                        | The root path for the backup repository. If not specified, backups will be saved in the bucket's root.                               |               |
| `secret-agent`                                | Secret Agent configuration (optional).<br>Mutually exclusive with 'secret-agent-name'.<br>See: [dto.SecretAgent](dto.secretagent.md) |               |
| `secret-agent-name`                           | Secret Agent configuration (optional). Link to one of preconfigured agents.<br>Mutually exclusive with 'secret-agent'.               |               |
| `storage-class`                               | StorageClass defines the storage class for data and metadata objects.<br>See: [dto.GcpStorageClass](dto.gcpstorageclass.md)          |               |
