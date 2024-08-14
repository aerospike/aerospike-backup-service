# Aerospike Backup Service

[![Build](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml/badge.svg)](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml)
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

The endpoints defined within the configuration section permit the user to view or modify the configuration file.
Endpoints ending in `/config` permit reading and changing the entire file at once, while `/config/cluster` endpoints
enable more granular changes.

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

:warning: ABS currently supports only AWS S3 cloud storage.

#### Backup policy

A backup policy is a set of rules that define how backups should be performed. It could include information about a
backup schedule, criteria for what data is being backed up, and the storage destination.
See [`GET: /config/policies`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readPolicies) for
full details about what parameters are available to customize a backup policy.

You can save multiple policies with different configurations.
When you run
the [`POST: /config/policies`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/addPolicy) command
to create a policy, ensure that you give your policy a name that will let you quickly identify its characteristics.

Aerospike recommends defining at least one backup and restore policy.

#### Backup routine

A backup routine is a set of procedures that actually perform backups based on the predefined backup policy.
Routines are individually named just as policies are.
See the [Routines](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readRoutines) section for
command examples showing how to find all routines, get information about a specific named routine, and add, remove, or
update an existing routine.

:warning: Incremental backups are deleted if they are empty and after each full backup. System metadata is backed up
only on full backups.

### Operations

- List backups: Returns the details of available backups. A time filter can be added to the request.
- Restore from a file: Starts a restore operation from a specified backup file/folder.
- Restore from a timestamp: Given a routine name, searches for the closest full backup to the given timestamp and
  applies the backup in the following order: full backup first, then incremental backups up to the given point in time,
  if they exist.

## Usage

### Service help

```
% ./backup -h
Aerospike Backup Service

Usage:
  Use the following properties for service configuration [flags]

Flags:
  -c, --config string   configuration file path/URL
  -h, --help            help for Use
  -r, --remote          use remote config file
  -v, --version         version for Use
```

Set the configuration file path with `-c`.

Without the `-r` flag, the file specified after `-c` is the actual configuration file.
With the `-r` flag, the file specified after `-c` contains the path or URL to the actual configuration file. 

For example, you may store your configurations remotely, such as on AWS S3 storage. 
In this case, you could have a remote_config.yaml file containing S3 details, and you would run the server with `-c remote_config.yaml -r`.


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
| `aerospike_backup_service_runs_total`                  | Full backup runs counter                    |
| `aerospike_backup_service_incremental_runs_total`      | Incremental backup runs counter             |
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
This will generate a `rpm/deb` package for supported platforms (`linux/amd64`,`linux/arm64`) with respective `sha256` checksum file in the `build/target` directory.
See the quick [guide](build/package/README.md) on how to get started with the Linux packages.

### Release
Use the following commands before a release to update the version.
```bash
NEXT_VERSION="<version>" make release
git add --all
git commit -m "Release: "$(cat VERSION)""
git tag "$(cat VERSION)"
git push 
```

## FAQ

### What happens when a backup doesn’t finish before another starts?

The service will skip the next startup until the previous backup run is completed.

### Can multiple backup routines be performed simultaneously?

The service uses the [asbackup](https://github.com/aerospike/aerospike-tools-backup) shared library, which is not
currently thread safe.
Given this limitation, backup routines are performed in sequence.

### Which storage providers are supported?

The backup service supports AWS S3 or compatible (such as MinIO) and local storage.

## Known Issues

* The service may crash if an invalid S3 backup key is provided during configuration.

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

```json
{
  "absDefaultCluster": {
    "seed-nodes": [
      {
        "host-name": "host.docker.internal",
        "port": 3000
      }
    ],
    "credentials": {
      "user": "tester",
      "password": "psw"
    }
  }
}
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

```json
{
  "routine1": {
    "backup-policy": "keepFilesPolicy",
    "source-cluster": "absDefaultCluster",
    "storage": "local",
    "interval-cron": "@yearly",
    "namespaces": ["source-ns7"]
  },
  "routine2": {
    "backup-policy": "removeFilesPolicy",
    "source-cluster": "absDefaultCluster",
    "storage": "local",
    "interval-cron": "@yearly",
    "namespaces": ["source-ns8"],
    "set-list": ["backupSet"],
    "bin-list": ["backupBin"]
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

```json
{
  "local": {
    "type": 0,
    "path": "./localStorage"
  },
  "minio": {
    "type": 1,
    "path": "s3://as-backup-bucket/storage1",
    "s3-region": "eu-central-1",
    "s3-profile": "minio",
    "s3-endpoint-override": "http://host.docker.internal:9000"
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

```json
{
  "routine1": [
    {
      "created": "2024-03-14T13:13:28.96962301Z",
      "from": "0001-01-01T00:00:00Z",
      "namespace": "source-ns7",
      "byte-count": 48,
      "file-count": 1,
      "Key": "s3://as-backup-bucket/storage1/minio/backup/1710422008983/source-ns4"
    }
  ],
  "routine2": [
    {
      "created": "2024-03-14T13:13:29.105744927Z",
      "from": "0001-01-01T00:00:00Z",
      "namespace": "source-ns8",
      "byte-count": 48,
      "file-count": 1,
      "key": "localStorage/filterBySetAndBin/backup/source-ns8"
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

Request body:

```json
{
  "destination": {
    "seed-nodes": [
      {
        "host-name": "localhost",
        "port": 3000
      }
    ],
    "credentials": {
      "user": "tester",
      "password": "psw"
    }
  },
  "policy": {
    "no-generation": "true"
  },
  "source": {
    "path": "s3://as-backup-bucket/storage1/minio/backup/1710422008983/source-ns4",
    "type": 1,
    "s3-region": "eu-central-1"
  }
}
```

The response is a job ID. You can get job status with the
endpoint [`GET {{baseUrl}}/v1/restore/status/:<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreStatus).

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

Request body:

```json
{
  "destination": {
    "seed-nodes": [
      {
        "host-name": "localhost",
        "port": 3000
      }
    ],
    "credentials": {
      "user": "tester",
      "password": "psw"
    }
  },
  "policy": {
    "no-generation": "true"
  },
  "routine": "routine1",
  "time": "1710671632452"
}
```

The response is a job ID. You can get job status with the
endpoint [`GET {{baseUrl}}/v1/restore/status/:<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreStatus).

Response:

```json
123456789
```
