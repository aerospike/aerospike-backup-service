# Aerospike Backup Service

[![Build](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml/badge.svg)](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aerospike/aerospike-backup-service)](https://pkg.go.dev/github.com/aerospike/aerospike-backup-service)
[![codecov](https://codecov.io/gh/aerospike/aerospike-backup-service/branch/main/graph/badge.svg)](https://codecov.io/gh/aerospike/aerospike-backup-service)

The Aerospike Backup Service provides a set of REST API endpoints to back up and restore a cluster.
You can perform full and incremental backups and set different backup policies and schedules.
There are also several monitoring endpoints to check backup information.

Use the [OpenAPI generation script](build/scripts/generate-openapi.sh) (or `make openapi`) to generate an OpenAPI specification for the service.
A pre-built OpenAPI specification is available in Swagger
format [here](https://aerospike.github.io/aerospike-backup-service/).

# Table of contents

<!-- toc -->

- [Features](#features)
- [Supported storage backends](#supported-storage-backends)
- [Quickstart](#quickstart)
- [Documentation](#documentation)
- [Migration Guide](#migration-guide)
    * [v3.5 -> v3.6](#v35---v36)
    * [v3.3 -> v3.4](#v33---v34)
    * [v3.2 -> v3.3](#v32---v33)
    * [v3.1 -> v3.2](#v31---v32)
    * [v3 -> v3.1](#v3---v31)
    * [v2 -> v3](#v2---v3)

<!-- tocstop -->

# Features

- Full and incremental backups with independent cron schedules, powered by [Quartz](https://github.com/reugn/go-quartz)
- Configurable backup policies: compression, encryption, bandwidth limiting, retention, rack-aware and
  partition-filtered backups
- Restore by explicit backup path or by routine name and timestamp, with automatic full/incremental sequencing
- A REST API for managing clusters, storage, policies, and routines, with changes applied immediately
- Prometheus metrics plus `/health` and `/ready` endpoints for monitoring and orchestration
- Runs as a binary, Docker container, systemd service, or Kubernetes workload (Helm chart included)

# Supported storage backends

- **AWS S3** (or compatible services such as MinIO)
- **Microsoft Azure**
- **Google Cloud Storage**
- **Local storage** (files stored on the same machine where the backup service is running)

# Quickstart

The fastest way to try Aerospike Backup Service locally is Docker Compose, which also starts an Aerospike cluster and
a MinIO container to store backups against:

```bash
git clone https://github.com/aerospike/aerospike-backup-service.git
cd aerospike-backup-service/build/docker-compose
docker compose -f docker-compose.yaml up -d
docker compose -f docker-compose.yaml ps --format "{{.Name}}: {{.Status}}"  # wait until all services report "healthy"
curl http://localhost:8080/health
```

See the [Docker Compose guide](build/docker-compose/README.md) for details, or the
[Installation guide](docs/installation.md) for running as a binary, a standalone Docker container, or a systemd
service.

Linux installation packages are available
under [releases](https://github.com/aerospike/aerospike-backup-service/releases).

# Documentation

- [Installation](docs/installation.md) — binary, Docker, systemd, and building from source
- [Configuration](docs/configuration.md) — configuration file format, scheduling, the configuration API, and FAQ
- [API examples](docs/api-examples.md) — example backup and restore requests and responses
- [Monitoring](docs/monitoring.md) — Prometheus metrics, alerts, and health/readiness endpoints
- [Linux packages](build/package/README.md) — installing and managing the DEB/RPM packages
- [Helm chart](helm/aerospike-backup-service) — deploying to Kubernetes
- [Changelog](CHANGELOG.md) — notable changes by release

# Migration Guide

## v3.5 -> v3.6

This release adds compact backups, more flexible restore-by-timestamp, performance optimizations in the record
reader/writer, and improved observability.

#### Breaking changes

- Restore job `status` values are now lowercase: `running`, `success`, `failure`, `canceled` (previously `Running`,
  `Done`, `Failed`, `Canceled`).
- Default log level changed from `DEBUG` to `INFO`. Set `logger.level: DEBUG` explicitly if you rely on verbose logging.
- Default cloud storage `min-part-size` increased from ~5 MB to 50 MB (S3, Azure, GCP).
- Backup scan `parallel` is now enforced per routine instead of per namespace. In routines with multiple namespaces,
  total concurrent scans across all namespaces are capped at `parallel` (previously each namespace could run up to
  `parallel` scans independently).

#### New features

- **Compact backups** — A `compact` flag in [backup policy](docs/readme/dto/dto.backuppolicy.md) skips base-64 encoding
  for BLOB types (Bytes, HLL, RawMap, RawList), producing smaller backup files.
- **Restore-by-timestamp overrides** — Optional `source` / `source-name` and `destination` / `destination-name` fields
  in [restore-by-timestamp requests](docs/readme/dto/dto.restoretimestamprequest.md). When both storage and destination
  are overridden, a routine does not need to be configured.
- **Restore-by-timestamp with `unique`** — Existing records remain unchanged; only new records are added from the backup.

#### Improvements

- Performance improvements in the record reader/writer.
- High-precision `aerospike_backup_service_backup_progress_pct` metric (float, no rounding); record-count estimate
  recalculates every 10 minutes during backup.
- New metrics: `aerospike_backup_service_backup_in_progress`, `aerospike_backup_service_restore_events_total`.


## v3.3 -> v3.4

This release delivers bug fixes and incremental improvements based on customer feedback. The primary addition is
rack‑aware backups.

#### Breaking changes

- The `prefer-racks` configuration field has moved from the [Backup routine](docs/readme/dto/dto.backuproutine.md) to
  the [Aerospike Cluster](docs/readme/dto/dto.aerospikecluster.md) configuration. Update your configuration accordingly.

#### New features

- Rack‑aware backup
    - A new `rack-list` field has been added to the [Backup routine](docs/readme/dto/dto.backuproutine.md).
    - It allows you to specify a list of racks to back up. Only nodes belonging to the specified racks will be scanned.
    - This field is mutually exclusive with `prefer-racks`, `node-list`, and `partition-list`.

- Human‑readable timestamps in backup paths
    - A new `timestamp-format` field is available in
      the [Backup common config](docs/readme/dto/dto.backupcommonconfig.md).
    - Allowed values:
        - ISO (e.g., `2006-01-02T15-04-05`)
        - EU (e.g., `02-Jan-2006-15-04-05`)
        - US (e.g., `Jan-02-2006-15-04-05`)
    - This field is optional. By default, ABS uses the Unix epoch timestamp to differentiate backups. When
      `timestamp-format` is set, a human‑readable timestamp is appended as a suffix to the epoch value. Functionality is
      unaffected.

- Scan policy configuration properties
    - The [Backup policy](docs/readme/dto/dto.backuppolicy.md) adds two optional fields:
        - `max-concurrent-nodes` — Maximum number of concurrent requests to server nodes. Default: issue requests to all
          nodes in parallel.
        - `use-scan-compression` — Enables built‑in compression during scan operations (Aerospike Server Enterprise
          Edition only).
    - Both fields are optional; they do not change functionality but may affect performance.

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

- **Restore Jobs Endpoint**: A new endpoint [`GET /v1/restore/jobs`](docs/api-examples.md#retrieve-restore-jobs)
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
- `aerospike_backup_service_restore_in_progress`: A gauge for the number of restore processes currently running.

The `aerospike_backup_service_restore_progress_pct` metric has been removed as it created a new time series for each
restore job, leading to high cardinality issues in Prometheus. Restore progress can now be monitored via the
[`/v1/restore/status/{jobId}`](docs/api-examples.md#restore-job-status) endpoint.

See the [monitoring guide](docs/monitoring.md#monitoring) for details.

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
