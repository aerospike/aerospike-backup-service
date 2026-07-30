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
- [Migration Guide](docs/migration.md) — breaking changes and upgrade notes between releases
