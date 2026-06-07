## dto.RestoreTimestampRequest
RestoreTimestampRequest represents a request to restore the database to a specific time. `destination`, `source`, and `secret-agent` are optional overrides; omitted values are resolved from the referenced routine.

| Field                | Description                                                                                                                                              | Default Value |
|----------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 📍 `routine`          | The backup routine name.                                                                                                                                 |               |
| 📍 `time`             | Required epoch time (in millis) for recovery. The closest backup before the timestamp will be applied.                                                   |               |
| `destination`        | The details of the Aerospike destination cluster.<br>Mutually exclusive with 'destination-name'.<br>See: [dto.AerospikeCluster](dto.aerospikecluster.md) |               |
| `destination-name`   | Link to one of preconfigured clusters.<br>Mutually exclusive with 'destination'.                                                                         |               |
| `disable-reordering` | Disable reverse order of incremental backups optimisation.                                                                                               | `false`       |
| `policy`             | Restore policy to use in the operation.<br>See: [dto.TimestampRestorePolicy](dto.timestamprestorepolicy.md)                                              |               |
| `secret-agent`       | Secret Agent configuration (optional).<br>Mutually exclusive with 'secret-agent-name'.<br>See: [dto.SecretAgent](dto.secretagent.md)                     |               |
| `secret-agent-name`  | Secret Agent configuration (optional). Link to one of preconfigured agents.<br>Mutually exclusive with 'secret-agent'.                                   |               |
| `source`             | The details of the storage configuration.<br>Mutually exclusive with 'source-name'.<br>See: [dto.Storage](dto.storage.md)                                |               |
| `source-name`        | Link to one of preconfigured storages.<br>Mutually exclusive with 'source'.                                                                              |               |

📍 = Required field