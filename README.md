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
    + [Configuration with API](#configuration-with-api)
  * [Monitoring](#monitoring)
  * [Example requests and responses](#example-requests-and-responses)
    + [Backup](#backup)
    + [Restore](#restore)
- [FAQ](#faq)
  * [What happens when a backup doesn’t finish before another starts (for the same routine)?](#what-happens-when-a-backup-doesnt-finish-before-another-starts-for-the-same-routine)
  * [Can multiple backup routines be performed simultaneously?](#can-multiple-backup-routines-be-performed-simultaneously)
  * [Which storage providers are supported?](#which-storage-providers-are-supported)
- [Build from source](#build-from-source)
    + [Prerequisites](#prerequisites)
    + [Build the service](#build-the-service)
    + [Build Docker image](#build-docker-image)
    + [Build Linux packages](#build-linux-packages)
    + [Release](#release)
- [Migration Guide](#migration-guide)
  * [v3 → v3.1](#v3-%E2%86%92-v31)
  * [v2 → v3](#v2-%E2%86%92-v3)
    + [Storage Object](#storage-object)
    + [Configuration Management Update](#configuration-management-update)
    + [Apply Endpoint](#apply-endpoint)
    + [Secret Agents](#secret-agents)
    + [Restore Request](#restore-request)
    + [Backup Retention Policy](#backup-retention-policy)
  * [v2 → v3](#v2-%E2%86%92-v3-1)
    + [Node list](#node-list)
    + [Secret Agent for cluster](#secret-agent-for-cluster)

<!-- tocstop -->

# Getting started

Aerospike Backup Service reads configurations from a YAML file provided when the service is launched. See [Run](#run)
for specific syntax.

Linux installation packages are available
under [releases](https://github.com/aerospike/aerospike-backup-service/releases).

# User guide

This section covers basic usage scenarios for the Aerospike Backup Service.

## Run

#### Binary

Run as a binary using a configuration file:

```bash
./aerospike-backup-service -c config.yml
```

Help:

```bash
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
In this case, you could have a remote_config.yaml file containing S3 details, and you would run the server with
`-c remote_config.yaml -r`.

#### Docker

Run in a container with a custom configuration file:

```bash
docker run -d -p 8080:8080 -v config.yml:/app/config.yml --name backup-service backup-service
```

#### Service

Run as service (default path for config is `/etc/aerospike-backup-service/aerospike-backup-service.yml`):

```bash
sudo systemctl start aerospike-backup-service
```

view service logs:

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

Configuration file example:

<!-- DefaultConfig -->

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/aerospike/aerospike-backup-service/refs/tags/v3.1.0/docs/config.schema.json
---
aerospike-clusters:
  abs-cluster: # <--- Custom cluster name
    seed-nodes:
      - host-name: "localhost"
        port: 3000
    credentials:
      user: "tester"
      password: "secret:asbackup:psw" # Password will be fetched from the secret agent
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
    incr-interval-cron: "0 */2 * * * *" # Incremental backups every 2 hours
    source-cluster: abs-cluster         # <--- Refers to the cluster name under aerospike-clusters
    storage: s3                         # <--- Refers to the storage name under storage
    backup-policy: dailyBackupPolicy    # <--- Refers to the policy name under backup-policies
```

Several configuration fields in the YAML file are marked with `May affect performance`.
These settings (such as parallel, file-limit, min-part-size, and compression)
can have a significant impact on backup throughput.
We recommend experimenting with different values in your environment to find the optimal balance.

### Configuration with API

Each entity defined in the API specification has endpoints for reading and writing backup configurations at general or
granular levels. While the API provides full control over the configuration, for most use cases,
it’s preferable to configure the service via the YAML configuration file, which is easier to maintain.

For specifics and example values, see the [OpenAPI docs](https://aerospike.github.io/aerospike-backup-service/).
The endpoints defined within the configuration section allow users to view or modify the configuration file.
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
You can get information about a specific configured storage option, for example to check the cloud storage location for
a backup.
You can also add, update, or remove a storage configuration. See
the [Storage](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readAllStorage) entities
under `/config/storage` for detailed information.

:warning: ABS currently supports AWS S3, GCP, Microsoft Azure cloud storage.

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
backups),
and the scope of data to back up (such as namespaces, sets, or bins).

See the [Routines](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readRoutines) section for
command examples showing how to find all routines, get information about a specific named routine, and add, remove, or
update an existing routine.

:warning: Incremental backups are deleted if they are empty and after each full backup. System metadata is backed up
only on full backups.

## Monitoring

The service exposes a wide variety of system metrics that [Prometheus](https://prometheus.io/) can scrape, including the
following application metrics:

| Name                                                   | Description                                 |
|--------------------------------------------------------|---------------------------------------------|
| `aerospike_backup_service_runs_total`                  | Successful backup runs counter              |
| `aerospike_backup_service_incremental_runs_total`      | Successful incremental backup runs counter  |
| `aerospike_backup_service_skip_total`                  | Full backup skip counter                    |
| `aerospike_backup_service_incremental_skip_total`      | Incremental backup skip counter             |
| `aerospike_backup_service_failure_total`               | Full backup failure counter                 |
| `aerospike_backup_service_incremental_failure_total`   | Incremental backup failure counter          |
| `aerospike_backup_service_duration_millis`             | Full backup duration in milliseconds        |
| `aerospike_backup_service_incremental_duration_millis` | Incremental backup duration in milliseconds |

* `/metrics` exposes metrics for Prometheus to check performance of the backup service.
  See [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/getting_started/) for instructions.
* `/health` allows monitoring systems to check the service health.
* `/ready` checks whether the service is able to handle requests.

See
the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
on liveness and readiness probes for more information.

The HTTP metrics endpoint can be found on
the [OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/) page.

## Example requests and responses

The following sections provide example requests and responses for various operations.
For full API documentation, refer to
the [Aerospike Backup Service OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/#/System/metrics).

While command-line tools such as curl, httpie, or wget work well for interacting with the API, a graphical
interface (such as [Postman](https://www.postman.com/downloads/) or [Insomnia](https://insomnia.rest/)) is generally
recommended for a more convenient and user-friendly experience.

### Backup

#### Trigger On-Demand Backup

This request starts the backup operation for the specified routine, regardless of its configured schedule.

[
`POST {{baseUrl}}/v1/backups/schedule/<routineName>?delay=<timeout>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/scheduleFullBackup)

* routineName: The name of the backup routine to trigger.
* delay (optional): Time in milliseconds to delay the start of the backup.

If the request is accepted, the server responds with Http 202 Accepted.

#### Get Current Backup

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

`estimated-end-time` an estimation, is calculated based on the current percentage done and duration.

`records-per-second` and `kilobytes-per-second` show current speed, they are updated every second.

`pipeline` represents the number of records that have been read from the source but not yet written to the
destination.
This metric helps identify bottlenecks:

- if Pipeline is zero or fluctuates near zero, it means the destination (storage) is consuming data faster than the
  source (Aerospike) can read.
- If Pipeline grows large, it indicates that the source is producing data faster than the destination can consume.

</details>

#### Cancel Backup Job

[
`POST {{baseUrl}}/v1/backups/cancel/<routineName>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/cancelCurrentBackup)

Cancel all currently running backups (both full and incremental) for the specified routine. Partially created backups
will be deleted.

#### Retrieve Backup List

Provides a list of backups for each configured routine, including details such as creation time, duration, namespace,
and storage
location.

[`GET {{baseUrl}}/v1/backups/full`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getFullBackups)

<details>
    <summary>Response</summary>

Response is a map of routine names to lists of backups.
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

</details>

It's possible to filter the results by adding query parameters:

[
`GET {{baseUrl}}/v1/backups/full/<name>?from=<from>&to=<to>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getFullBackups)

where `name` is the routine name, `from` and `to` are timestamps in milliseconds since epoch.

#### Disable Routine

[
`POST {{baseUrl}}/v1/routines/<routineName>/disable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/disableRoutine)

[
`POST {{baseUrl}}/v1/routines/<routineName>/enable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/enableRoutine)

Set the disabled flag for the given routine to `true` or `false` (default is `false`).

- Disabled routines will not schedule new jobs.
- Running jobs will be canceled, similar to the `Cancel Backup Job` endpoint.

### Restore

#### Direct restore using a specific backup

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

`destination` field says where to restore to. It can be any Aerospike cluster.

Alternatively, you can use `destination-name` and `storage-name` instead of `destination` and `storage` respectively.
They
refer to the names of the corresponding entities in the configuration file.

</details>

The response is a job ID.

#### Restore using routine name and timestamp

This option restores the most recent full backup for the given timestamp and then applies all subsequent incremental
backups up to that timestamp. You don't need to specify the exact backup path or storage.

[`
POST {{baseUrl}}/v1/restore/timestamp`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreTimestamp)

<details>
    <summary>Request</summary>

<!-- RestoreTimestampRequest -->

```json
{
  "destination-name": "abs-cluster",
  "time": 1704110400000,
  "routine": "routine1"
}
```

</details>

The response is a job ID.

#### Restore job status

You can get job status with the
endpoint

[
`GET {{baseUrl}}/v1/restore/status/<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreStatus).

<details>
    <summary>Request</summary>


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
</details>

#### Cancel Restore Job

Cancel the restore job identified by `<jobId>`. Data that has already been restored will remain intact.

- [
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
    - Incremental backups are skipped if any other backup (full or incremental) is still running.
    - Incremental backups will not run until at least one full backup has been successfully completed.

## Can multiple backup routines be performed simultaneously?

Yes, multiple backup routines can run in parallel. Furthermore, it is possible to back up different namespaces from the
same cluster using separate routines with different schedules, all running simultaneously.

To manage resource utilization, you can configure the `cluster.max-parallel-scans` property to limit the number of read
threads operating on a single cluster.

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
DOCKER_USERNAME="<jforg-username>" DOCKER_PASSWORD="<jfrog-password>" TAG="<tag>" make docker-buildx 
```

#### For local use

```bash
TAG="<tag>" make docker-build
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

## v3 → v3.1

## v2 → v3

### Storage Object

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

### Configuration Management Update

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

### Apply Endpoint

The `apply` endpoint reads and applies the configuration from the file (after it was modified externally).

### Secret Agents

The `secret-agent` configuration field to store the list of secret agents is now named `secret-agents`.

### Restore Request

In the new version (v3) of the API, the **`restore`** request (`/v1/restore/full` and `/v1/restore/incremental`)
was changed to simplify and streamline the process.

- **v2:** The `Storage` object contained a `path` that was reused as the backup data location.
- **v3:** The `path` in the `Storage` object now only refers to the **root path** of the storage.
  The specific backup data location is now specified using a new required field: **`backup-data-path`**.
  This change allows you to reuse the same storage for different restore requests.

### Backup Retention Policy

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

## v2 → v3

### Node list

Backup routine has a new optional `node-list` property.

Node list is a comma-separated list of IP addresses and/or host names followed by port numbers.

```text
<IP addr 1>:<port 1>[,<IP addr 2>:<port 2>[,...]]
<IP addr 1>:<TLS_NAME 1>:<port 1>[,<IP addr 2>:<TLS_NAME 2>:<port 2>[,...]]
```

Back up the given cluster nodes only.
This argument is mutually exclusive to partition-list/after-digest arguments.
Default: back up all nodes in the cluster

#### Extra ttl

A new optional field, `extra-ttl`, has been added to the restore policy configuration.
It specifies the amount of extra time-to-live (TTL) to add to records that have expirable void-times.

### Secret Agent for cluster

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