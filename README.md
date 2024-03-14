# Aerospike Backup Service

[![Build](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml/badge.svg)](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml)
[![codecov](https://codecov.io/gh/aerospike/aerospike-backup-service/branch/main/graph/badge.svg)](https://codecov.io/gh/aerospike/aerospike-backup-service)

> [!NOTE]  
> Aerospike Backup Service is currently in Beta and not supported by Aerospike. Production usage is not recommended and changes may occur.
> Enterprise customers, please contact support to sign a Beta agreement. 

The Aerospike Backup Service provides a set of REST API endpoints to back up and restore a cluster. 
You can perform full and incremental backups and set different backup policies and schedules.
There are also several monitoring endpoints to check backup information.

Use the [OpenAPI generation script](./scripts/generate_OpenApi.sh) to generate an OpenAPI specification for the service.
A pre-built OpenAPI specification is available in Swagger format [here](https://aerospike.github.io/aerospike-backup-service/).

## Table of Contents

- [Getting Started](#getting-started)
- [User Guide](#user-guide)
- [Build From Source](#build-from-source)
- [Usage](#usage)
- [Monitoring](#monitoring)
- [FAQ](#faq)

## Getting Started
Aerospike Backup Service reads configurations from a YAML file provided when the service is launched. See [Run](#run) for specific syntax.
A sample configuration file and docker-compose script will help you get started testing the service.
Follow the [docker-compose instructions](./docker-compose) to set up your test environment.

Linux installation packages are available under [releases](https://github.com/aerospike/aerospike-backup-service/releases).

## User Guide

### Entities

Each entity defined in the API specification has endpoints for reading and writing backup configurations at general or granular levels.

For specifics and example values, see the [OpenAPI docs](https://aerospike.github.io/aerospike-backup-service/).

#### Cluster Connection
Cluster configuration entities denote the configuration properties needed to establish connections to Aerospike clusters.
These connections include the cluster IP address, port number, authentication information, and more.
See [`POST: /config/clusters`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/addCluster) for the full specification.

:warning: Use the [Aerospike Secret Agent](https://aerospike.com/docs/tools/backup#secret-agent-options) to avoid including secrets in your configuration.

#### Storage Connection
This entity includes properties of connections to local or cloud storage, where the backup files are stored.
You can get information about a specific configured storage option, for example to check the cloud storage location for a backup.
You can also add, update, or remove a storage configuration. See the [Storage](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readAllStorage) entities under `/config/storage` for detailed information.

:warning: ABS currently supports only AWS S3 cloud storage.

#### Backup Policy
A backup policy is a set of rules that define how backups should be performed. It could include information about a backup schedule, criteria for what data is being backed up, and the storage destination. See [`GET: /config/policies`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readPolicies) for full details about what parameters are available to customize a backup policy.

You can save multiple policies with different configurations.
When you run the [`POST: /config/policies`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/addPolicy) command to create a policy, ensure that you give your policy a name that will let you quickly identify its characteristics.

Aerospike recommends defining at least one backup and restore policy.

#### Backup Routine
A backup routine is a set of procedures that actually perform backups based on the predefined backup policy.
Routines are individually named just as policies are.
See the [Routines](https://aerospike.github.io/aerospike-backup-service/#/Configuration/readRoutines) section for command examples showing how to find all routines, get information about a specific named routine, and add, remove, or update an existing routine.

:warning: Incremental backups are deleted if they are empty and after each full backup. System metadata is backed up only on full backups.

### Operations

- List backups: Returns the details of available backups. A time filter can be added to the request.
- Restore from a file: Starts a restore operation from a specified backup file/folder.
- Restore from a timestamp: Given a routine name, searches for the closest full backup to the given timestamp and applies the backup in the following order: full backup first, then incremental backups up to the given point in time, if they exist.

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
  -v, --version         version for Use
```

### Run

Run as a binary using a configuration file:

```bash
./target/aerospike-backup-service -c config/config.yml
```

Run in a container with a custom configuration file:

```bash
docker run -d -p 8080:8080 -v custom_config.yml:/app/config.yml --name backup-service backup-service
```

Example configuration files can be found in the [config](./config/) folder.

## Monitoring

The service exposes a wide variety of system metrics that [Prometheus](https://prometheus.io/) can scrape, including the following application metrics:

| Name                                 | Description                                 |
|--------------------------------------|---------------------------------------------|
| `backup_runs_total`                  | Full backup runs counter                    |
| `backup_incremental_runs_total`      | Incremental backup runs counter             |
| `backup_skip_total`                  | Full backup skip counter                    |
| `backup_incremental_skip_total`      | Incremental backup skip counter             |
| `backup_failure_total`               | Full backup failure counter                 |
| `backup_incremental_failure_total`   | Incremental backup failure counter          |
| `backup_duration_millis`             | Full backup duration in milliseconds        |
| `backup_incremental_duration_millis` | Incremental backup duration in milliseconds |

* `/metrics` exposes metrics for Prometheus to check performance of the backup service. See [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/getting_started/) for instructions.
* `/health` allows monitoring systems to check the service health.
* `/ready` checks whether the service is able to handle requests.

See the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) on liveness and readiness probes for more information.

The HTTP metrics endpoint can be found on the [OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/) page.

## Build From Source

### Prerequisites

- Go 1.22

### Build the C shared libraries

```bash
make build-submodules
```

Read the official documentation for the [library build instructions](https://github.com/aerospike/aerospike-tools-backup#build-examples).

### Build the service

The following command generates a binary under the `target` directory.

```bash
make build
```

### Build Docker image

#### AMD64
```
docker build --build-arg GOARCH=amd64 -t backup-service .
```

#### ARM64
```
docker build --build-arg GOARCH=arm64 -t backup-service .
```

### Build Linux package

Run `make deb` or `make rpm` based on the desired package manager.
This will generate a package in the `target` directory.
See the quick [guide](./packages/) on how to get started with the Linux packages.

## FAQ

### What happens when a backup doesn’t finish before another starts?

The service will skip the next startup until the previous backup run is completed.

### Can multiple backup routines be performed simultaneously?

The service uses the [asbackup](https://github.com/aerospike/aerospike-tools-backup) shared library, which is not currently thread safe. 
Given this limitation, backup routines are performed in sequence. 

### Which storage providers are supported?

The backup service supports AWS S3 or compatible (such as MinIO) and local storage.
