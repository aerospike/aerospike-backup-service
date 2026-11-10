# Integration tests

End-to-end tests of the backup service. ABS runs in-process, built by the same
`app.InitComponents` as `cmd/backup`, against real servers in containers: Aerospike
Enterprise, Aerospike Secret Agent, and emulators for S3 (MinIO), GCS
(fake-gcs-server) and Azure Blob (Azurite). Docker must be running.

```bash
make test-integration
```

To run one suite, one test or one case, pass `-run` the runner, then the test,
then the case:

```bash
go test -tags=integration -count=1 -v ./test/integration/ -run 'TestStorage/TestS3/static_credentials_from_secret_agent'
```

## Where things are

- **`*_test.go` files are the tests.** `suite_test.go` declares the suites and their
  runners; every other `*_test.go` file declares only `Test*` methods, each with a
  doc comment saying what it checks. `internal/doccheck` enforces both rules.
- **The other `.go` files are the infrastructure**: the suite bases, the ABS
  instance (`setupEnv`), the API client (`triggerFullBackup`, `restoreByPath`, …),
  the assertions, and one file per container. The package comment in
  [`doc.go`](doc.go) maps each file.

A test reads top to bottom as: start what it needs (`startMinIO`, `startSecretAgent`,
`startSecuredNode`), configure ABS with `setupEnv(func(c *dto.Config) { … })`,
act through the API, and assert. The `customize` function changes only the field the
test is about; everything else comes from the minimal base config.

## Adding a test

1. Every test here needs at least one container. A test that runs without Docker is
   a unit test: put it next to the code it tests instead.
2. Add a `Test*` method on the suite that already starts what you need, in the file
   for its area. A new area gets a new `<area>_test.go`.
3. Write the doc comment first: its first paragraph is the row in the table below.
   Name the gaps too: what the test deliberately does not cover, and why.
4. Anything that is not the scenario itself (a container, a fixture, a helper two
   tests share) goes in a non-test file.
5. Run `make docs` to regenerate the table.

## Server-side backup

`TestServerBackup` runs the same scenarios as the scan tests in `restore_path_test.go`,
`restore_timestamp_test.go` and `restore_timestamp_cumulative_test.go` with `backup-mode:
server` (`server_backup_test.go`), and checks the routine, policy and API features a server
backup should honour (`server_backup_features_test.go`). Server-side backup is not in a
published image, so the suite starts two single-node clusters from a locally built `asd` in the
`aerospike-builder:latest` image, and skips when the build is missing:

- `ABS_IT_ASD_BINARY`: the `asd` binary, built for Linux on the Docker host's architecture.
  Default: `~/work/aerospike-server/target/Linux-<arch>/bin/asd`.
- `ABS_IT_FEATURES_CONF`: the Enterprise feature key file.
  Default: `~/work/aerospike-server-enterprise/etc/features.conf`.

Unlike the scan tests, restores go into the second cluster unless a test restores into the source. Each check a scan backup
passes and server-side backup cannot pass yet is kept as a subtest skipped with the gap as its
reason (`gap*` in `serverbuild.go`).

### Gaps

**Fix in** names the component the fix belongs to; where one fix needs another first, the
first is listed first.

