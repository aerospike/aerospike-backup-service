# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/).

Entries below `v3.0.0` predate this file; see the
[Migration Guide](https://github.com/aerospike/aerospike-backup-service/releases) release notes for that history.
Detailed upgrade instructions (breaking changes and how to adapt existing configuration) live in
[docs/migration.md](docs/migration.md); this file is the changelog, that one is the upgrade guide.

## [Unreleased]

## [3.6.1] - 2026-07-23

No functional changes; see 3.6.0 below.

## [3.6.0] - 2026-07-23

### Added

- Compact backups: a `compact` flag on backup policy skips base-64 encoding for BLOB types (Bytes, HLL, RawMap,
  RawList), producing smaller backup files.
- Restore-by-timestamp `source`/`source-name` and `destination`/`destination-name` overrides.
- Restore-by-timestamp `unique` option: existing records remain unchanged, only new records are added.
- New metrics: `aerospike_backup_service_backup_in_progress`, `aerospike_backup_service_restore_events_total`.

### Changed

- Restore job `status` values are now lowercase (`running`, `success`, `failure`, `canceled`), previously
  `Running`/`Done`/`Failed`/`Canceled`.
- Default log level changed from `DEBUG` to `INFO`.
- Default cloud storage `min-part-size` increased from ~5 MB to 50 MB (S3, Azure, GCP).
- Backup scan `parallel` is now enforced per routine instead of per namespace.
- `aerospike_backup_service_backup_progress_pct` is now high-precision (float, no rounding).

## [3.5.0] - 2026-03-10

No user-facing changes tracked in this file for this release; see the GitHub release notes.

## [3.4.0] - 2025-11-05

### Added

- Rack-aware backups via a new `rack-list` field on backup routines.
- Human-readable timestamp suffixes in backup paths via `timestamp-format` (ISO/EU/US) on the backup common config.
- Scan policy properties `max-concurrent-nodes` and `use-scan-compression` on backup policy.

### Changed

- **Breaking:** `prefer-racks` moved from the backup routine to the Aerospike cluster configuration.

## [3.3.1] - 2025-09-21

Patch release; see the GitHub release notes.

## [3.3.0] - 2025-08-21

### Added

- Aerospike 8.1 support.
- Independent read/write parallelism tuning via `parallel-write` on backup policy.

### Changed

- **Breaking:** TLS configuration validation is now stricter; previously-accepted incomplete/inconsistent settings
  may now fail validation at startup.
- **Breaking:** storage config removed from `RestoreTimestampRequest`; storage is read from the routine instead.

### Fixed

- Backup reader no longer exceeds the configured number of concurrent scanning threads.
- Backup retention policy application; retention is now paused during restore.
- Handling of missing routines, full backup counters, and config-application race conditions.
- Bandwidth limiter throughput predictability.

### Security

- Dependencies updated to incorporate the latest security fixes.

## [3.2.0] - 2025-07-08

### Added

- Restore Jobs endpoint: `GET /v1/restore/jobs`, with filtering by time range and status.
- `min-part-size` support for Azure and GCP storage (previously S3-only).
- Automatic masking of private keys in logs.
- New metrics: `aerospike_backup_service_backup_events_total`, `aerospike_backup_service_backup_duration_seconds`,
  `aerospike_backup_service_last_successful_backup_timestamp`, `aerospike_backup_service_restore_in_progress`.

### Changed

- **Breaking:** `namespaces` on a backup routine is now mandatory (previously omitting it backed up all namespaces).
- **Breaking:** `bandwidth` on restore policy is now specified in MiB/s instead of bytes per second (minimum 8 MiB/s).

### Removed

- Root permissions requirement to run the service.
- `aerospike_backup_service_restore_progress_pct` metric (caused high-cardinality time series); use the restore
  status endpoint instead.

## [3.1.0] - 2025-05-15

### Added

- Strict configuration validation at startup.
- Partition-list filtering for backups (`partition-list` on backup policy).
- Object storage class support for S3, Azure Blob Storage, and Google Cloud Storage.
- `concurrent-incremental` flag on backup policy, to allow incremental backups to run alongside full backups.
- `with-cluster-configuration` flag on backup policy, to skip cluster config backup.

## [3.0.0] - 2025-01-14

### Added

- Separate storage schemas per provider type (`local-storage`, `s3-storage`, `azure-storage`, `gcp-storage`),
  replacing the unified v2 `Storage` schema.
- Configurable `RetentionPolicy` (`full`/`incremental` counts) for backup policies, replacing `KeepAll`/`RemoveAll`/
  `RemoveIncremental`.
- `node-list` property on backup routines to back up specific cluster nodes only.
- `extra-ttl` restore policy field.
- Optional `secret-agent` property on credentials, for storing passwords and TLS certificates externally.

### Changed

- **Breaking:** configuration API changes now apply immediately; the separate "apply" step from v2 was removed
  (an `apply` endpoint remains, for reloading a config file modified externally).
- **Breaking:** the `secret-agent` configuration field (singular) was renamed to `secret-agents` (plural).
- **Breaking:** restore requests now require a `backup-data-path` field; the `Storage.path` field is the storage
  root only and can no longer be reused as the backup data location.

[Unreleased]: https://github.com/aerospike/aerospike-backup-service/compare/v3.6.1...HEAD
[3.6.1]: https://github.com/aerospike/aerospike-backup-service/compare/v3.6.0...v3.6.1
[3.6.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.5.0...v3.6.0
[3.5.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.4.0...v3.5.0
[3.4.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.3.1...v3.4.0
[3.3.1]: https://github.com/aerospike/aerospike-backup-service/compare/v3.3.0...v3.3.1
[3.3.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.2.0...v3.3.0
[3.2.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.1.0...v3.2.0
[3.1.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.0.1...v3.1.0
[3.0.0]: https://github.com/aerospike/aerospike-backup-service/compare/v2.0.1...v3.0.0
