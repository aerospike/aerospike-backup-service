# Docker-Compose Installation

Two docker-compose files are provided:
- `docker-compose.yaml` - Uses the pre-built image from Docker Hub (recommended for most users)
- `docker-compose-build.yaml` - Builds the service from source (for development)

**Note:** All commands must be run from the `build/docker-compose/` directory, as the compose files use relative paths for bind mounts.

## Install

Navigate to the docker-compose directory:
```shell
cd build/docker-compose
```

Using the pre-built image (default):
```shell
docker compose -f docker-compose.yaml up -d
```

Building from source:
```shell
docker compose -f docker-compose-build.yaml up -d
```

This command sets up:

- An Aerospike container to be used as a data source.
- A [MinIO](https://min.io/) container for storing backup data.
- A Backup Service container using a sample [configuration file](./aerospike-backup-service.yml) with a routine
  specifying daily full backups and hourly incremental.

## Verify Installation

```shell
docker ps
```

```
CONTAINER ID   IMAGE                                       COMMAND                  CREATED              STATUS                        PORTS                                                             NAMES
6113ba55af02   aerospike/aerospike-backup-service:3.6.1    "aerospike-backup-se…"   About a minute ago   Up 38 seconds (healthy)       0.0.0.0:8080->8080/tcp, [::]:8080->8080/tcp                       aerospike-backup-service
018dd00cdd3c   aerospike/aerospike-server-enterprise:8.1   "/usr/bin/as-tini-st…"   About a minute ago   Up About a minute (healthy)   0.0.0.0:3000-3003->3000-3003/tcp, [::]:3000-3003->3000-3003/tcp   aerospike-cluster
9bd8386be71d   minio/minio:latest                          "/usr/bin/docker-ent…"   About a minute ago   Up About a minute (healthy)   0.0.0.0:9000-9001->9000-9001/tcp, [::]:9000-9001->9000-9001/tcp   minio
```

## View Logs

```shell
docker logs -f <container-id>
```

## Uninstall

Using the pre-built image:
```shell
docker compose -f docker-compose.yaml down
```

Building from source:
```shell
docker compose -f docker-compose-build.yaml down
```
