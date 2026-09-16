## dto.Config
Config represents the service configuration file.

| Field                | Description                                                                                                                                         |
|----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| `aerospike-clusters` | AerospikeClusters is a map of Aerospike clusters that can be used by backup routines.                                                               |
| `backup-policies`    | BackupPolicies is a map of backup policies that can be used by backup routines.                                                                     |
| `backup-routines`    | BackupRoutines is a map of backup routines.                                                                                                         |
| `secret-agents`      | SecretAgents is a map of secret agents used by backup routines (for encryption keys), clusters (for credentials), and storage (for authentication). |
| `service`            | ServiceConfig contains general service settings.<br>See: [dto.ServiceConfig](dto.serviceconfig.md)                                                  |
| `storage`            | Storage is a map of storages that can be used by backup routines.                                                                                   |
