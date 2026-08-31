# Configuration

## Configuration File Format

The configuration system in the Aerospike Backup Service is designed around modular entities—clusters, storage, backup
policies, secret agents and routines—that you define and then connect together.

A backup routine is the central piece of configuration: it ties together all the other entities to define how and when
backups occur. When defining a routine, you reference:

* A cluster to specify the Aerospike cluster to back up.
* A storage configuration that defines where the backups are stored.
* A backup policy that controls backup behavior.

Each of these referenced components must be created and named in the configuration before the routine can use them. This
modular approach lets you reuse and combine policies, clusters, and storage setups across multiple routines.

## Scheduling with Quartz

The service uses the [Quartz](https://github.com/reugn/go-quartz?tab=readme-ov-file#cron-expression-format) scheduler
for executing recurring jobs. Quartz supports rich scheduling options through cron expressions, enabling precise control
over job execution timing.

Quartz uses either:

* A 6/7-field cron expression format:
  `Seconds Minutes Hours Day-of-Month Month Day-of-Week Year` (`Year` is optional)
* Shorthand expressions for common schedules:
  `@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`

Cron expressions are evaluated in **UTC** unless you set `schedule-timezone`.
Accepted values:

* omitted or `UTC` — Coordinated Universal Time (the default; existing configs are unchanged)
* `Local` — the timezone of the host running the service (`TZ` or `/etc/localtime`)
* an IANA name such as `America/New_York` — useful in containers, which typically run in UTC

`UTC` and `Local` are case-insensitive; IANA names are case-sensitive. Abbreviations such as
`EST` and POSIX `TZ` strings are rejected at startup.

Set a service-wide default under `service.backup` (requires a restart) or override it on a
routine (can be changed through the routine API):

```yaml
service:
  backup:
    schedule-timezone: America/New_York

backup-routines:
  nightly:
    interval-cron: "0 0 2 * * ?"   # 02:00 in America/New_York
    schedule-timezone: UTC         # optional per-routine override
```

Backup folder names and the optional `timestamp-format` suffix stay UTC regardless of
`schedule-timezone`. Daylight saving applies to `Local` and IANA zones: a daily 02:30
schedule does not fire on the spring-forward day when 02:30 does not exist locally, and
daily schedules in the repeated fall-back hour fire once. Use UTC to avoid DST-driven
variations in the elapsed time between runs.

**📆 Quartz Cron Expression Examples for Backup Scheduling**

| Schedule Description                | Cron Expression       | Use Case                                                             |
|-------------------------------------|-----------------------|----------------------------------------------------------------------|
| **Midnight daily**                  | `0 0 0 * * ? *`       | Daily full backup during off-peak hours                              |
| **Every 2 hours**                   | `0 0 0/2 * * ? *`     | Frequent incremental backups throughout the day                      |
| **Every 30 minutes**                | `0 0/30 * * * ? *`    | High-frequency backup for critical, fast-changing data               |
| **Weekdays at 3 AM**                | `0 0 3 ? * MON-FRI *` | Scheduled business-day backups for non-production clusters           |
| **First day of each month at 1 AM** | `0 0 1 1 * ? *`       | Monthly archival full backup                                         |
| **Every Sunday at 2 AM**            | `0 0 2 ? * SUN *`     | Weekly full backup before a new work week starts                     |
| **Twice daily at 1 AM and 1 PM**    | `0 0 1,13 * * ? *`    | Split-day incremental backups to catch morning and afternoon changes |

## Configuration file example

<!-- DefaultConfig -->

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/aerospike/aerospike-backup-service/refs/tags/v3.6.1/docs/config.schema.json
---
aerospike-clusters:
  abs-cluster: # <--- Custom cluster name
    seed-nodes:
      - host-name: localhost
        port: 3000
    credentials:
      user: tester
      password: secrets:asbackup:psw # Password will be fetched from the secret agent
      secret-agent-name: secret-agent  # <--- Refers to the secret agent name under secret-agents

secret-agents:
  secret-agent: # <--- Custom secret agent name
    address: localhost
    port: 5000
    connection-type: TCP

storage:
  s3: # <--- Custom storage name
    s3-storage: # Storage type; can be one of "local-storage", "s3-storage", "azure-storage", "gcp-storage"
      path: backups
      bucket: as-backup-bucket
      s3-region: eu-central-1
      min-part-size: 50_000_000 # Upload chunk size in bytes (May affect performance)

backup-policies:
  dailyBackupPolicy: # <--- Custom policy name
    parallel: 8 # Parallelism level (May affect performance)
    file-limit: 1000 # Max backup file size in MB (May affect performance)
    compression: # Backup files will be compressed before uploading (May affect performance)
      mode: ZSTD
      level: 1
    retention:
      full: 10 # Retain 10 full backups
      incremental: 5 # Retain incremental backups for the 5 latest full backups

backup-routines:
  dailyLocalBackupRoutine: # <--- Custom routine name
    interval-cron: "@daily" # Full backup will be triggered daily at midnight
    incr-interval-cron: "0 0 0/2 * * ? *" # Incremental backups every 2 hours
    source-cluster: abs-cluster           # <--- Refers to the cluster name under aerospike-clusters
    storage: s3                           # <--- Refers to the storage name under storage
    backup-policy: dailyBackupPolicy      # <--- Refers to the policy name under backup-policies
    namespaces: []                        # <--- An empty list is used to configure a backup of the whole cluster

service:
  http:
    port: 8080
  logger:
    level: INFO
    file-writer:
      filename: /var/log/aerospike-backup-service.log
```

See the [`dto.Config`](readme/dto/dto.config.md) for details.
Several configuration fields in the YAML file are marked with `May affect performance`.
These settings, such as parallel, file-limit, min-part-size, and compression,
can have a significant impact on backup throughput.
We recommend experimenting with different values in your environment to find the optimal balance.

The `service` section configures the operation settings of the Aerospike Backup Service,
which include logging and HTTP endpoint. See the [`dto.ServiceConfig`](readme/dto/dto.serviceconfig.md)
for details.
HTTP rate limiter behavior:
- Rate limiting applies to all clients by default.
- Entries in `service.http.rate.white-list` are exempt from rate limiting.
- If `white-list` contains `0.0.0.0/0`, all clients are exempt and rate limiting is effectively disabled.

## Configuration with API

Each entity defined in the API specification has endpoints for reading and writing backup configurations at general or
granular levels. While the API provides full control over the configuration, for most use cases,
we recommend configuring the service with the YAML configuration file because it is easier to maintain.

For specifics and example values, see the [OpenAPI docs](https://aerospike.github.io/aerospike-backup-service/).
The endpoints defined within the configuration section allow you to view or modify the configuration file.

Endpoints ending with /config enable reading and modifying the entire file at once, while endpoints like
`/config/clusters`, `/config/policies`, `/config/routines`, and `/config/storage` provide more granular control.
Changes made through any of these endpoints are applied immediately.
However, backup processes already in progress will continue using the configuration that was active when they started.

#### Cluster connection

Cluster configuration entities denote the configuration properties needed to establish connections to Aerospike
clusters.
These connections include the cluster IP address, port number, authentication information, and more.
See [`POST: /config/clusters`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/addCluster) for the
full specification.

:warning: Use the [Aerospike Secret Agent](https://aerospike.com/docs/tools/backup#secret-agent-options) to avoid
including secrets in your configuration. See [Security](security.md) for how secrets are resolved, cached, and rotated.

#### Storage connection

This entity includes properties of connections to local or cloud storage, where the backup files are stored.
You can get information about a specific configured storage option, such as checking the cloud storage location for
a backup.
You can also add, update, or remove a storage configuration.
See the [Storage](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readAllStorage) entities
under `/config/storage` for detailed information.

:warning: ABS currently supports AWS S3, GCP, and Microsoft Azure cloud storage.

#### Backup policy

A backup policy is a set of rules that defines how backups should be performed.
It includes settings for performance tuning, data selection, encryption, compression, and other operational details.
See [`GET: /config/policies`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readPolicies) for
full details about what parameters are available to customize a backup policy.

You can save multiple policies with different configurations.
When you run
the [`POST: /config/policies`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/addPolicy) command
to create a policy, ensure that you give your policy a name that will let you quickly identify its characteristics.

#### Backup routine

A backup routine is a set of procedures that actually perform backups based on the predefined backup policy.
It includes configurations for the source cluster, storage destination, scheduling (separately for full and incremental
backups), and the scope of data to back up (such as namespaces, sets, or bins).

See the [Routines](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readRoutines) section for
command examples showing how to find all routines, get information about a specific named routine, and add, remove, or
update an existing routine.

:warning: Incremental backups are deleted if they are empty and after each full backup. System metadata is backed up
only on full backups.

## Partial backup with filter expressions

The `filter-exp` field on a backup routine applies an Aerospike [filter expression](https://aerospike.com/docs/develop/expressions/#record-filtering-with-expressions)
during scan-based backups. Only records that match the expression are included in the backup.

Filter expressions are **not** plain text (you cannot write `age > 25` directly in YAML). They are a binary format
serialized as a **base64 string**. Build the expression with an Aerospike client library, then paste the encoded
value into your routine configuration.

### Generating a filter expression

Use the [Aerospike Expressions guide](https://aerospike.com/docs/develop/expressions/#record-filtering-with-expressions)
to understand the expression API, then encode the result with your client:

**Go**

```go
import as "github.com/aerospike/aerospike-client-go/v8"

exp, err := as.ExpGreater(as.ExpIntBin("age"), as.ExpIntVal(25)).Base64()
// exp == "kwOTUQKjYWdlGQ=="
```

**Java**

```java
Expression filter = Exp.build(Exp.gt(Exp.intBin("age"), Exp.val(25)));
System.out.println(filter.getBase64());
```

**Python**

```python
from aerospike_helpers import expressions as exp

encoded = exp.GT(exp.IntBin("age"), 25).compile()
# Use client.get_expression_base64(encoded) to get the base64 string
```

If you already have a filter on the cluster (for example an XDR shipping filter or expression secondary index), you can
reuse its base64 value:

```bash
asinfo -v "xdr-get-filter:dc=DC1;namespace=test;b64=true"
```

### Configuration example

`filter-exp` can only be used when backing up a **single set** (or all sets in a namespace with no `set-list`).
It is mutually exclusive with multi-set backup.

```yaml
backup-routines:
  adultsBackup:
    interval-cron: "@daily"
    source-cluster: abs-cluster
    storage: s3
    backup-policy: dailyBackupPolicy
    namespaces:
      - test
    set-list:
      - users
    filter-exp: "kwOTUQKjYWdlGQ=="  # age > 25
```

### Common examples

| Filter | Base64 value |
|--------|--------------|
| `age > 25` | `kwOTUQKjYWdlGQ==` |
| `country = "US"` | `kwGTUQOnY291bnRyeaMDVVM=` |
| `age >= 18 AND (country = "US" OR country = "CA")` | `kxCTBJNRAqNhZ2USkxGTAZNRA6djb3VudHJ5owNVU5MBk1EDp2NvdW50cnmjA0NB` |

For more complex logic (metadata filters, list/map operations, geo filters, etc.), see the
[Aerospike Expressions documentation](https://aerospike.com/docs/develop/expressions/).

## FAQ

### What timezone do backup schedules use?

Backup cron expressions are evaluated in UTC by default. Set `schedule-timezone` to `UTC`,
`Local`, or an IANA name such as `America/New_York` on `service.backup` (service-wide default;
requires a restart) or on a routine (overrides the default). Backup paths and the
`timestamp-format` suffix remain UTC for every timezone setting.

### What happens when a backup doesn't finish before another starts (for the same routine)?

- **Full Backups:**
    - Full backups cannot overlap. If a scheduled full backup is due to start but the previous one is still running, the
      new backup is skipped entirely. It is not queued but will wait for the next scheduled execution.
    - Full backups always take priority over incremental backups. If an incremental backup is running when a full backup
      is scheduled, the full backup will start as planned, and the incremental backup will continue running without
      interruption.

- **Incremental Backups:**
    - By default, incremental backups are skipped if any other backup (full or incremental) is still running.
      This behavior can be overridden using the `concurrent-incremental` field in the
      [backup policy](readme/dto/dto.backuppolicy.md), which allows incremental backups to run concurrently.
    - Incremental backups will not run until at least one full backup has been successfully completed.

### Can multiple backup routines be performed simultaneously?

Yes, multiple backup routines can run in parallel. Furthermore, it is possible to back up different namespaces from the
same cluster using separate routines with different schedules, all running simultaneously.

To manage resource utilization, you can configure the `cluster.max-parallel-scans` property to limit the number of read
threads operating on a single cluster.

### How does the backup service identify what data to back up during incremental backups?

The Aerospike Backup Service uses Aerospike's scan operation to identify and backup records,
with different behaviors for full and incremental backups:

* **Full Backups:**
    * Capture all records in the specified namespaces/sets without any time filter.
      The service uses a scan operation with no lower time boundary (modAfter = 0).

* **Incremental Backups:**:
    * Only capture records that have been modified since the last successful backup (full or incremental). The service
      tracks the timestamp of the last backup in a metadata YAML file stored alongside the backup data. This timestamp
      becomes the lower time boundary (modAfter parameter) for the next incremental backup.
      For the upper time boundary (modBefore), two approaches are available:

        - **Default Behavior (Open-ended)**: No upper time boundary is set. This means records modified during the
          backup process itself might be included in the backup, but with unpredictable results. For example, if a
          backup starts at 12:00 and runs for 5 minutes, a record created at 12:01 might be included with either its new
          or old version—there's no guarantee which state will be captured.
        - **Sealed Backups**: When the sealed property in the backup policy is set to true, the backup service will only
          include records modified before the backup start time. While this creates a more precise point-in-time
          snapshot, there's still unpredictability: if a record is updated during the backup process, it might be
          captured in its old state or excluded entirely from the backup.

Users should select the appropriate approach based on their recovery point objectives and consistency requirements. The
default open-ended approach ensures better data coverage but with some state unpredictability, while sealed backups
provide better point-in-time consistency but might miss records updated during the backup process.
