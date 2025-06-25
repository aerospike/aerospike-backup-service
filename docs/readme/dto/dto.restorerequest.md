## dto.RestoreRequest
RestoreRequest represents a restore operation request.

| Field                   | Description                                                                                                                                                                             |
|-------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 🔴 `backup-data-path`    | Path to the data from storage root.<br>You can obtain this value by:<br>- Browsing the storage UI, or<br>- Reading the `key` field in the response from GET `v1/backups/full/{routine}` |
| `destination`           | The details of the Aerospike destination cluster.<br>Mutually exclusive with 'destination-name'.<br>See: [dto.AerospikeCluster](dto.aerospikecluster.md)                                |
| `destination-name`      | Link to one of preconfigured clusters.<br>Mutually exclusive with 'destination'.                                                                                                        |
| `policy`                | Restore policy to use in the operation.<br>See: [dto.RestorePolicy](dto.restorepolicy.md)                                                                                               |
| `secret-agent`          | Secret Agent configuration (optional).<br>Mutually exclusive with 'secret-agent-name'.<br>See: [dto.SecretAgent](dto.secretagent.md)                                                    |
| `secret-agent-name`     | Secret Agent configuration (optional). Link to one of preconfigured agents.<br>Mutually exclusive with 'secret-agent'.                                                                  |
| `source`                | The details of the storage configuration.<br>Mutually exclusive with 'source-name'.<br>See: [dto.Storage](dto.storage.md)                                                               |
| `source-name`           | Link to one of preconfigured storages.<br>Mutually exclusive with 'source'.                                                                                                             |

🔴 = Required field