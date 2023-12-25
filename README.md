# Aerospike Backup Service

[![Build](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml/badge.svg)](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml)
[![codecov](https://codecov.io/gh/aerospike/aerospike-backup-service/branch/main/graph/badge.svg)](https://codecov.io/gh/aerospike/aerospike-backup-service)

The Aerospike backup-service provides a set of REST APIs to schedule full and incremental backups.
Additionally, these APIs can be used to restore data from a backup to a cluster.

Use the [OpenApi generation script](./scripts/generate_OpenApi.sh) to generate OpenAPI Specification for the service.
A pre-built OpenAPI Specification is available [here](https://aerospike.github.io/aerospike-backup-service/).

## Table of Contents

- [Getting Started](#getting-started)
- [User Guide](#user-guide)
- [Build From Source](#build-from-source)
- [Usage](#usage)
- [Monitoring](#monitoring)
- [FAQ](#faq)

## Getting Started

A sample configuration file and docker-compose script will help you get started testing the service.
Follow the [instructions](./docker-compose) to set up your test environment using docker-compose.

## User Guide

### Entities

A summary of each entity is included. For specifics and example values, see the [OpenAPI docs](https://aerospike.github.io/aerospike-backup-service/).

- Cluster Connection - Details about the cluster connection. Note you can use the [Secret Agent](https://aerospike.com/docs/tools/backup#secret-agent-options) to avoid including secrets in your configuration.
- Storage Connection - Details about storage connection, where the backup files will be stored.
- Backup Policy - Details about what backup settings to use for backups and restores. We recommend defining both a backup and restore policy.
- Backup Routine - How frequently to take backups. Notes: Incrementals are deleted if they are empty and after each full backup, and SMD data is backed up only on full backups.

### Operations

- List backups - Returns the details of available backups. A time filter can be added to the request.
- Restore from a file - Starts a restore operation from a specified backup file/folder.
- Restore from a timestamp - Given a routine name, searches for the closest full backup to the given timestamp and applies the backup in the following order: full backup first, then incremental backups up to the given point in time, if they exist.

## Build From Source

### Prerequisites

- Go 1.21

### Build the C shared libraries

```bash
make build-submodules
```

Read the official documentation for the [library build instructions](https://github.com/aerospike/aerospike-tools-backup#build-examples).

### Build the service

The following command will generate a binary under the `target` directory.

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

### Build installation package

Run `make deb` or `make rpm` based on the desired package manager.
This will generate a package in the `target` directory.

## Usage

### Service help

```
% ./backup -h
Aerospike Backup Service

Usage:
  Use the following properties for service configuration [flags]

Flags:
  -c, --config string       configuration file path/URL
  -h, --help                help for Use
  -f, --log-format string   log format (PLAIN, JSON) (default "PLAIN")
  -l, --log-level string    log level (default "DEBUG")
  -v, --version             version for Use
```

### Run

Run as a binary using a configuration file

```bash
./target/aerospike-backup-service -c config/config.yml
```

Run in a container with a custom configuration file

```bash
docker run -d -p 8080:8080 -v custom_config.yml:/app/config.yml --name backup-service backup-service "-lINFO"
```

Example configuration files can be found in the [config](./config/) folder.

## Monitoring

The service exposes a wide variety of system metrics that [Prometheus](https://prometheus.io/) can scrape, including the following application metrics.
| Name | Description |
| --- | --- |
| backup_runs_total | Full backup runs counter |
| backup_incremental_runs_total | Incremental backup runs counter |
| backup_skip_total | Full backup skip counter |
| backup_incremental_skip_total | Incremental backup skip counter |
| backup_failure_total | Full backup failure counter |
| backup_incremental_failure_total | Incremental backup failure counter |
| backup_duration_millis | Full backup duration in milliseconds |
| backup_incremental_duration_millis | Incremental backup duration in milliseconds |

The HTTP metrics endpoint can be found on the [OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/) page.

## FAQ

### What happens when a backup doesn’t finish before another starts?

The service will skip the next startup until the previous backup run is completed.

### Can multiple backup routines be performed simultaneously?

The service uses the asbackup shared library, which is not currently thread safe. Given this limitation, there is a global lock in which backup operations wait for each other to process.

### Which storage providers are supported?

The backup service supports AWS S3 or compatible (such as MinIO) and local storage.
