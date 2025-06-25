## dto.AerospikeCluster
AerospikeCluster represents the configuration for an Aerospike cluster for backup.

| Field                    | Description                                                                                                                                                                                                | Default Value |
|--------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 🔴 `seed-nodes`           | The seed nodes details.<br>Array of: [dto.SeedNode](dto.seednode.md)                                                                                                                                       |               |
| `conn-timeout`           | The connection timeout in milliseconds.                                                                                                                                                                    | `30000`       |
| `credentials`            | The authentication details to the Aerospike cluster.<br>See: [dto.Credentials](dto.credentials.md)                                                                                                         |               |
| `label`                  | The cluster name. Optional: used only in logs and error messages.                                                                                                                                          |               |
| `max-parallel-scans`     | Specifies the maximum number of parallel scans allowed across the cluster.<br>This property helps reduce the load on the cluster and is shared among all backups using the cluster.<br>Default: unlimited. |               |
| `tls`                    | The cluster TLS configuration.<br>See: [dto.TLS](dto.tls.md)                                                                                                                                               |               |
| `use-services-alternate` | Whether should use "services-alternate" instead of "services" in info request during cluster tending.                                                                                                      | `false`       |

🔴 = Required field