# Aerospike Backup Service — Target Metrics Catalog

**Purpose:** Printable reference for the ideal Prometheus metrics schema (target state).  
**Source proposal:** [ABS metrics (Confluence)](https://aerospike.atlassian.net/wiki/spaces/BAR/pages/5269946442/ABS+metrics)  
**Current inventory:** `docs/metrics.json` (as of ABS v3.6.x)

---

## Status legend

| Status | Meaning |
|--------|---------|
| **Exists** | Already exported under the target name (may need minor doc/semantic alignment). |
| **Rename** | Already exported under a different name and/or unit; target is a rename or rescaled equivalent. |
| **Extend** | Same metric name today, but labels, outcomes, or semantics differ from the target. |
| **Data ready** | Not exported yet; the service already collects the underlying data ( wiring + tests only). |
| **Bigger job** | Requires new design, API changes, and/or backup-go / cross-component work. |

---

## Target metrics — Backup

### Running state (live tracking)

| Target metric | Type | Labels | Description | Status | Notes |
|---------------|------|--------|-------------|--------|-------|
| `aerospike_backup_service_backup_active` | Gauge | `routine`, `type` | Number of backups currently running for the given routine and backup type (`full` / `incremental`). Value is `1` while running, series absent when idle. | **Rename** | Today: `aerospike_backup_service_backup_in_progress` (same semantics). |
| `aerospike_backup_service_backup_progress_ratio` | Gauge | `routine`, `type` | Live progress of the currently executing backup as a float ratio from `0.0` to `1.0`. Unregisters / absent when no backup is running. | **Rename** | Today: `aerospike_backup_service_backup_progress_pct` (same data, scaled `0–100` instead of `0–1`). |
| `aerospike_backup_service_backup_estimated_completion_timestamp` | Gauge | `routine`, `type` | Application-calculated Unix epoch timestamp (seconds) when the active backup is expected to finish. Absent when no backup is running or ETA cannot be estimated yet (< ~1% progress). | **Data ready** | Computed today as `RunningJob.EstimatedEndTime` in `pkg/service/estimates.go`; not exported to Prometheus. |
| `aerospike_backup_service_backup_bytes_total` | Counter | `routine`, `type` | Cumulative total of raw bytes written to backup storage across all historical backup runs for this routine (compressed size when compression is enabled). Live throughput: `rate(...[5m])`. Never reset during process lifetime. | **Data ready** | `backup-go` stats expose bytes written (`GetStats().GetBytesWritten()`); collector needs delta tracking between scrapes. |
| `aerospike_backup_service_backup_records_total` | Counter | `routine`, `type` | Cumulative total of Aerospike records read/processed across all historical backup runs for this routine. Live throughput: `rate(...[5m])`. Never reset during process lifetime. | **Data ready** | `backup-go` stats expose `ReadRecords`; collector needs delta tracking between scrapes. |

### Finished state (historical & SLA)

| Target metric | Type | Labels | Description | Status | Notes |
|---------------|------|--------|-------------|--------|-------|
| `aerospike_backup_service_backup_events_total` | Counter | `routine`, `type`, `outcome` | Count of finalized backup executions by routine, type, and outcome: `success`, `failure`, `canceled`, `retry`, `skip`. | **Exists** | Shipped and wired via `ObserveBackupEvent`. |
| `aerospike_backup_service_backup_duration_seconds` | Histogram | `routine`, `type` | Runtime in seconds for finished backup jobs. Target: observe **successful** completions only. | **Exists** | Shipped. **Gap:** today duration is also recorded for `failure` and `canceled` when `duration > 0`. Align observation rules with target. |
| `aerospike_backup_service_last_successful_backup_timestamp_seconds` | Gauge | `routine`, `type` | Unix epoch timestamp (seconds) when the **last successful backup finished**. Constant during idle periods until the next success. | **Rename** | Today: `aerospike_backup_service_last_successful_backup_timestamp` (no `_seconds` suffix). **Gap:** currently uses metadata **`Created`** time, not **`Finished`** — target should use completion time (Cloud SLA feedback). |
| `aerospike_backup_service_last_backup_size_bytes` | Gauge | `routine`, `type` | Byte size of the last successfully completed backup (compressed serialized size when compression is enabled). Updated only on success. | **Data ready** | Available in backup metadata (`BackupMetadata.ByteCount`) after history sync. Consider reviewer rename to `last_successful_backup_bytes` (avoid word *size*). |
| `aerospike_backup_service_last_backup_size_records` | Gauge | `routine`, `type` | Record count of the last successfully completed backup. Updated only on success. | **Data ready** | Available in backup metadata (`BackupMetadata.RecordCount`). Consider reviewer rename to `last_successful_backup_record_count` (avoid word *size*; do **not** use `_total` — that suffix is for counters). |

---

## Target metrics — Restore

### Running state (live tracking, per job)

| Target metric | Type | Labels | Description | Status | Notes |
|---------------|------|--------|-------------|--------|-------|
| `aerospike_backup_service_restore_active` | Gauge | `routine` | Number of restore jobs currently running for the given routine. Restore-by-path uses sentinel `@by-path`. | **Extend** | Today: `aerospike_backup_service_restore_in_progress` — **global** gauge, no `routine` label. Per-job routine mapping exists in runners but is not exported. |
| `aerospike_backup_service_restore_progress_ratio` | Gauge | `job_id` | Live progress of a specific restore job, ratio `0.0`–`1.0`. Absent when job is not running. | **Data ready** | Progress computed in `NewRestoreRunningJob` / `restoreJob.buildStatus()`; collector must iterate jobs (today only `StatusCounts()`). |
| `aerospike_backup_service_restore_estimated_completion_timestamp` | Gauge | `job_id` | Unix epoch timestamp (seconds) when the restore job is expected to finish. Absent when not running or ETA unavailable. | **Data ready** | Same as backup ETA — `EstimatedEndTime` on restore `RunningJob`; not exported. |
| `aerospike_backup_service_restore_bytes_total` | Counter | `job_id` | Cumulative bytes written to Aerospike during restore for a specific job. Live speed: `rate(...[1m])`. Counter persists after job completes (stops incrementing). | **Data ready** | Restore stats expose bytes; needs per-job collector + delta tracking. Optional: add `routine` label (open question in proposal). |
| `aerospike_backup_service_restore_records_total` | Counter | `job_id` | Cumulative records written during restore for a specific job. Live speed: `rate(...[1m])`. | **Data ready** | Restore stats expose `ReadRecords`; needs per-job collector + delta tracking. Optional: add `routine` label. |

### Finished state (historical & audit)

| Target metric | Type | Labels | Description | Status | Notes |
|---------------|------|--------|-------------|--------|-------|
| `aerospike_backup_service_restore_events_total` | Counter | `routine`, `outcome` | Count of finalized restore jobs by routine and outcome: `success`, `failure`, `canceled`, `retry`. Restore-by-path uses `@by-path`. | **Extend** | Today: same name but **only** `outcome` label (`success`, `failure`, `canceled`). Missing: `routine`, `retry` outcome. |
| `aerospike_backup_service_restore_duration_seconds` | Histogram | `routine` | Runtime in seconds for successfully completed restore jobs. Restore-by-path uses `@by-path`. | **Data ready** | `restoreJob.started` and `restoreJob.finished` captured on completion; histogram not registered today. |

---

## Target metrics — Future / optional

| Target metric | Type | Labels | Description | Status | Notes |
|---------------|------|--------|-------------|--------|-------|
| `aerospike_backup_service_backup_stuck_seconds_total` *(proposed name TBD)* | Counter | `routine`, `type` | Monotonic counter that increases while backup progress is stalled; reset to zero on each progress advance. Emitted every scrape (~30s). Enables alerts like “no progress for 60s / 2h”. | **Bigger job** | Not in current codebase. Needs progress-stall detection design; may require `backup-go` changes. “Stuck trigger” from proposal open questions. |
| `aerospike_backup_service_restore_stuck_seconds_total` *(proposed name TBD)* | Counter | `job_id` | Same stall-detection pattern for restore jobs. | **Bigger job** | Same as backup stuck counter. |

---

## Label conventions (target)

| Label | Used on | Values / notes |
|-------|---------|----------------|
| `routine` | Backup metrics; restore aggregate metrics | Backup routine name from config. |
| `type` | Backup metrics only | `full`, `incremental`. |
| `outcome` | Event counters | Backup: `success`, `failure`, `canceled`, `retry`, `skip`. Restore: `success`, `failure`, `canceled`, `retry`. |
| `job_id` | Restore live + throughput metrics | Restore job identifier. **[BKRS-208](https://aerospike.atlassian.net/browse/BKRS-208)** (user-provided job IDs) improves operability; random int64 IDs work technically today. |
| `routine` (restore) | Restore aggregate metrics | Routine name, or `@by-path` for restore-by-path without a named routine. |

---

## Metrics to drop

### Drop immediately after migration window (already deprecated)

These are legacy v2 metrics. They duplicate `backup_events_total` and `backup_duration_seconds`. **Target: remove in a major release** once downstream dashboards/alerts migrate.

| Metric to drop | Type | Replacement |
|----------------|------|-------------|
| `aerospike_backup_service_runs_total` | Counter | `backup_events_total{outcome="success", type="full"}` |
| `aerospike_backup_service_incremental_runs_total` | Counter | `backup_events_total{outcome="success", type="incremental"}` |
| `aerospike_backup_service_failure_total` | Counter | `backup_events_total{outcome="failure", type="full"}` |
| `aerospike_backup_service_incremental_failure_total` | Counter | `backup_events_total{outcome="failure", type="incremental"}` |
| `aerospike_backup_service_skip_total` | Counter | `backup_events_total{outcome="skip", type="full"}` |
| `aerospike_backup_service_incremental_skip_total` | Counter | `backup_events_total{outcome="skip", type="incremental"}` |
| `aerospike_backup_service_duration_millis` | Gauge | `backup_duration_seconds` (histogram) |
| `aerospike_backup_service_incremental_duration_millis` | Gauge | `backup_duration_seconds` (histogram) |

### Drop after rename migration (currently active, superseded by target names)

Ship deprecated aliases for **one release cycle**, then remove.

| Metric to drop | Superseded by |
|----------------|---------------|
| `aerospike_backup_service_backup_in_progress` | `aerospike_backup_service_backup_active` |
| `aerospike_backup_service_backup_progress_pct` | `aerospike_backup_service_backup_progress_ratio` |
| `aerospike_backup_service_last_successful_backup_timestamp` | `aerospike_backup_service_last_successful_backup_timestamp_seconds` |
| `aerospike_backup_service_restore_in_progress` | `aerospike_backup_service_restore_active` |

---

## Summary scorecard

| Category | Count |
|----------|------:|
| **Exists** (ready now) | 2 |
| **Rename** | 3 |
| **Extend** (same name, more labels/outcomes) | 2 |
| **Data ready** (wire-up work) | 10 |
| **Bigger job** | 2 |
| **Total target metrics** | **17** (+ 2 optional stuck counters) |
| **Metrics to drop** | **12** (8 deprecated + 4 post-rename) |

---

## Implementation phasing (suggested)

| Phase | Scope | Effort |
|-------|--------|--------|
| **A** | Renames with deprecated aliases; backup ETA gauge; restore duration histogram; extend `restore_events_total` labels | ~1–2 weeks |
| **B** | Throughput counters (bytes/records); last-backup size gauges; restore live gauges; `@by-path`; finish-time timestamp fix | ~2–3 weeks |
| **C** | BKRS-208 user job IDs; restore per-job metrics polish; drop deprecated v2 metrics | ~2–4 weeks |
| **D** | Stuck-progress counters | separate initiative |

---

*Generated for internal planning. Update this catalog when metrics ship or the Confluence proposal changes.*
