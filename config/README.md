# Development config fixtures

The YAML files in this directory are not deployment templates. They exist to
exercise the full range of configuration options during local development and
CI:

- [`config.yml`](config.yml) is a broad fixture covering most backup policy,
  routine, and storage options; it doubles as a schema-validation target in
  [`.github/workflows/validate-config-files.yml`](../.github/workflows/validate-config-files.yml).
- [`minio_config.yml`](minio_config.yml) is a minimal example wired up for the
  MinIO-backed [docker-compose setup](../build/docker-compose/README.md).

For a production-oriented starting point, use
[`build/package/config/aerospike-backup-service.yml`](../build/package/config/aerospike-backup-service.yml)
instead, which is the file installed by the Docker image and Linux packages.