| Gap | Effect | Fix in | Fix | Test |
|---|---|---|---|---|
| A backup's metadata records no record or byte count | `record-count`, `byte-count` and `file-count` are 0 | server, backup-go, ABS | The server writes the counts it already has (in the partition manifests) into `metadata.json`; the lister returns them; ABS lists them | `TestServerRestoreByPath`, both timestamp workflows |
| A backup's metadata records no finish time | `finished` is the zero time, `finished-timestamp` and `duration` are 0 | server, backup-go, ABS | The server writes the finish time into `metadata.json`; the lister returns it; ABS lists it | none |
| The index and UDF counts of a backup are not read | `secondary-index-count` and `udf-count` are 0 | backup-go, ABS | The lister counts the definitions in `sindex.json` and `udf.json`; ABS lists them. Reliable only once the server writes those files before `metadata.json` | `TestServerBackupRestoreWithIndexes` |
| A restore job reports no record or index counts | `inserted-records` and `index-count` of the restore status are 0 | ABS | The server reports them in `restore-status` and backup-go returns them (`Applied`, `Read`, …); ABS's restore handler does not read them | `TestServerRestoreByPath` |
| An incremental backup with no changes is written and listed | It is listed, and becomes the last incremental; a scan backup skips it | server, then ABS | Once the metadata records counts, ABS skips an incremental with none, as it does for a scan backup. Alternatively the server does not commit an empty incremental | `TestServerRestoreByTimestampWorkflow` |
| A restore of a backup with no records fails | The server finds no partition manifests in the backup, fails the fetch and aborts the restore: a backup of only indexes and UDFs, or a chain whose full backup has no records, cannot be restored | server | Treat a backup with zero partition manifests as an empty backup and finish | `TestServerBackupRestoreWithIndexes`, `TestServerRestoreByTimestampCumulativeWorkflow` |
| A restore into another namespace fails | ABS rejects a restore whose namespace mapping names another namespace: the server reads the backup under the destination namespace instead of the backup's own and finds no manifests | server, then ABS | The server reads the backup's own namespace; ABS then accepts the mapping | `TestServerRestoreNamespaceMapping` |
| `metadata.json` is committed before `sindex.json`, `udf.json` and the final manifests are written | A backup is listed a few seconds before it can be fully restored | server | Write `metadata.json` last: it is the commit record | none |
| No retention is applied | Every backup is kept | ABS | Delete the oldest server backups under the storage path, as the scan retention manager does for its folders | none |
| No cluster configuration is saved | `GET /v1/retrieve/configuration` finds none | ABS | Write the cluster configuration next to each server backup, under a key the configuration retriever can find | none |
| Only one backup runs per cluster at a time | A routine triggered while another routine backs up the same cluster is skipped | ABS | Queue the second routine's backup until the cluster is free, instead of skipping it | `TestServerTwoRoutines` |

## Tests

Generated by `make docs` from the doc comments in this directory; do not edit.

<!-- tag IntegrationTests -->

### TestBackup

BackupSuite runs backup and restore scenarios against one shared Aerospike node with security off.

| Test | What it checks | Cases |
|---|---|---|
| [`TestBackupWithFilterExpression`](filter_expression_test.go) | Backs up with a routine filter-exp of age > 25 and checks that only the two matching records out of five are in the backup. |  |
| [`TestClusterRejectsUnparsableFilterAsParameterError`](filter_expression_test.go) | Pins the server behavior ABS's retry classification rests on. Configuration only checks that filter-exp is base64, so an expression the server cannot parse does reach the cluster, and it must come back as PARAMETER_ERROR: the code pkg/util/try treats as not retryable (unit-tested in retry_test.go). It scans with the Aerospike client directly; ABS is not involved. |  |
| [`TestHTTPSListenerWithSecretAgentKeyPassword`](https_test.go) | Serves the ABS API over HTTPS only, from a private key encrypted on disk whose passphrase ABS fetches from Secret Agent when it builds the listener, and runs a full backup through it. |  |
| [`TestRestoreByPath`](restore_path_test.go) | Runs a full backup, wipes the namespace, restores via POST /v1/restore/full, and verifies both the restored data and the exposed restore metrics. |  |
| [`TestBackupRestoreWithIndexes`](restore_path_test.go) | Backs up a secondary index and a set index, drops both, and checks that restoring the backup recreates them. |  |
| [`TestRestoreByTimestampCumulativeWorkflow`](restore_timestamp_cumulative_test.go) | Runs full backup, incremental backup, another incremental backup, then restores the namespace to a point in time. Since the mode is cumulative, the second incremental should cover the first one. | `full`<br>`incremental_1`<br>`incremental_2`<br>`restore_by_timestamp` |
| [`TestRestoreByTimestampWorkflow`](restore_timestamp_test.go) | Runs full backup, incremental backup, empty incremental backup, then restores the namespace to a point in time via POST /v1/restore/timestamp. Subtests share env, backup storage, and Aerospike state. | `full`<br>`incremental`<br>`empty_incremental`<br>`restore_by_timestamp` |
| [`TestBackupRestoreWithSecretAgentEncryption`](secret_agent_test.go) | Starts Aerospike Secret Agent with the local file backend, fetches the backup encryption key from it, then restores with the same key. |  |

### TestStorage

StorageSuite backs up to each cloud storage emulator, one auth option at a time. The cluster is not under test, so it is the same shared plain node as BackupSuite.

