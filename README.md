# Aerospike Backup Service
[![Build](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml/badge.svg)](https://github.com/aerospike/aerospike-backup-service/actions/workflows/build.yml)

The objective of this project is to provide a REST API service for the Aerospike backup and restore operations.  
The service is written in Go and wraps the [asbackup/asrestore](https://github.com/aerospike/aerospike-tools-backup) 
tools, built as shared libraries.

Use the [OpenApi generation script](./scripts/generate_OpenApi.sh) to generate OpenApi Specification for the service.

## C Library known issues
* Not thread-safe
* Uncaptured stdout logs

## Prerequisites

* Go 1.21

## Build

### Update project submodules
```bash
git submodule update --init --recursive
```

### Build the C shared libraries
```bash
cd modules/aerospike-tools-backup
make shared EVENT_LIB=libuv
../../scripts/copy_shared.sh
```
Read the official documentation for the [library build instructions](https://github.com/aerospike/aerospike-tools-backup#build-examples).

### Build the service <sup>1</sup>

#### MacOS
```bash
cd cmd/backup
CGO_ENABLED=1 go build .
```

#### Linux ARM64
```bash
cd cmd/backup
CGO_CFLAGS="-I/app/modules/aerospike-tools-backup/modules/c-client/target/Linux-aarch64/include \
  -I/app/modules/aerospike-tools-backup/include" CGO_ENABLED=1 go build .
```

#### Linux x86-64
```bash
cd cmd/backup
CGO_CFLAGS="-I/app/modules/aerospike-tools-backup/modules/c-client/target/Linux-x86_64/include \
  -I/app/modules/aerospike-tools-backup/include" CGO_ENABLED=1 go build .
```

<sup>1</sup> By default, CGO is enabled in Go when compiling for native architecture and disabled when cross-compiling.
It's therefore recommended to always set CGO_ENABLED=0 or CGO_ENABLED=1 when cross-compiling depending on whether you need to use CGO or not.

### Build Docker image
#### AMD64
```
docker build --build-arg GOARCH=amd64 -t backup-service .
```

#### ARM64
```
docker build --build-arg GOARCH=arm64 -t backup-service .
```

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
  -l, --log string      log level (default "DEBUG")
  -r, --remote string   remote configuration file path
  -v, --version         version for Use
```

### Run
Run as a binary using a configuration file
```bash
./backup -c config/config.yml
```

Run in a container with a custom configuration file
```bash
docker run -d -p 8080:8080 -v custom_config.yml:/app/config.yml --name backup-service backup-service "-lINFO"
```

Example configuration files can be found in the [config](./cmd/backup/config/) folder.
