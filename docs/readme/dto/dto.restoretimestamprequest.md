## dto.RestoreTimestampRequest
RestoreTimestampRequest represents a restore by timestamp operation request.

| Field                | Description                                                                                                                                              | Default Value |
|----------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 📍 `routine`          | The backup routine name.                                                                                                                                 |               |
| 📍 `time`             | Required epoch time (in millis) for recovery. The closest backup before the timestamp will be applied.                                                   |               |
| `destination`        | The details of the Aerospike destination cluster.<br>Mutually exclusive with 'destination-name'.<br>See: [dto.AerospikeCluster](dto.aerospikecluster.md) |               |
| `destination-name`   | Link to one of preconfigured clusters.<br>Mutually exclusive with 'destination'.                                                                         |               |
| `disable-reordering` | Disable reverse order of incremental backups optimisation.                                                                                               | `false`       |
| `policy`             | Restore policy to use in the operation.<br>See: [dto.RestorePolicy](dto.restorepolicy.md)                                                                |               |
| `secret-agent`       | Secret Agent configuration (optional).<br>Mutually exclusive with 'secret-agent-name'.<br>See: [dto.SecretAgent](dto.secretagent.md)                     |               |
| `secret-agent-name`  | Secret Agent configuration (optional). Link to one of preconfigured agents.<br>Mutually exclusive with 'secret-agent'.                                   |               |

📍 = Required field