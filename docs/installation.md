# Installation

This guide covers running Aerospike Backup Service (ABS) as a binary, in Docker, as a systemd service, and
building it from source. For the fastest path to a running instance, see the
[quickstart](../README.md#quickstart) in the main README.

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
<!-- tag RemoteConfig -->

```yaml
s3-storage:
  bucket: as-backup-bucket
  path: config.yml
  s3-region: eu-central-1
```
<!-- /tag -->

Run the server

```bash
./aerospike-backup-service -c remote_config.yaml -r
```

#### Docker

Run in a container with a custom configuration file:

```bash
docker run -d -p 8080:8080 -v $(pwd)/config.yml:/etc/aerospike-backup-service/aerospike-backup-service.yml --name backup-service aerospike/aerospike-backup-service:latest
```

See the [Docker Compose guide](../build/docker-compose/README.md) for a full local stack, including Aerospike and
MinIO containers.

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

The packaged service runs as the unprivileged system account `aerospike-backup-service`, which the package
creates, under a systemd sandbox. Three consequences are worth knowing before you configure it:

| | |
| --- | --- |
| Writable paths | `/etc/aerospike-backup-service`, `/var/lib/aerospike-backup-service` and `/var/log/aerospike-backup-service` only. A `local-storage` path anywhere else needs `ReadWritePaths` in a drop-in. |
| Credentials | `/root` and `/home` are hidden. Put cloud credentials under `/var/lib/aerospike-backup-service`, or in `/etc/default/aerospike-backup-service` (deb) / `/etc/sysconfig/aerospike-backup-service` (rpm). |
| File modes | Backups are created `0640`/`0750` owned by the service account. Add a reader to the group with `usermod -aG aerospike-backup-service <user>`. |

Customise the unit with `sudo systemctl edit aerospike-backup-service` — the unit itself is vendor-owned in
`/usr/lib/systemd/system` and is replaced on upgrade. A worked drop-in ships at
`/usr/share/doc/aerospike-backup-service/local-storage-path.conf.example`.

Upgrading an installation that predates the service account? See the
[Migration Guide](migration.md#v36---v37) — local-storage paths, credential locations, TLS file modes and
privileged ports each need a look, and each fails at runtime rather than at startup.

## Build from source

### Prerequisites

- Go <!-- tag GoVersion -->1.25.13<!-- /tag -->

### Build the service

The following command generates a binary under the `build/target` directory.

```bash
# Build release binary (default)
make build

# Build debug binary (includes pprof profiler on localhost:6060)
make build BUILD_MODE=debug
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
See the quick [guide](../build/package/README.md) on how to get started with the Linux packages.
