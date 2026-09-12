# Monitoring

The service exposes a wide variety of system metrics that [Prometheus](https://prometheus.io/) can scrape, including the
following application metrics:

<!-- tag Metrics -->

| Name                                                        | Type      | Description                                                                                                             | Labels                 |
|-------------------------------------------------------------|-----------|-------------------------------------------------------------------------------------------------------------------------|------------------------|
| `aerospike_backup_service_backup_duration_seconds`          | Histogram | Duration in seconds of finished backups by routine and type (full/incremental)                                          | routine, type          |
| `aerospike_backup_service_backup_events_total`              | Counter   | Backup service job events by routine, type (full/incremental), and outcome (success, failure, canceled, retry, skip)    | routine, type, outcome |
| `aerospike_backup_service_backup_in_progress`               | Gauge     | Number of backups currently running for the given routine and backup type (full/incremental)                            | routine, type          |
| `aerospike_backup_service_backup_progress_pct`              | Gauge     | Progress of backup processes in percentage                                                                              | routine, type          |
| `aerospike_backup_service_last_successful_backup_timestamp` | Gauge     | Unix timestamp (seconds) of the last successful backup per routine                                                      | routine, type          |
| `aerospike_backup_service_restore_events_total`             | Counter   | Total completed restore jobs by outcome (success, failure, canceled)                                                    | outcome                |
| `aerospike_backup_service_restore_in_progress`              | Gauge     | Number of restore processes running                                                                                     |                        |
| `aerospike_backup_service_duration_millis`                  | Gauge     | Full backup duration in milliseconds (Deprecated: use aerospike_backup_service_backup_duration_seconds instead.)        |                        |
| `aerospike_backup_service_failure_total`                    | Counter   | Full backup failure counter (Deprecated: use aerospike_backup_service_backup_events_total instead.)                     |                        |
| `aerospike_backup_service_incremental_duration_millis`      | Gauge     | Incremental backup duration in milliseconds (Deprecated: use aerospike_backup_service_backup_duration_seconds instead.) |                        |
| `aerospike_backup_service_incremental_failure_total`        | Counter   | Incremental backup failure counter (Deprecated: use aerospike_backup_service_backup_events_total instead.)              |                        |
| `aerospike_backup_service_incremental_runs_total`           | Counter   | Successful incremental backup runs counter (Deprecated: use aerospike_backup_service_backup_events_total instead.)      |                        |
| `aerospike_backup_service_incremental_skip_total`           | Counter   | Incremental backup skip counter (Deprecated: use aerospike_backup_service_backup_events_total instead.)                 |                        |
| `aerospike_backup_service_runs_total`                       | Counter   | Successful backup runs counter (Deprecated: use aerospike_backup_service_backup_events_total instead.)                  |                        |
| `aerospike_backup_service_skip_total`                       | Counter   | Full backup skip counter (Deprecated: use aerospike_backup_service_backup_events_total instead.)                        |                        |
<!-- /tag -->

**Example PromQL Queries**

Use these queries in Grafana panels or the Prometheus expression browser to monitor and alert on backup performance.

- ✅ Number of successful full and incremental backups for a specific routine (e.g., daily-ns1):

  `sum by (type) ( aerospike_backup_service_backup_events_total{routine="daily-ns1", outcome="success"} )`

- 🏃 Number of currently active backup processes:

  `sum(aerospike_backup_service_backup_in_progress)`

- 🔁 Number of backup retry attempts in the past hour:

  `increase(aerospike_backup_service_backup_events_total{outcome="retry"}[1h])`

- ❌ Total number of failed backups across all routines and types:

  `sum( aerospike_backup_service_backup_events_total{outcome="failure"} )`

- 🚫 Total number of Canceled backups:

  `sum(aerospike_backup_service_backup_events_total{outcome="canceled"})`

- ⏰ Time since last backup for routine:

  `time() - aerospike_backup_service_last_successful_backup_timestamp{routine="daily-ns1"}`

**Example Prometheus Alert**

- This alert fires if any backup job failure has been recorded in the last 15 minutes:

```yaml
- alert: BackupJobFailureDetected
  expr: increase(aerospike_backup_service_backup_events_total{outcome="failure"}[15m]) > 0
  labels:
    severity: warning
  annotations:
    summary: "Backup job failure detected"
    description: "At least one backup job has failed in the last 15 minutes."
```

- Alert if no successful backup in the last 24h for a specific routine (e.g., daily-ns1):

```yaml
- alert: BackupTooOld
  expr: (time() - aerospike_backup_service_last_successful_backup_timestamp{routine="daily-ns1"}) > 86400
  labels:
    severity: critical
  annotations:
    summary: "No recent backup for routine daily-ns1"
    description: "The last successful backup for routine daily-ns1 was more than 24 hours ago."
```

**Endpoints**

* `/metrics` exposes metrics for Prometheus to check performance of the backup service.
  See [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/getting_started/) for instructions.
* `/health` allows monitoring systems to check the service health.
* `/ready` checks whether the service is able to handle requests.

See
the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
on liveness and readiness probes for more information.

The HTTP metrics endpoint can be found on
the [OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/) page.

## Backup Progress Monitoring

The `aerospike_backup_service_backup_progress_pct` metric provides percentage completion for running backup processes.

**Labels**

* `routine`: Name of the backup routine
* `type`: Backup type (`full` or `incremental`)

#### How It's Calculated

The progress percentage is calculated as `Progress = (Records Processed / Total Estimated Records) × 100`.

**Total Records Estimation**

How the total is estimated depends on whether the scan carries a filter:

* **Unfiltered scan** — the default for a full backup. The namespace record count is read from the cluster's info
  client and scaled by the share of partitions being backed up. Nothing is scanned or sampled.
* **Filtered scan** — an incremental backup (which bounds the scan by modification time), or a routine that sets
  `filter-exp`, `partition-list` or `node-list`. One partition is scanned for metadata only and the sample count is
  multiplied by the number of partitions being backed up (4096 for a routine that backs up the whole namespace).

The total is not a one-time estimate: it is recalculated every 10 minutes for as long as the backup runs. A recount
that raises the denominator makes `aerospike_backup_service_backup_progress_pct` move backwards.

**Duration Estimation**

Uses linear extrapolation based on current progress rate `Estimated Total Time = Elapsed Time / Progress Percentage`.
Only available after 1% completion.

**Usage Notes**

This metric provides a reasonable estimate of backup progress and completion time,
though accuracy may vary depending on actual record distribution and processing conditions.
Early estimates should be interpreted with appropriate tolerance for variance.
The metric is useful for monitoring backup status and getting approximate completion times,
especially for longer-running backup operations.
