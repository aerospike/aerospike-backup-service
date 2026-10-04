# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/).

Entries below `v3.0.0` predate this file; see the
[Migration Guide](https://github.com/aerospike/aerospike-backup-service/releases) release notes for that history.
Detailed upgrade instructions (breaking changes and how to adapt existing configuration) live in
[docs/migration.md](docs/migration.md); this file is the changelog, that one is the upgrade guide.

## [3.7.0] - TBD

### Added

- HTTPS listener (`service.https`) with optional mTLS, CRL-based client certificate revocation, TLS
  material reload without restart, and PKCS#8 encrypted keys.
- Cumulative incremental backups: `incr-mode` on backup policy (`differential` or `cumulative`).
- `schedule-timezone` on `service.backup` and on backup routines: cron schedules can run in UTC
  (default), the host timezone, or a named IANA zone. Backup paths remain UTC.
- `filter-exp` on backup routines for partial backups.
- Set index support.

### Changed

- **Breaking:** secrets are returned as `"[secret]"` by the configuration API and redacted in logs;
  sending `"[secret]"` back on `PUT` keeps the stored value.
- **Breaking:** stricter configuration validation: entity names must be a single path segment,
  paths must be clean, secret references are checked at load, and `compression`/`encryption`
  policies must set `mode`.
- **Breaking:** an empty rate-limiter `white-list` limits every client; use `0.0.0.0/0` to exempt all.
- A failed namespace is retried in its own attempt folder and no longer deletes other namespaces'
  data; permanent failures are not retried, and a namespace that cannot start fails the run.
- Retention deletes only completed incrementals; partial backups left by a crashed run are removed
  at startup.

### Security

- **Breaking:** the `.deb` and `.rpm` packages run the service as the unprivileged
  `aerospike-backup-service` account under a systemd sandbox. The config file is owner-only, backup
  artifacts are no longer world-readable, the unit file moves to `/usr/lib/systemd/system`, and
  logs move to `/var/log/aerospike-backup-service/`. See [docs/migration.md](docs/migration.md).
- Local-storage backups are created owner-only (files `0600`, directories `0700`) in every
  deployment.
- Hardened Helm and Kubernetes deployment defaults.

### Known limitations

- Restore namespace remapping (`policy.namespace`) supports a single namespace only: the restore must
  contain exactly one namespace, equal to `policy.namespace.source`. Otherwise the restore is rejected
  upfront with `namespace remap from "<source>" requires the restore to be scoped to exactly that namespace`.
  - Restore by path (`/v1/restore/full`, `/v1/restore/incremental`): point `backup-data-path` at a single
    namespace's backup (the `key` returned by `GET /v1/backups/full`) and run one restore per namespace.
  - Restore by timestamp (`/v1/restore/timestamp`): remapping works only for routines that back up a
    single namespace.
  - To restore several namespaces without remapping, omit `policy.namespace`; the destination cluster
    must have namespaces with the same names.

## [3.6.2] - 2026-08-26

Hotfix release built on [backup-go v0.11.1](https://github.com/aerospike/backup-go/releases/tag/v0.11.1).

### Fixed

- A namespace backup could fail entirely when `sindex-list` returned an index type that ABS did not recognize,
  including set indexes (Aerospike Database 8.1.2 and later). Unrecognized index types are now logged as a
  warning and skipped instead of failing the backup; set indexes themselves remain unsupported for backup.

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

[3.7.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.6.2...HEAD
[3.6.2]: https://github.com/aerospike/aerospike-backup-service/compare/v3.6.1...v3.6.2
[3.6.1]: https://github.com/aerospike/aerospike-backup-service/compare/v3.6.0...v3.6.1
[3.6.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.5.0...v3.6.0
[3.5.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.4.0...v3.5.0
[3.4.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.3.1...v3.4.0
[3.3.1]: https://github.com/aerospike/aerospike-backup-service/compare/v3.3.0...v3.3.1
[3.3.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.2.0...v3.3.0
[3.2.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.1.0...v3.2.0
[3.1.0]: https://github.com/aerospike/aerospike-backup-service/compare/v3.0.1...v3.1.0
[3.0.0]: https://github.com/aerospike/aerospike-backup-service/compare/v2.0.1...v3.0.0