| Test | What it checks | Cases |
|---|---|---|
| [`TestAzure`](storage_azure_test.go) | Exercises every way ABS can authenticate to an Azure Blob endpoint that is reachable against a self-hosted emulator, against one shared Azurite container: Shared Key (literal and Secret Agent) and a SAS token embedded in the endpoint URL. AAD and Managed Identity are not covered: they cannot be pointed at an emulator. | `shared key, literal values`<br>`shared key, account-key from secret agent`<br>`SAS token embedded in endpoint` |
| [`TestGCP`](storage_gcp_test.go) | Exercises every way ABS can authenticate to a GCS-compatible endpoint, against one shared fake-gcs-server container: the no-auth Endpoint-only path, and real service-account key auth (both key-file and key-JSON, literal and Secret Agent) with the credential exchange redirected at a local fake OAuth token endpoint instead of Google's real one. ADC (Application Default Credentials, the path taken when neither KeyFile nor Key is set and Endpoint is empty) is not covered: it can only resolve against a real GCP environment (metadata server, gcloud config, or GOOGLE_APPLICATION_CREDENTIALS), which an emulator cannot stand in for. | `endpoint, no authentication`<br>`key file, literal`<br>`key json, literal`<br>`key json from secret agent` |
| [`TestS3`](storage_s3_test.go) | Exercises every way ABS can authenticate to an S3-compatible endpoint, against one shared MinIO container: static credentials (literal and Secret Agent), the AWS SDK default credential chain (env vars), and a named shared-config profile. IAM-role credentials are not covered: MinIO has no IMDS to hand them out, and faking one adds infrastructure without proving anything about ABS's own code. | `static credentials, literal values`<br>`static credentials from secret agent`<br>`default credential chain via environment`<br>`named profile via shared credentials file` |

### TestClusterAuth

AuthSuite backs up from Aerospike nodes with security enabled, one per transport/authentication profile.

| Test | What it checks | Cases |
|---|---|---|
| [`TestInternalPlain`](auth_test.go) | Backs up over plaintext with auth-mode INTERNAL, taking the password from each place ABS can read it. | `literal password`<br>`password path`<br>`password from secret agent` |
| [`TestInternalServerTLS`](auth_test.go) | Backs up with auth-mode INTERNAL over TLS where only the server presents a certificate, trusted through ca-path. |  |
| [`TestMutualTLS`](auth_test.go) | Backs up over mutual TLS with an encrypted client key, both as an INTERNAL user and as a PKI user identified by the certificate alone. | `internal`<br>`pki`<br>`internal with key-file-password from secret agent` |

### TestServerBackup

ServerBackupSuite backs up with server-side backup from one cluster running a locally built server and restores into another. It skips unless the server build is configured.

