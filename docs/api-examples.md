# Example requests and responses

The following sections provide example requests and responses for various operations.
For full API documentation, refer to
the [Aerospike Backup Service OpenAPI specification](https://aerospike.github.io/aerospike-backup-service/#/System/metrics).

While command-line tools such as curl, httpie, or wget work well for interacting with the API, a graphical
interface (such as [Postman](https://www.postman.com/downloads/) or [Insomnia](https://insomnia.rest/)) is generally
recommended for a more convenient and user-friendly experience.

## Backup

#### Trigger On-Demand Backup

ℹ️ *Available since v1.0*

This request starts the backup operation for the specified routine, regardless of its configured schedule.

[
`POST {{baseUrl}}/v1/backups/schedule/<routineName>?delay=<timeout>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/scheduleFullBackup)

* routineName: The name of the backup routine to trigger.
* delay (optional): Time in milliseconds to delay the start of the backup.

If the request is accepted, the server responds with Http 202 Accepted.

#### Get Current Backup

ℹ️ *Available since v1.0*

This endpoint retrieves the current statistics for a backup in progress, identified by its routine name.

[
`GET {{baseUrl}}/v1/backups/currentBackup/<routineName>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getCurrentBackup)

* routineName: The name of the routine for which to retrieve current backup information.

<details>
    <summary>Response</summary>

<!-- CurrentBackupResponse -->

```json
{
  "full": {
    "total-records": 100000,
    "done-records": 50000,
    "start-time": "2024-01-01T12:00:00Z",
    "percentage-done": 50,
    "estimated-end-time": "2024-01-01T13:00:00Z",
    "metrics": {
      "records-per-second": 1000,
      "kilobytes-per-second": 30000,
      "pipeline": 167
    },
    "duration": 1800
  }
}
```

See [fields description](readme/dto/dto.runningjob.md) for details.

</details>

#### Cancel Backup Job

ℹ️ *Available since v3.0*

[
`POST {{baseUrl}}/v1/backups/cancel/<routineName>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/cancelCurrentBackup)

Cancel all currently running backups (both full and incremental) for the specified routine. Partially created backups
will be deleted.

#### Retrieve Backup List

ℹ️ *Available since v1.0*

Provides a list of backups for each configured routine, including details such as creation time, duration, namespace,
and storage location.

[`GET {{baseUrl}}/v1/backups/full`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getFullBackups)

<details>
    <summary>Response</summary>

The response is a map of routine names to lists of backups.
<!-- FullBackupsResponse -->

```json
{
  "routine1": [
    {
      "created": "2024-01-01T12:00:00Z",
      "timestamp": 1704110400000,
      "finished": "2024-01-01T12:05:00Z",
      "finished-timestamp": 0,
      "duration": 300,
      "namespace": "source-ns1",
      "record-count": 42,
      "byte-count": 480000,
      "file-count": 1,
      "secondary-index-count": 5,
      "udf-count": 1,
      "key": "routine1/backup/1704110400000/source-ns1",
      "storage": {
        "s3-storage": {
          "bucket": "as-backup-bucket",
          "path": "backups",
          "s3-region": "eu-central-1"
        }
      },
      "compression": "ZSTD",
      "encryption": "NONE"
    }
  ]
}
```

For fields description see [fields description](readme/dto/dto.backupdetails.md)

</details>

You can filter the results by adding query parameters:

[
`GET {{baseUrl}}/v1/backups/full/<name>?from=<from>&to=<to>`](https://aerospike.github.io/aerospike-backup-service/#/Backup/getFullBackups)

Here, `name` is the routine name, `from` and `to` are timestamps in milliseconds since epoch.

#### Disable Routine

ℹ️ *Available since v3.0*

[
`POST {{baseUrl}}/v1/routines/<routineName>/disable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/disableRoutine)

[
`POST {{baseUrl}}/v1/routines/<routineName>/enable/`](https://aerospike.github.io/aerospike-backup-service/#/Configuration/enableRoutine)

Set the disabled flag for the given routine to `true` or `false` (default is `false`).

- Disabled routines will not schedule new jobs.
- Running jobs will be canceled, similar to the `Cancel Backup Job` endpoint.

## Restore

#### Direct restore using a specific backup

ℹ️ *Available since v1.0*

This request restores a backup from a specified path to a designated destination.

[
`POST {{baseUrl}}/v1/restore/full`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreFull)

<details>
    <summary>Request body</summary>

<!-- RestoreFullRequest -->

```json
{
  "destination": {
    "seed-nodes": [
      {
        "host-name": "host.docker.internal",
        "port": 3000
      }
    ],
    "credentials": {
      "user": "user",
      "password": "password"
    }
  },
  "source": {
    "s3-storage": {
      "bucket": "as-backup-bucket",
      "path": "backups",
      "s3-region": "eu-central-1"
    }
  },
  "policy": {
    "no-generation": true
  },
  "backup-data-path": "routine1/backup/1704110400000/source-ns1"
}
```

The `no-generation` parameter allows overwriting of existing keys if set to `true`.

In the `source` section, `path` is the `key` value returned as a response in the [Retrieve Backup List](#retrieve-backup-list)
example.

The `destination` field says where to restore to. It can be any Aerospike cluster.

You can also use `destination-name` and `storage-name` instead of `destination` and `storage` respectively.
They refer to the names of the corresponding entities in the configuration file.

For more details see [fields description](readme/dto/dto.restorerequest.md)

</details>

The response is a job ID.

#### Restore using routine name and timestamp

ℹ️ *Available since v1.0*

This option automatically restores data by identifying and applying the
appropriate backup sequence based on the specified timestamp.
For each namespace defined in the backup routine, the system locates the most recent full backup
prior to the given time and applies all incremental backups created after that full backup,
up to the target timestamp.

There is no need to specify individual backup paths or storage locations — the system handles this internally. The
restore process requires a full backup as a foundation; incremental backups cannot be used on their own.

By default, backups are applied in chronological order. However, when restoring to an empty namespace, the system may
reverse the order of application and use the `CREATE_ONLY` policy. This optimization ensures that each record is written
exactly once—applying only the latest version—thus reducing write load and generation noise. If needed, this
optimization can be disabled using the `disable-reordering` flag in the `RestoreTimestampRequest`.

Overall, the process is fully automated: users do not need to manually choose or arrange backups for the restore to
succeed. The restore process runs in parallel for every namespace.

**Example**

```text
Timeline ─────────────────────────────────────────────────────────────────────────────────────────▶

Backups:
   [Full A]──[Incr A1]──[Incr A2]──[Full B]──[Incr B1]──[Incr B2]──▶ T ◀──[Incr B3]──[Full C]──...
                                                                     ↑
                                                               Restore Point
```

What gets restored at T2:

* Full backup: `Full B`
* Incremental backups: `Incr B1`, `Incr B2`
* Excluded: `Incr B3` and anything after T2

Restore order (to empty namespace): `Incr B2`, `Incr B1`, `Full B`.

- Backups are applied in reverse order. This ensures that the most recent version of each record is restored first. Any
  earlier versions of the same record are skipped, by using `CREATE_ONLY` policy, reducing unnecessary writes.

Restore order (to non-empty namespace or with `disable-reordering`): `Full B`, `Incr B1`, `Incr B2`.

* Backups are applied in chronological order.
  All versions of each record are restored step by step.
  If a record was modified multiple times, each update is applied, with the final version appearing last.

[`
POST {{baseUrl}}/v1/restore/timestamp`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreTimestamp)

<details>
    <summary>Request body</summary>

```json
{
  "destination-name": "abs-cluster",
  "time": 1704110400000,
  "routine": "routine1"
}
```

Request is almost identical to [restore by path](#direct-restore-using-a-specific-backup), but instead of
`backup-data-path`
should provide a pair `time` and `routine`.
`time` is epoch milliseconds (an absolute instant); `schedule-timezone` does not change how it is interpreted.

Optional overrides:
- `destination` / `destination-name` overrides destination cluster from routine.
- `source` / `source-name` overrides storage from routine.
- Any omitted value falls back to the referenced routine config.

For more details see [fields description](readme/dto/dto.restoretimestamprequest.md)
</details>

The response is a job ID.

#### Restore job status

ℹ️ *Available since v1.0*

You can get job status with the endpoint

[
`GET {{baseUrl}}/v1/restore/status/<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/restoreStatus).

It works identical for both restore types.

<details>
    <summary>Response example</summary>
<!-- CurrentRestoreResponse -->

```json
{
  "read-records": 100000,
  "total-bytes": 30000000,
  "expired-records": 0,
  "skipped-records": 0,
  "ignored-records": 0,
  "inserted-records": 50000,
  "existed-records": 0,
  "fresher-records": 0,
  "index-count": 4,
  "udf-count": 1,
  "errors-in-doubt": 0,
  "current-job": {
    "total-records": 100000,
    "done-records": 50000,
    "start-time": "2024-01-01T12:00:00Z",
    "percentage-done": 50,
    "estimated-end-time": "2024-01-01T13:00:00Z",
    "metrics": {
      "records-per-second": 1000,
      "kilobytes-per-second": 30000,
      "pipeline": 8192
    },
    "duration": 1800
  },
  "status": "running"
}
```

For fields description see [fields description](readme/dto/dto.restorejobstatus.md)
</details>

#### Retrieve Restore Jobs

ℹ️ *Available since v3.2*

Provides a list of all restore jobs, with optional filtering by time range and status.

[
`GET {{baseUrl}}/v1/restore/jobs?from=<from>&to=<to>&status=<status>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/retrieveRestoreJobs)

- `from` (optional): Lower bound timestamp filter in milliseconds since epoch.
- `to` (optional): Upper bound timestamp filter in milliseconds since epoch.
- `status` (optional): Comma-separated status filter (e.g., `Running,Done,Failed,Canceled`). Use `!` prefix to exclude
  statuses (e.g., `!Failed,Canceled`).

<details>
    <summary>Response example</summary>

<!-- CurrentRestoresResponse -->

```json
{
  "12345678": {
    "read-records": 100000,
    "total-bytes": 30000000,
    "expired-records": 0,
    "skipped-records": 0,
    "ignored-records": 0,
    "inserted-records": 50000,
    "existed-records": 0,
    "fresher-records": 0,
    "index-count": 4,
    "udf-count": 1,
    "errors-in-doubt": 0,
    "current-job": {
      "total-records": 100000,
      "done-records": 50000,
      "start-time": "2024-01-01T12:00:00Z",
      "percentage-done": 50,
      "estimated-end-time": "2024-01-01T13:00:00Z",
      "metrics": {
        "records-per-second": 1000,
        "kilobytes-per-second": 30000,
        "pipeline": 0
      },
      "duration": 1800
    },
    "status": "running"
  }
}
```

</details>

#### Cancel Restore Job

ℹ️ *Available since v3.0*

Cancel the restore job identified by `<jobId>`. Data that has already been restored will remain intact.

[
`POST {{baseUrl}}/v1/restore/cancel/<jobId>`](https://aerospike.github.io/aerospike-backup-service/#/Restore/cancelRestore)
