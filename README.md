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

## Table of contents

- [Getting Started](#getting-started)
- [User Guide](#user-guide)
- [Build From Source](#build-from-source)
- [Usage](#usage)
- [Monitoring](#monitoring)
- [FAQ](#faq)

## Getting started

Aerospike Backup Service reads configurations from a YAML file provided when the service is launched. See [Run](#run)
for specific syntax.
A sample configuration file and docker-compose script will help you get started testing the service.
Follow the [docker-compose instructions](./docker-compose) to set up your test environment.

Linux installation packages are available
under [releases](https://github.com/aerospike/aerospike-backup-service/releases).

## User guide

### Entities

Each entity defined in the API specification has endpoints for reading and writing backup configurations at general or
granular levels.

For specifics and example values, see the [OpenAPI docs](https://aerospike.github.io/aerospike-backup-service/).

#### Configuration

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

### Operations

- List backups: Returns the details of available backups. A time filter can be added to the request.
- Restore from path: Starts a restore operation from a specified backup folder.
- Restore from a timestamp: Given a routine name, searches for the closest full backup to the given timestamp and
  applies the backup in the following order: full backup first, then incremental backups up to the given point in time,
  if they exist.

## Usage

### Service help

```
% ./backup -h
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

### Run

Run as a binary using a configuration file:

```bash
./build/target/aerospike-backup-service -c config/config.yml
```

Run in a container with a custom configuration file:

```bash
docker run -d -p 8080:8080 -v custom_config.yml:/app/config.yml --name backup-service backup-service
```

Example configuration files can be found in the [config](./config/) folder.

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

## Build from source

### Prerequisites

- Go 1.22

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

## FAQ

### What happens when a backup doesn’t finish before another starts (for the same routine)?

- **Full Backups:**
    - Full backups cannot overlap. If a scheduled full backup is due to start but the previous one is still running, the
      new backup is skipped entirely. It is not queued but will wait for the next scheduled execution.
    - Full backups always take priority over incremental backups. If an incremental backup is running when a full backup
      is scheduled, the full backup will start as planned, and the incremental backup will continue running without
      interruption.

- **Incremental Backups:**
    - Incremental backups are skipped if any other backup (full or incremental) is still running.
    - Incremental backups will not run until at least one full backup has been successfully completed.

### Can multiple backup routines be performed simultaneously?

Yes, multiple backup routines can run in parallel. Furthermore, it is possible to back up different namespaces from the
same cluster using separate routines with different schedules, all running simultaneously.

To manage resource utilization, you can configure the `cluster.max-parallel-scans` property to limit the number of read
threads operating on a single cluster.

### Which storage providers are supported?

The backup service supports the following storage providers:

- **AWS S3** (or compatible services such as MinIO)
- **Microsoft Azure**
- **Google Cloud Storage**
- **Local storage** (files stored on the same machine where the backup service is running)

## Example requests and responses

### Read configurations

This section details how to fetch configurations for clusters, policies, and storage options. This is useful for setting
up or verifying the configuration of your system.

#### Get cluster configuration

This endpoint returns the configurations of existing clusters, including the default cluster setup with seed nodes and
credentials.

Request:

```http
GET {{baseUrl}}/v1/config/clusters
```

<details>
    <summary>Response:</summary>

<!-- ClustersResponse -->

```json
[
  {
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
  }
]
```

</details>

#### Get routine configuration

Retrieves the configured backup routines.

Request:

```http
GET {{baseUrl}}/v1/config/routines
```

<details>
    <summary>Response:</summary>

<!-- RoutinesResponse -->

```json
{
  "routine1": {
    "backup-policy": "keepFilesPolicy",
    "source-cluster": "absDefaultCluster",
    "storage": "local",
    "interval-cron": "@yearly",
    "namespaces": [
      "test-namespace"
    ]
  },
  "routine2": {
    "backup-policy": "removeFilesPolicy",
    "source-cluster": "absDefaultCluster",
    "storage": "local",
    "interval-cron": "@monthly",
    "incr-interval-cron": "@daily",
    "namespaces": [
      "test-namespace"
    ],
    "set-list": [
      "backupSet"
    ],
    "bin-list": [
      "backupBin"
    ]
  }
}
```

</details>

#### Get storage configuration

Returns all the configured storage endpoints, including, if applicable, cloud storage endpoint information such as
region and path.

Request:

```http
GET {{baseUrl}}/v1/config/storage
```

<details>
    <summary>Response:</summary>

<!-- StorageResponse -->

```json
{
  "aws-s3": {
    "s3-storage": {
      "bucket": "as-backup-bucket",
      "path": "backups",
      "s3-region": "eu-central-1"
    }
  },
  "azure-blob-storage": {
    "azure-storage": {
      "endpoint": "http://127.0.0.1:6000/devstoreaccount1",
      "container-name": "testcontainer",
      "path": "backups",
      "account-name": "devstoreaccount1",
      "account-key": "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
    }
  },
  "gcp-gcs": {
    "gcp-storage": {
      "key-file-path": "key-file.json",
      "bucket-name": "gcp-backup-bucket",
      "path": "backups",
      "endpoint": "http://127.0.0.1:9020"
    }
  },
  "local": {
    "local-storage": {
      "path": "backups"
    }
  }
}
```

</details>

### Retrieve backup list

#### Full backup list

Provides a list of backups for each configured routine, including details such as creation time, namespace, and storage
location.

Request:

```http
GET {{baseUrl}}/v1/backups/full
```

<details>
    <summary>Response:</summary>

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

### Restore backup (direct restoration)

#### Direct restore using a specific backup

Destination field says where to restore to. It can be one of the clusters we read in 1st section, or any other Aerospike
cluster.

This request restores a backup from a specified path to a designated destination.
The `no-generation` parameter allows overwriting of existing keys if set to `true`.

In the `source` section, `path` is the `key` value returned as a response in the [Full Backup List](#full-backup-list)
example. The `type` parameter under `source` denotes S3 storage if set to `1` and local storage if set to `0`.

Request:

```http
POST {{baseUrl}}/v1/restore/full
```

<details>
    <summary>Request body:</summary>

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

</details>

The response is a job ID. You can get job status with the
endpoint [
`GET {{baseUrl}}/v1/restore/status/:<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreStatus).

Response:

```json
123456789
```

#### Restore using routine name and timestamp

This option restores the most recent full backup for the given timestamp and then applies all subsequent incremental
backups up to that timestamp.
In this example, the `destination` and `policy` fields are the same as in the previous example.

Request:

```http
POST {{baseUrl}}/v1/restore/timestamp
```

<details>
    <summary>Response:</summary>

<!-- RestoreTimestampRequest -->

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
  "policy": {
    "no-generation": true
  },
  "time": 1704110400000,
  "routine": "routine1"
}
```

</details>

The response is a job ID. You can get job status with the
endpoint [
`GET {{baseUrl}}/v1/restore/status/:<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreStatus).

Response:

```json
123456789
```

## Breaking API Changes (v2 → v3):

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

### **Example**

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

#### Key Changes

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

## New API functions (v2 → v3):

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

### Extra ttl

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

### Cancel Restore Job

New endpoint:

- [
  `POST {{baseUrl}}/v1/restore/cancel/:<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/cancelRestore)

Cancel the restore job identified by `<jobId>`. Data that has already been restored will remain intact.

### Cancel Backup Job

New endpoint:

- [
  `POST {{baseUrl}}/v1/backups/cancel/:<routineName>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/cancelCurrentBackup)

Cancel all currently running backups (both full and incremental) for the specified routine. Partially created backups
will be deleted.

### Disable Routine

New endpoints:

- [
  `POST {{baseUrl}}/v1/routines/:<routineName>/disable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/disableRoutine)
- [
  `POST {{baseUrl}}/v1/routines/:<routineName>/enable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/enableRoutine)

Set the disabled flag for the given routine to `true` or `false` (default is `false`).

- Disabled routines will not schedule new jobs.
- Running jobs will be canceled, similar to the `Cancel Backup Job` endpoint.