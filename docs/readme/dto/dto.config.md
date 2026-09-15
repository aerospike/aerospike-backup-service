## dto.Config
Config represents the service configuration file.

| Field                | Description                                                                                                                                                                                 |
|----------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `aerospike-clusters` | AerospikeClusters is a map of Aerospike clusters that can be used by backup routines.<br>Names must be a single path segment.                                                               |
| `backup-policies`    | BackupPolicies is a map of backup policies that can be used by backup routines.<br>Names must be a single path segment.                                                                     |
| `backup-routines`    | BackupRoutines is a map of backup routines.<br>Names must be a single path segment: the name is the folder the routine's backups live in.                                                   |
| `secret-agents`      | SecretAgents is a map of secret agents used by backup routines (for encryption keys), clusters (for credentials), and storage (for authentication).<br>Names must be a single path segment. |
| `service`            | ServiceConfig contains general service settings.<br>See: [dto.ServiceConfig](dto.serviceconfig.md)                                                                                          |
| `storage`            | Storage is a map of storages that can be used by backup routines.<br>Names must be a single path segment.                                                                                   |