| Test | What it checks | Cases |
|---|---|---|
| [`TestServerSetList`](server_backup_features_test.go) | Backs up with a routine set-list of two of three sets and checks that only those sets are restored into the destination cluster. |  |
| [`TestServerBinList`](server_backup_features_test.go) | Backs up with a routine bin-list of two of three bins and checks that only those bins are restored into the destination cluster. |  |
| [`TestServerFilterExpression`](server_backup_features_test.go) | Backs up with a routine filter-exp of age > 25 and checks that only the matching records are restored into the destination cluster. |  |
| [`TestServerNoIndexes`](server_backup_features_test.go) | Backs up with the policy's no-indexes set and checks that restoring the backup creates no index on the destination cluster. |  |
| [`TestServerRestoreNamespaceMapping`](server_backup_features_test.go) | Restores a backup of one namespace into another one of the destination cluster through the restore policy's namespace mapping. |  |
| [`TestServerBackupLimits`](server_backup_features_test.go) | Backs up with the policy's records-per-second, parallel and parallel-write, and checks that the server job ran with them as its backup-rps and its numbers of scan and upload threads. |  |
| [`TestServerRestoreOptions`](server_backup_features_test.go) | Restores one backup several times, each with one restore policy option, and checks its effect: tps and parallel become the server job's restore-rps and number of worker threads, extra-ttl moves a restored record's expiry out by that much, and set-list restores only the sets it names: two of the backup's three. Each strict restore clears the destination namespace first. Last, it restores into a truncated namespace, which the server refuses until the truncate is undone with the command the error names. | `tps`<br>`parallel`<br>`extra_ttl`<br>`set_list`<br>`truncated_namespace` |
| [`TestServerRestoreCancel`](server_backup_features_test.go) | Cancels a strict restore through POST /v1/restore/cancel while the server hydrates the destination namespace, and checks that ABS aborts the job and opens the namespace to clients again. The restore is throttled to keep it running until the cancel. |  |
| [`TestServerMultipleNamespaces`](server_backup_features_test.go) | Backs up a routine of two namespaces, a full and an incremental server backup each, and restores both chains into the destination cluster by timestamp. Restore by timestamp runs the namespaces at the same time, so the two cold restores overlap. The namespaces hold different numbers of records, and the routine state reports the server job of each namespace, matched to the backup it wrote. |  |
| [`TestServerNonS3Storage`](server_backup_features_test.go) | Configures server backup mode with local storage, which the server cannot write to, and expects the configuration to be rejected when it loads. |  |
| [`TestServerTwoRoutines`](server_backup_features_test.go) | Triggers full backups of two routines on the same cluster and storage at once, and checks that each runs and lists its own backups only. |  |
| [`TestServerProgress`](server_backup_features_test.go) | Checks that a running full backup reports its progress through GET /v1/backups/currentBackup as a percentage, with zero record counts: the server counts no records to back up, so any count would be made up (BKRS-504). |  |
| [`TestServerBackupCancel`](server_backup_features_test.go) | Cancels a running full backup through POST /v1/backups/cancel and checks that the server aborts the job, the routine state reports the abort and nothing is listed. |  |
| [`TestServerBackupFailure`](server_backup_features_test.go) | Backs up to a bucket that does not exist. The backup fails: the failure is counted, nothing shows as running, no last full backup time is set, and the routine state reports the server's record of the job with its reason. The server aborts a job that fails on a node, so the job is ABORTED by the server, unlike a canceled one (operator). |  |
| [`TestServerRestoreByPath`](server_backup_test.go) | Mirrors TestRestoreByPath with server-side backup: runs a full backup, restores it via POST /v1/restore/full into the destination cluster, and verifies the backup details, the restored data and the exposed restore metrics. The destination namespace holds records the backup does not: a strict restore leaves exactly the backed-up data. |  |
| [`TestServerBackupRestoreWithIndexes`](server_backup_test.go) | Mirrors TestBackupRestoreWithIndexes with server-side backup: backs up a secondary index and a set index and checks that restoring the backup into the destination cluster creates them there. |  |
| [`TestServerRestoreByTimestampWorkflow`](server_backup_test.go) | Mirrors TestRestoreByTimestampWorkflow with server-side backup: runs full backup, incremental backup, empty incremental backup, then restores the namespace into the destination cluster to a point in time via POST /v1/restore/timestamp. Unlike the scan test, the full backup holds a record, which the incremental backup holds updated: the server cannot restore an empty backup, and the restore must end with the newest value. A point in time between the two backups restores the full backup alone, and one before the full backup restores nothing. Subtests share env, backup storage, and Aerospike state. | `full`<br>`incremental`<br>`empty_incremental`<br>`restore_by_timestamp`<br>`restore_between_full_and_incremental`<br>`restore_before_full` |
| [`TestServerRestoreByTimestampIntoEmptyNamespace`](server_backup_test.go) | Restores a full and an incremental backup by timestamp into a destination namespace that holds no records, in each restore mode. An empty namespace is where restore by timestamp switches a scan restore to the unique policy, which applies a chain newest first; the server takes a chain only full backup first. |  |
| [`TestServerRestoreByTimestampCumulativeWorkflow`](server_backup_test.go) | Mirrors TestRestoreByTimestampCumulativeWorkflow with server-side backup: runs full backup, incremental backup, another incremental backup, then restores the namespace into the destination cluster to a point in time. Since the mode is cumulative, the second incremental should cover the first one. | `full`<br>`incremental_1`<br>`incremental_2` |
| [`TestServerFuzzyRestoreIntoSource`](server_fuzzy_restore_test.go) | Backs up the source cluster, then updates one record, deletes another and adds a third, and restores the backup in fuzzy mode into the source cluster itself, which stays live. The updated and deleted records get their backed-up values back; the added record, which the backup does not hold, stays. |  |
| [`TestServerFuzzyRestoreByTimestamp`](server_fuzzy_restore_test.go) | Backs up a full and an incremental backup of the source cluster, updates the record again, and restores the chain by timestamp in fuzzy mode into the source cluster: the record ends with its value from the incremental backup. |  |
| [`TestServerRestoreRequestRejected`](server_fuzzy_restore_test.go) | Sends restore requests that server mode does not accept: one by path and one by timestamp without a mode, which server mode requires, one with a mode that does not exist, one with an option the server has no equivalent for, and one that maps the backup's namespace onto another. Each is rejected before any job starts. |  |

<!-- /tag -->
