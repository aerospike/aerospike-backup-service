# Aerospike Backup Service

[![Build](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml/badge.svg)](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aerospike/aerospike-backup-service)](https://pkg.go.dev/github.com/aerospike/aerospike-backup-service)
[![codecov](https://codecov.io/gh/aerospike/aerospike-backup-service/branch/main/graph/badge.svg)](https://codecov.io/gh/aerospike/aerospike-backup-service)

The Aerospike Backup Service provides a set of REST API endpoints to back up and restore a cluster.
You can perform full and incremental backups and set different backup policies and schedules.
There are also several monitoring endpoints to check backup information.

Use the [OpenAPI generation script](./scripts/generate-openapi.sh) to generate an OpenAPI specification for the service.
A pre-built OpenAPI specification is available in Swagger
format [here](https://aerospike.github.io/aerospike-backup-service/).

# Table of contents

<!-- toc -->

- [Getting started](#getting-started)
- [User guide](#user-guide)
    * [Run](#run)
    * [Configuration](#configuration)
        + [Configuration File Format](#configuration-file-format)
        + [Scheduling with Quartz](#scheduling-with-quartz)
        + [Configuration file example](#configuration-file-example)
        + [Configuration with API](#configuration-with-api)
    * [Monitoring](#monitoring)
        + [Backup Progress Monitoring](#backup-progress-monitoring)
    * [Example requests and responses](#example-requests-and-responses)
        + [Backup](#backup)
        + [Restore](#restore)
- [FAQ](#faq)
    * [What happens when a backup doesn’t finish before another starts (for the same routine)?](#what-happens-when-a-backup-doesnt-finish-before-another-starts-for-the-same-routine)
    * [Can multiple backup routines be performed simultaneously?](#can-multiple-backup-routines-be-performed-simultaneously)
    * [How does the backup service identify what data to back up during incremental backups?](#how-does-the-backup-service-identify-what-data-to-back-up-during-incremental-backups)
    * [Which storage providers are supported?](#which-storage-providers-are-supported)
- [Build from source](#build-from-source)
    + [Prerequisites](#prerequisites)
    + [Build the service](#build-the-service)
    + [Build Docker image](#build-docker-image)
    + [Build Linux packages](#build-linux-packages)
    + [Release](#release)
- [Migration Guide](#migration-guide)
    * [v3.2 -> v3.3](#v32---v33)
    * [v3.1 -> v3.2](#v31---v32)
    * [v3 -> v3.1](#v3---v31)
    * [v2 -> v3](#v2---v3)

<!-- tocstop -->

# Getting started

Aerospike Backup Service reads configurations from a YAML file that is provided when the service is launched.
See [Run](#run) for specific syntax.

Linux installation packages are available
under [releases](https://github.com/aerospike/aerospike-backup-service/releases).

# User guide

This section covers basic usage scenarios for Aerospike Backup Service.

## Run

#### Binary

Run as a binary using a configuration file:

```console
./aerospike-backup-service -c config.yml
```

Help:

```console
./aerospike-backup-service -h
Aerospike Backup Service

Usage:
  aerospike-backup-service [flags]

Flags:
  -c, --config string   configuration file path/URL
  -h, --help            help for aerospike-backup-service
  -r, --remote          use remote config file
  -v, --version         version for aerospike-backup-service
```

Set the configuration file path with `-c`.

Without the `-r` flag, the file specified after `-c` is the actual configuration file.
With the `-r` flag, the file specified after `-c` contains the path or URL to the actual configuration file.

For example, you may store your configurations remotely, such as on AWS S3 storage.
In this case, you could have a `remote_config.yaml` file containing S3 details:
<!-- RemoteConfig -->

```yaml
s3-storage:
  bucket: as-backup-bucket
  path: config.yml
  s3-region: eu-central-1

```

Run the server

```bash
./aerospike-backup-service -c remote_config.yaml -r
```

#### Docker

Run in a container with a custom configuration file:

```bash
docker run -d -p 8080:8080 -v config.yml:/app/config.yml --name backup-service backup-service
```

#### Service

Run as a service. The default path for the configuration file is
`/etc/aerospike-backup-service/aerospike-backup-service.yml`.

```bash
sudo systemctl start aerospike-backup-service
```

View service logs:

```bash
sudo journalctl -u aerospike-backup-service -n 100 --no-page -f
```

## Configuration

### Configuration File Format

The configuration system in the Aerospike Backup Service is designed around modular entities—clusters, storage, backup
policies, secret agents and routines—that you define and then connect together.

A backup routine is the central piece of configuration: it ties together all the other entities to define how and when
backups occur. When defining a routine, you reference:

* A cluster to specify the Aerospike cluster to back up.
* A storage configuration that defines where the backups are stored.
* A backup policy that controls backup behavior.

Each of these referenced components must be created and named in the configuration before the routine can use them. This
modular approach lets you reuse and combine policies, clusters, and storage setups across multiple routines.

### Scheduling with Quartz

The service uses the [Quartz](https://github.com/reugn/go-quartz?tab=readme-ov-file#cron-expression-format) scheduler
for executing recurring jobs. Quartz supports rich scheduling options through cron expressions, enabling precise control
over job execution timing.

Quartz uses either:

* A 6/7-field cron expression format:
  `Seconds Minutes Hours Day-of-Month Month Day-of-Week Year` (`Year` is optional)
* Shorthand expressions for common schedules:
  `@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`

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

### Configuration file example

<!-- DefaultConfig -->

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/aerospike/aerospike-backup-service/refs/tags/v3.3.0/docs/config.schema.json
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
    connection-type: tcp

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
    port: 9000
  logger:
    level: INFO
    file-writer:
      filename: /var/log/aerospike-backup-service.log
```

See the [`dto.Config`](docs/readme/dto/dto.config.md) for details.
Several configuration fields in the YAML file are marked with `May affect performance`.
These settings, such as parallel, file-limit, min-part-size, and compression,
can have a significant impact on backup throughput.
We recommend experimenting with different values in your environment to find the optimal balance.

The `service` section configures the operation settings of the Aerospike Backup Service,
which include logging and HTTP endpoint. See the [`dto.BackupServiceConfig`](docs/readme/dto/dto.backupserviceconfig.md)
for details.

### Configuration with API

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
including secrets in your configuration.

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

## Monitoring

The service exposes a wide variety of system metrics that [Prometheus](https://prometheus.io/) can scrape, including the
following application metrics:

<!-- Metrics -->

| Name                                                        | Type      | Description                                                                                                      | Labels                 |
|-------------------------------------------------------------|-----------|------------------------------------------------------------------------------------------------------------------|------------------------|
| `aerospike_backup_service_runs_total`                       | Counter   | Successful backup runs counter (Deprecated, use `aerospike_backup_service_backup_events_total`)                  |                        |
| `aerospike_backup_service_incremental_runs_total`           | Counter   | Successful incremental backup runs counter (Deprecated, use `aerospike_backup_service_backup_events_total`)      |                        |
| `aerospike_backup_service_skip_total`                       | Counter   | Full backup skip counter (Deprecated, use `aerospike_backup_service_backup_events_total`)                        |                        |
| `aerospike_backup_service_incremental_skip_total`           | Counter   | Incremental backup skip counter (Deprecated, use `aerospike_backup_service_backup_events_total`)                 |                        |
| `aerospike_backup_service_failure_total`                    | Counter   | Full backup failure counter (Deprecated, use `aerospike_backup_service_backup_events_total`)                     |                        |
| `aerospike_backup_service_incremental_failure_total`        | Counter   | Incremental backup failure counter (Deprecated, use `aerospike_backup_service_backup_events_total`)              |                        |
| `aerospike_backup_service_duration_millis`                  | Counter   | Full backup duration in milliseconds (Deprecated, use `aerospike_backup_service_backup_duration_seconds`)        |                        |
| `aerospike_backup_service_incremental_duration_millis`      | Counter   | Incremental backup duration in milliseconds (Deprecated, use `aerospike_backup_service_backup_duration_seconds`) |                        |
| `aerospike_backup_service_backup_progress_pct`              | Gauge     | Progress of backup processes in percentage                                                                       | routine, type          |
| `aerospike_backup_service_restore_in_progress`              | Counter   | Number of restore processes running                                                                              |                        |
| `aerospike_backup_service_backup_events_total`              | Counter   | Backup service job events by routine, type (full/incremental), and outcome (success, failure, retry, skip)       | routine, type, outcome |
| `aerospike_backup_service_backup_duration_seconds`          | Histogram | Duration in seconds of finished backups by routine and type (full/incremental)                                   | routine, type          |
| `aerospike_backup_service_last_successful_backup_timestamp` | Gauge     | Unix timestamp of the last successful backup per routine                                                         | routine                |

**Example PromQL Queries**

Use these queries in Grafana panels or the Prometheus expression browser to monitor and alert on backup performance.

- ✅ Number of successful full and incremental backups for a specific routine (e.g., daily-ns1):

  `sum by (type) ( aerospike_backup_service_backup_events_total{routine="daily-ns1", outcome="success"} )`

- 🔁 Number of backup retry attempts in the past hour:
  `increase(aerospike_backup_service_backup_events_total{outcome="retry"}[1h])`

- ❌ Total number of failed backups across all routines and types:

  `sum( aerospike_backup_service_backup_events_total{outcome="failure"} )`

- 🚫 Total number of cancelled backups:
  `sum(aerospike_backup_service_backup_events_total{outcome="cancel"})`

- ⏰ Time since last backup for routine

  `time() - aerospike_backup_service_last_successful_backup_timestamp{routine="daily-ns1"}`

**Example Prometheus Alert**

- This alert fires if any backup job failure has been recorded in the last 15 minutes:

```yaml
- alert: BackupJobFailureDetected
  expr: increase(aerospike_backup_service_backup_events_total{outcome="failure"}[15m]) > 0
  labels:
    severity: warning
  annotations:
    summary: "Backup job failure detected"
    description: "At least one backup job has failed in the last 15 minutes."
```

- Alert if no successful backup in the last 24h for a specific routine (e.g., daily-ns1):

```yaml
- alert: BackupTooOld
  expr: (time() - aerospike_backup_service_last_successful_backup_timestamp{routine="daily-ns1"}) > 86400
  labels:
    severity: critical
  annotations:
    summary: "No recent backup for routine daily-ns1"
    description: "The last successful backup for routine daily-ns1 was more than 24 hours ago."
```

**Endpoints**

* `/metrics` exposes metrics for Prometheus to check performance of the backup service.
  See [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/getting_started/) for instructions.
* `/health` allows monitoring systems to check the service health.
* `/ready` checks whether the service is able to handle requests.

See
the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
on liveness and readiness probes for more information.

The HTTP metrics endpoint can be found on
the [OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/) page.

### Backup Progress Monitoring

The `aerospike_backup_service_backup_progress_pct` metric provides percentage completion for running backup processes.

**Labels**

* `routine`: Name of the backup routine
* `type`: Backup type (Full or Incremental)

#### How It's Calculated

The progress percentage is calculated as `Progress = (Records Processed / Total Estimated Records) × 100`.

**Total Records Estimation**

When a backup starts, ABS samples one partition (metadata scan only) and multiplies the sample count by total partition
count (typically 4096).

**Duration Estimation**

Uses linear extrapolation based on current progress rate `Estimated Total Time = Elapsed Time / Progress Percentage`.
Only available after 1% completion.

**Usage Notes**

This metric provides a reasonable estimate of backup progress and completion time,
though accuracy may vary depending on actual record distribution and processing conditions.
Early estimates should be interpreted with appropriate tolerance for variance.
The metric is useful for monitoring backup status and getting approximate completion times,
especially for longer-running backup operations.

## Example requests and responses

The following sections provide example requests and responses for various operations.
For full API documentation, refer to
the [Aerospike Backup Service OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/#/System/metrics).

While command-line tools such as curl, httpie, or wget work well for interacting with the API, a graphical
interface (such as [Postman](https://www.postman.com/downloads/) or [Insomnia](https://insomnia.rest/)) is generally
recommended for a more convenient and user-friendly experience.

### Backup

#### Trigger On-Demand Backup

ℹ️ *Available since v1.0*

This request starts the backup operation for the specified routine, regardless of its configured schedule.

[
`POST {{baseUrl}}/v1/backups/schedule/<routineName>?delay=<timeout>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/scheduleFullBackup)

* routineName: The name of the backup routine to trigger.
* delay (optional): Time in milliseconds to delay the start of the backup.

If the request is accepted, the server responds with Http 202 Accepted.

#### Get Current Backup

ℹ️ *Available since v1.0*

This endpoint retrieves the current statistics for a backup in progress, identified by its routine name.

[
`GET {{baseUrl}}/v1/backups/currentBackup/<routineName>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getCurrentBackup)

* routineName: The name of the routine for which to retrieve current backup information.

<details>
    <summary>Response</summary>

<!-- CurrentBackupResponse -->

```json
{
  "full": {
    "total-records": 100000,
    "done-records": 50000,
    "start-time": "2024-01-01T12:00:00Z",
    "percentage-done": 50,
    "estimated-end-time": "2024-01-01T13:00:00Z",
    "metrics": {
      "records-per-second": 1000,
      "kilobytes-per-second": 30000,
      "pipeline": 0
    }
  }
}
```

See [fields description](docs/readme/dto/dto.runningjob.md) for details.

</details>

#### Cancel Backup Job

ℹ️ *Available since v3.0*

[
`POST {{baseUrl}}/v1/backups/cancel/<routineName>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/cancelCurrentBackup)

Cancel all currently running backups (both full and incremental) for the specified routine. Partially created backups
will be deleted.

#### Retrieve Backup List

ℹ️ *Available since v1.0*

Provides a list of backups for each configured routine, including details such as creation time, duration, namespace,
and storage location.

[`GET {{baseUrl}}/v1/backups/full`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getFullBackups)

<details>
    <summary>Response</summary>

The response is a map of routine names to lists of backups.
<!-- FullBackupsResponse -->

```json
{
  "routine1": [
    {
      "created": "2024-01-01T12:00:00Z",
      "timestamp": 1704110400000,
      "finished": "2024-01-01T12:05:00Z",
      "duration": 300,
      "from": "0001-01-01T00:00:00Z",
      "namespace": "source-ns1",
      "record-count": 42,
      "byte-count": 480000,
      "file-count": 1,
      "secondary-index-count": 5,
      "udf-count": 1,
      "key": "routine1/backup/1704110400000/source-ns1",
      "storage": {
        "s3-storage": {
          "bucket": "as-backup-bucket",
          "path": "backups",
          "s3-region": "eu-central-1"
        }
      },
      "compression": "ZSTD",
      "encryption": "NONE"
    }
  ]
}
```

For fields description see [fields description](docs/readme/dto/dto.backupdetails.md)

</details>

You can filter the results by adding query parameters:

[
`GET {{baseUrl}}/v1/backups/full/<name>?from=<from>&to=<to>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getFullBackups)

Here, `name` is the routine name, `from` and `to` are timestamps in milliseconds since epoch.

#### Disable Routine

ℹ️ *Available since v3.0*

[
`POST {{baseUrl}}/v1/routines/<routineName>/disable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/disableRoutine)

[
`POST {{baseUrl}}/v1/routines/<routineName>/enable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/enableRoutine)

Set the disabled flag for the given routine to `true` or `false` (default is `false`).

- Disabled routines will not schedule new jobs.
- Running jobs will be canceled, similar to the `Cancel Backup Job` endpoint.

### Restore

#### Direct restore using a specific backup

ℹ️ *Available since v1.0*

This request restores a backup from a specified path to a designated destination.

[
`POST {{baseUrl}}/v1/restore/full`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreFull)

<details>
    <summary>Request body</summary>

<!-- RestoreFullRequest -->

```json
{
  "destination": {
    "seed-nodes": [
      {
        "host-name": "host.docker.internal",
        "port": 3000
      }
    ],
    "credentials": {
      "user": "user",
      "password": "password"
    }
  },
  "source": {
    "s3-storage": {
      "bucket": "as-backup-bucket",
      "path": "backups",
      "s3-region": "eu-central-1"
    }
  },
  "policy": {
    "no-generation": true
  },
  "backup-data-path": "routine1/backup/1704110400000/source-ns1"
}
```

The `no-generation` parameter allows overwriting of existing keys if set to `true`.

In the `source` section, `path` is the `key` value returned as a response in the [Full Backup List](#full-backup-list)
example. The `type` parameter under `source` denotes S3 storage if set to `1` and local storage if set to `0`.

The `destination` field says where to restore to. It can be any Aerospike cluster.

You can also use `destination-name` and `storage-name` instead of `destination` and `storage` respectively.
They refer to the names of the corresponding entities in the configuration file.

For more details see [fields description](docs/readme/dto/dto.restorerequest.md)

</details>

The response is a job ID.

#### Restore using routine name and timestamp

ℹ️ *Available since v1.0*

This option automatically restores data by identifying and applying the
appropriate backup sequence based on the specified timestamp.
For each namespace defined in the backup routine, the system locates the most recent full backup
prior to the given time and applies all incremental backups created after that full backup,
up to the target timestamp.

There is no need to specify individual backup paths or storage locations — the system handles this internally. The
restore process requires a full backup as a foundation; incremental backups cannot be used on their own.

By default, backups are applied in chronological order. However, when restoring to an empty namespace, the system may
reverse the order of application and use the `CREATE_ONLY` policy. This optimization ensures that each record is written
exactly once—applying only the latest version—thus reducing write load and generation noise. If needed, this
optimization can be disabled using the `disable-reordering` flag in the `RestoreTimestampRequest`.

Overall, the process is fully automated: users do not need to manually choose or arrange backups for the restore to
succeed. The restore process runs in parallel for every namespace.

**Example**

```text
Timeline ─────────────────────────────────────────────────────────────────────────────────────────▶

Backups:
   [Full A]──[Incr A1]──[Incr A2]──[Full B]──[Incr B1]──[Incr B2]──▶ T ◀──[Incr B3]──[Full C]──...
                                                                     ↑
                                                               Restore Point
```

What gets restored at T2:

* Full backup: `Full B`
* Incremental backups: `Incr B1`, `Incr B2`
* Excluded: `Incr B3` and anything after T2

Restore order (to empty namespace): `Incr B2`, `Incr B1`, `Full B`.

- Backups are applied in reverse order. This ensures that the most recent version of each record is restored first. Any
  earlier versions of the same record are skipped, by using `CREATE_ONLY` policy, reducing unnecessary writes.

Restore order (to non-empty namespace or with `disable-reordering`): `Full B`, `Incr B1`, `Incr B2`.

* Backups are applied in chronological order.
  All versions of each record are restored step by step.
  If a record was modified multiple times, each update is applied, with the final version appearing last.

[`
POST {{baseUrl}}/v1/restore/timestamp`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreTimestamp)

<details>
    <summary>Request body</summary>

```json
{
  "destination-name": "abs-cluster",
  "time": 1704110400000,
  "routine": "routine1"
}
```

Request is almost identical to [restore by path](#direct-restore-using-a-specific-backup), but instead of
`backup-data-path`
should provide a pair `time` and `routine`.

For more details see [fields description](docs/readme/dto/dto.restoretimestamprequest.md)
</details>

The response is a job ID.

#### Restore job status

ℹ️ *Available since v1.0*

You can get job status with the endpoint

[
`GET {{baseUrl}}/v1/restore/status/<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreStatus).

It works identical for both restore types.

<details>
    <summary>Response example</summary>
<!-- CurrentRestoreResponse -->

```json
{
  "read-records": 100000,
  "total-bytes": 30000000,
  "expired-records": 0,
  "skipped-records": 0,
  "ignored-records": 0,
  "inserted-records": 50000,
  "existed-records": 0,
  "fresher-records": 0,
  "index-count": 4,
  "udf-count": 1,
  "errors-in-doubt": 0,
  "current-job": {
    "total-records": 100000,
    "done-records": 50000,
    "start-time": "2024-01-01T12:00:00Z",
    "percentage-done": 50,
    "estimated-end-time": "2024-01-01T13:00:00Z",
    "metrics": {
      "records-per-second": 1000,
      "kilobytes-per-second": 30000,
      "pipeline": 0
    }
  },
  "status": "Running"
}
```

For fields description see [fields description](docs/readme/dto/dto.restorejobstatus.md)
</details>

#### Retrieve Restore Jobs

ℹ️ *Available since v3.2*

Provides a list of all restore jobs, with optional filtering by time range and status.

[
`GET {{baseUrl}}/v1/restore/jobs?from=<from>&to=<to>&status=<status>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/retrieveRestoreJobs)

- `from` (optional): Lower bound timestamp filter in milliseconds since epoch.
- `to` (optional): Upper bound timestamp filter in milliseconds since epoch.
- `status` (optional): Comma-separated status filter (e.g., `Running,Done,Failed,Cancelled`). Use `!` prefix to exclude
  statuses (e.g., `!Failed,Cancelled`).

<details>
    <summary>Response example</summary>

<!-- CurrentRestoresResponse -->

```json
{
  "12345678": {
    "read-records": 100000,
    "total-bytes": 30000000,
    "expired-records": 0,
    "skipped-records": 0,
    "ignored-records": 0,
    "inserted-records": 50000,
    "existed-records": 0,
    "fresher-records": 0,
    "index-count": 4,
    "udf-count": 1,
    "errors-in-doubt": 0,
    "current-job": {
      "total-records": 100000,
      "done-records": 50000,
      "start-time": "2024-01-01T12:00:00Z",
      "percentage-done": 50,
      "estimated-end-time": "2024-01-01T13:00:00Z",
      "metrics": {
        "records-per-second": 1000,
        "kilobytes-per-second": 30000,
        "pipeline": 0
      }
    },
    "status": "Running"
  }
}
```

</details>

#### Cancel Restore Job

ℹ️ *Available since v3.0*

Cancel the restore job identified by `<jobId>`. Data that has already been restored will remain intact.

[
`POST {{baseUrl}}/v1/restore/cancel/<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/cancelRestore)

# FAQ

## What happens when a backup doesn’t finish before another starts (for the same routine)?

- **Full Backups:**
    - Full backups cannot overlap. If a scheduled full backup is due to start but the previous one is still running, the
      new backup is skipped entirely. It is not queued but will wait for the next scheduled execution.
    - Full backups always take priority over incremental backups. If an incremental backup is running when a full backup
      is scheduled, the full backup will start as planned, and the incremental backup will continue running without
      interruption.

- **Incremental Backups:**
    - By default, incremental backups are skipped if any other backup (full or incremental) is still running.
      This behavior can be overridden using the [
      `backup-policy.concurrent-incremental`](#concurrent-incremental-backups) setting, which allows incremental backups
      to run concurrently.
    - Incremental backups will not run until at least one full backup has been successfully completed.

## Can multiple backup routines be performed simultaneously?

Yes, multiple backup routines can run in parallel. Furthermore, it is possible to back up different namespaces from the
same cluster using separate routines with different schedules, all running simultaneously.

To manage resource utilization, you can configure the `cluster.max-parallel-scans` property to limit the number of read
threads operating on a single cluster.

## How does the backup service identify what data to back up during incremental backups?

The Aerospike Backup Service uses Aerospike’s scan operation to identify and backup records,
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
          or old version—there’s no guarantee which state will be captured.
        - **Sealed Backups**: When the sealed property in the backup policy is set to true, the backup service will only
          include records modified before the backup start time. While this creates a more precise point-in-time
          snapshot, there’s still unpredictability: if a record is updated during the backup process, it might be
          captured in its old state or excluded entirely from the backup.

Users should select the appropriate approach based on their recovery point objectives and consistency requirements. The
default open-ended approach ensures better data coverage but with some state unpredictability, while sealed backups
provide better point-in-time consistency but might miss records updated during the backup process.

## Which storage providers are supported?

The backup service supports the following storage providers:

- **AWS S3** (or compatible services such as MinIO)
- **Microsoft Azure**
- **Google Cloud Storage**
- **Local storage** (files stored on the same machine where the backup service is running)

# Build from source

### Prerequisites

- Go 1.23

### Build the service

The following command generates a binary under the `build/target` directory.

```bash
make build
```

### Build Docker image

#### Multiplatform

```bash
DOCKER_USERNAME="<jforg-username>" DOCKER_PASSWORD="<jfrog-password>" IMAGE_TAG="<tag>" make docker-buildx 
```

#### For local use

```bash
IMAGE_TAG="<tag>" make docker-build
```

### Build Linux packages

Run `make packages`.
This will generate a `rpm/deb` package for supported platforms (`linux/amd64`,`linux/arm64`) with respective `sha256`
checksum file in the `build/target` directory.
See the quick [guide](build/package/README.md) on how to get started with the Linux packages.

### Release

Use the following commands before a release to update the version.

```bash
NEXT_VERSION="<version>"  make release
NEXT_HELM_CHART_VERSION="<helm-chart-version>" make helm-chart-release
git add --all
git commit -m "Release: "$(cat VERSION)""
git tag "$(cat VERSION)"
git push 
```

# Migration Guide

## v3.2 -> v3.3

This release includes minor breaking changes and numerous fixes to improve the
reliability of backup and restore routines. Notable fixes address:

* Backup reader now ensures that no more than the specified number of concurrent threads are scanning simultaneously.
* Backup retention policy application; Retention is on pause during restore.
* Handling of missing routines, full backup counters, and race conditions in config application.
* Improved error handling and logging.
* Bandwidth limiter update for more predictable throughput.
* Dependencies updated to incorporate the latest security fixes.

#### Breaking Changes

- TLS configuration validation is now stricter. The service will reject incomplete or inconsistent TLS settings at
  startup. Some configurations that were previously accepted (but not fully valid) may now fail validation. Please
  review your TLS settings to ensure that all required certificate, key, and CA parameters are provided correctly.

- Storage config removed from `RestoreTimestampRequest`.
  Storage details are read from the routine. Because of strict validation, clients must not include storage
  configuration in restore requests anymore.

#### New Features

- Support Aerospike 8.1

- Independent tuning of read and write parallelism.
  New field `parallel-write` can now be configured separately in [backup policy](docs/readme/dto/dto.backuppolicy.md)
  giving operators finer control over performance. By default it is equal to `parallel-read`.

## v3.1 -> v3.2

This release introduces minor breaking changes and new features.
It is focused on stability and bug fixes, and includes an updated, faster version of the underlying backup library.

#### Breaking Changes

- The `namespaces` field in a [backup routine](docs/readme/dto/dto.backuproutine.md) entry is now mandatory.
  Previously, omitting this field would default to backing up all namespaces.
  To achieve the same behavior now, you must explicitly provide an empty list (`namespaces: []`).
  This change prevents accidental backups of all namespaces if the field is forgotten.

- The `bandwidth` field in the [restore policy](docs/readme/dto/dto.restorepolicy.md) is now specified in MiB/s instead
  of bytes per second. The minimum value for this property is 8 MiB/s.
  This aligns its unit with the equivalent property in the [backup policy](docs/readme/dto/dto.backuppolicy.md) and
  other Aerospike tools.

#### New Features

- **Restore Jobs Endpoint**: A new endpoint [`GET /v1/restore/jobs`](#retrieve-restore-jobs)
  has been added to retrieve a list of all restore jobs, with options to filter by time range and status.
- **Add min-part-size to Azure and GCP**:
  The `min-part-size` property, previously available only for S3 storage,
  is now supported for both [Azure](docs/readme/dto/dto.azurestorage.md) and [GCP](docs/readme/dto/dto.gcpstorage.md)
  storage. This property allows you to configure the minimum size of individual upload chunks, which can help optimize
  performance for large backups.
- **Removed Root Permissions Requirement**: The backup service no longer requires root permissions to run.
- **Mask Sensitive Data**: The service now automatically masks private keys in the logs, to prevent accidental exposure.

#### Prometheus Metrics Update

New metrics have been added to provide more detailed monitoring of backup and restore operations:

- `aerospike_backup_service_backup_events_total`: A counter for backup events, labeled by routine, type, and outcome.
- `aerospike_backup_service_backup_duration_seconds`: A histogram of backup durations.
- `aerospike_backup_service_last_successful_backup_timestamp`: A gauge for the timestamp of the last successful backup.
- `aerospike_backup_service_restore_in_progress`: A counter for the number of restore processes currently running.

The `aerospike_backup_service_restore_progress_pct` metric has been removed as it created a new time series for each
restore job, leading to high cardinality issues in Prometheus. Restore progress can now be monitored via the
[`/v1/restore/status/{jobId}`](#restore-job-status) endpoint.

See [monitoring](#Monitoring) section for details.

## v3 -> v3.1

There is no breaking change in this release, but several new features have been introduced.

#### Strict validation

The backup service performs strict validation of the configuration file during startup.
Invalid configurations will result in errors preventing the service from starting. This will help prevent unexpected
behavior caused by invalid configurations.

#### Filter by partition

backup policy has new flag `partition-list`. When enabled, the backup service will back up only specific partitions of
the
cluster.
The format supports individual partitions or ranges.

- A range is specified as "<start>,<count>" (e.g., "100,50" backs up 50 partitions starting from 100).
- A single partition is specified as a number (e.g., "0").
- Multiple entries can be comma-separated: e.g., "0,100,200,300,400,500".

By default, all partitions (0 to 4095) are backed up.
This field is mutually exclusive with node-list.

This feature can be used to parallelize backups across multiple instances. Each instance can be assigned a subset of
partitions to back up, ensuring efficient distribution of workload.

#### Support for specifying object storage classes

Every storage provider now supports specifying an object storage class

* `STANDARD`, `GLACIER`, `DEEP_ARCHIVE`etc for S3
* `Hot`, `Cool`, `Cold`, `Archive` for Azure Blob Storage
* `Standard`, `Nearline`, `Coldline`, `Archive` for Google Cloud Storage

Object storage classes define the durability and availability levels for objects within a bucket.
Supported values depend on the chosen storage provider.

#### Concurrent incremental backups

Backup policy has new flag `concurrent-incremental`.

* When false (default), incremental backups are skipped if another backup for same routine is in progress.

* When true, incremental backups can run concurrently with full backups for the same routine.

#### Skip cluster config backup

Backup policy has new field `with-cluster-configuration`.
When false (default), the backup service will exclude cluster configuration from the backup.

## v2 -> v3

#### Storage Object

The `Storage` object schema has been updated in **v3** to improve clarity, modularity, and support for additional
storage types.

- **v2:** Unified schema with a `type` field to differentiate storage types.
- **v3:** Separate schemas for each storage type:
    - `local-storage`
    - `s3-storage`
    - `azure-storage`
    - `gcp-storage`
- Validation ensures **only one storage type** is configured per `dto.Storage`.

**S3 Path Construction**:

- **v2**: S3 paths were constructed as `s3://<bucket>/<path>`.
- **v3**: `bucket` and `path` are now separate fields in `dto.S3Storage`.

Example:

<!-- Storage -->

```yaml
aws-s3:
  s3-storage:
    bucket: as-backup-bucket
    path: backups
    s3-region: eu-central-1
azure-blob-storage:
  azure-storage:
    endpoint: http://127.0.0.1:6000/devstoreaccount1
    container-name: testcontainer
    path: backups
    account-name: devstoreaccount1
    account-key: Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==
gcp-gcs:
  gcp-storage:
    key-file-path: key-file.json
    bucket-name: gcp-backup-bucket
    path: backups
    endpoint: http://127.0.0.1:9020
local:
  local-storage:
    path: backups

```

#### Configuration Management Update

Changes to the configuration API take effect immediately in version 3.0.

Configuration changes in versions prior to 3.0 required an explicit "apply" step after CRUD operations to update the
runtime configuration.

**Key Changes**

- **Config Updates:** Each CRUD update now automatically saves the configuration to the file and applies it to the
  runtime system. No need for a separate "apply" operation.
  The memory config is always in sync with the runtime.
- **Validation:** Invalid configurations will be rejected immediately, not applied and not saved.
- **The running backup processes:** will finish as they are, but:
    - If a routine entry is absent in the updated configuration file, it will not be rescheduled.
    - If the routine entry is updated, it will be rescheduled with the new parameters.

#### Apply Endpoint

The `apply` endpoint reads and applies the configuration from the file (after it was modified externally).

#### Secret Agents

The `secret-agent` configuration field to store the list of secret agents is now named `secret-agents`.

#### Restore Request

In the new version (v3) of the API, the **`restore`** request (`/v1/restore/full` and `/v1/restore/incremental`)
was changed to simplify and streamline the process.

- **v2:** The `Storage` object contained a `path` that was reused as the backup data location.
- **v3:** The `path` in the `Storage` object now only refers to the **root path** of the storage.
  The specific backup data location is now specified using a new required field: **`backup-data-path`**.
  This change allows you to reuse the same storage for different restore requests.

#### Backup Retention Policy

This release introduces a new, configurable **RetentionPolicy** for managing backup storage more effectively.
The feature allows users to specify retention rules for both full and incremental backups, replacing the previous
options `KeepAll`, `RemoveAll`, and `RemoveIncremental`.
Retention policy is an optional part of a backup policy. It consists of two integer fields:

* `full`: The total number of full backups to retain. If not specified, all full backups are kept. The minimum is 1,
  meaning each new full backup deletes the previous one.
* `incremental`: The number of most recent full backups that also retain incremental backups made between them. Cannot
  exceed the value of `full`. If omitted, all incremental backups are kept. A value of `0` means that all previous
  existing incremental backups will be deleted after each full backup is made.

If no retention policy is specified, the system defaults to retaining all full and incremental backups, the same as the
`KeepAll` value in older versions.

After each successfull full backup, all existing backups are scanned to count full and incremental backups.
ABS then removes older full backups and their associated incremental backups as needed to retain only
the last `full` backups and incremental backups for the most recent `incremental` backups.

#### Node list

Backup routine has a new optional `node-list` property.

Node list is a comma-separated list of IP addresses and/or host names followed by port numbers.

```text
<IP addr 1>:<port 1>[,<IP addr 2>:<port 2>[,...]]
<IP addr 1>:<TLS_NAME 1>:<port 1>[,<IP addr 2>:<TLS_NAME 2>:<port 2>[,...]]
```

Back up the given cluster nodes only.
This argument is mutually exclusive to partition-list/after-digest arguments.
Default: back up all nodes in the cluster

##### Extra ttl

A new optional field, `extra-ttl`, has been added to the restore policy configuration.
It specifies the amount of extra time-to-live (TTL) to add to records that have expirable void-times.

#### Secret Agent for cluster

The credential object has a new optional `secret-agent` property that points to a secret agent, one of those listed in
the `secret-agents` configuration parameter.
Secret agent is responsible for storing secrets like passwords and TLS certificates.
`The password` field can contain either the password itself (recommended only for testing), or a path pointing to the
password in the Aerospike Secret Agent (starts with `secrets:`).

The fields `password` and `password-path` are mutually exclusive.

```yaml
dto.Credentials:
  description: Credentials represents authentication details to the Aerospike cluster.
  properties:
    auth-mode:
      description: "The authentication mode string (INTERNAL, EXTERNAL, PKI)."
      enum:
        - INTERNAL
        - EXTERNAL
        - PKI
      type: string
    password:
      description: |-
        The password for the cluster authentication.
        It can be either plain text or path into the secret agent.
      example: testPswd
      type: string
    password-path:
      description: The file path with the password string.
      example: /path/to/pass.txt
      type: string
    secret-agent:
      allOf:
        - $ref: '#/components/schemas/dto.SecretAgent'
      description: |-
        Secret Agent configuration (optional).
        Mutually exclusive with secret-agent-name.
      type: object
    secret-agent-name:
      description: |-
        Secret Agent configuration (optional). Link to one of preconfigured agents.
        Mutually exclusive with secret-agent.
      type: string
    user:
      description: The username for the cluster authentication.
      example: testUser
      type: string
  type: object
```
