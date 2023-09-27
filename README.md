# Aerospike Backup Service
[![Build](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml/badge.svg)](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml)

The objective of this project is to provide a REST API service for the Aerospike backup and restore operations.  
The service is written in Go and wraps the [asbackup/asrestore](https://github.com/aerospike/aerospike-tools-backup) 
tools, built as shared libraries.

## Feature list

- [x] Local backup storage
- [x] S3 backup storage
- [x] Core REST API
- [x] Prometheus metrics
- [x] Backup policy configuration
- [x] Scheduled backup capability
- [x] REST API for restore operation
- [ ] OpenAPI Specification
- [ ] Basic UI interface
- [ ] Dockerfile

## C Library known issues
* Not thread-safe
* Uncaptured stdout logs
* System signals interceptors
* S3 configuration properties are not read from the backup_config

## Prerequisites

* Go 1.21

## Build

* Update project submodules
```bash
git submodule update —init —recursive
```

* Build the C shared library
```bash
cd modules/aerospike-tools-backup
make shared EVENT_LIB=libuv
./scripts/copy_shared.sh
```
Read the official documentation for the [library build instructions](https://github.com/aerospike/aerospike-tools-backup#build-examples).

* Build the service <sup>1</sup>
```bash
cd cmd/backup
CGO_ENABLED=1 go build .
```
<sup>1</sup> By default, CGO is enabled in Go when compiling for native architecture and disabled when cross-compiling.
It's therefore recommended to always set CGO_ENABLED=0 or CGO_ENABLED=1 when cross-compiling depending on whether you need to use CGO or not.

## Usage

### Service help
```
% ./backup -h                  
Aerospike Backup Service

Usage:
  Use the following properties for service configuration [flags]

Flags:
  -c, --config string   configuration file path
  -h, --help            help for Use
      --host string     service host (default "0.0.0.0")
      --port int        service port (default 8080)
  -v, --version         version for Use
```

### Run using configuration file
```bash
./backup -c config/config.yml
```
Example configuration files can be found in the [config](./cmd/backup/config/) folder.
