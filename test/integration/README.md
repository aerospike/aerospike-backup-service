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

Unlike the scan tests, every restore goes into the second cluster. Each check a scan backup
passes and server-side backup cannot pass yet is kept as a subtest skipped with the gap as its
reason (`gap*` in `serverbuild.go`).

### Gaps

**Fix in** names the component the fix belongs to; where one fix needs another first, the
first is listed first.

| Gap | Effect | Fix in | Fix | Test |
|---|---|---|---|---|
| A backup's metadata records no record or byte count | `record-count` and `byte-count` are 0; `file-count` is 1, so that restore does not take the backup for an empty one | server, backup-go, ABS | The server writes the counts it already has (in the partition manifests) into `metadata.json`; the lister returns them; ABS lists them | `TestServerRestoreByPath`, both timestamp workflows |
| A backup's metadata records no finish time | `finished` is the zero time, `finished-timestamp` and `duration` are 0 | server, backup-go, ABS | The server writes the finish time into `metadata.json`; the lister returns it; ABS lists it | none |
| The index and UDF counts of a backup are not read | `secondary-index-count` and `udf-count` are 0 | backup-go, ABS | The lister counts the definitions in `sindex.json` and `udf.json`; ABS lists them. Reliable only once the server writes those files before `metadata.json` | `TestServerBackupRestoreWithIndexes` |
| A restore job reports no record or index counts | `inserted-records` and `index-count` of the restore status are 0 | ABS | The server reports them in `restore-status` and backup-go returns them (`Applied`, `Read`, …); ABS's restore handler does not read them | `TestServerRestoreByPath` |
| A running backup reports its progress in percent | `total-records` of the running job is 100 and `done-records` the percentage, not records | ABS | Report `done-records` from `RecsBackedUp` of the backup status; the server knows no total ahead of time, so the total stays an estimate | `TestServerProgress` |
| An incremental backup with no changes is written and listed | It is listed, and becomes the last incremental; a scan backup skips it | server, then ABS | Once the metadata records counts, ABS skips an incremental with none, as it does for a scan backup. Alternatively the server does not commit an empty incremental | `TestServerRestoreByTimestampWorkflow` |
| A fuzzy restore of a backup with no records never ends | Any restore chain that starts with an empty full backup hangs | server | The server takes the backup's zero partition manifests for a failed fetch and retries; it should treat them as an empty backup and finish | `TestServerBackupRestoreWithIndexes`, both timestamp workflows |
| A fuzzy restore into another namespace never ends | A restore with a namespace mapping hangs | server, ABS | The server reads the backup under the destination namespace instead of the backup's own, finds no manifests and retries; it should read the backup's namespace, or refuse the mapping. Until it does, ABS rejects a mapping in server mode | `TestServerRestoreNamespaceMapping` |
| Cancelling or timing out a restore does not abort the server job | The job holds the namespace, and every later restore into it fails until `restore-abort` | ABS | Make the server restore handler cancelable and call `AbortRestore` (backup-go has it) when the restore is canceled or its context ends | none |
| `metadata.json` is committed before `sindex.json`, `udf.json` and the final manifests are written | A backup is listed a few seconds before it can be fully restored | server | Write `metadata.json` last: it is the commit record | none |
| The backup policy's `no-indexes` and `no-udfs` are not sent | The server backs up the indexes and the UDFs | ABS | Send them: backup-go's backup request has `NoIndexes` and `NoUDFs` | `TestServerNoIndexes` |
| The backup policy's `bandwidth` is accepted | It is ignored | ABS | Reject it in the DTO validation when `backup-mode` is `server`: it has no server equivalent | none |
| The restore policy's `no-indexes` and `no-udfs` are not sent | The indexes and the UDFs of the backup are restored | ABS | Send them: backup-go's restore request has `NoIndexes` and `NoUDFs` | none |
| Options the server has no equivalent for are accepted | The routine's `rack-list`, `partition-list` and `node-list`, and the restore policy's `replace`, `unique`, `no-generation` and `bin-list`, are ignored | ABS | Reject them in the DTO validation when `backup-mode` is `server` | none |
| Compression and encryption policies are accepted | A backup is written uncompressed and unencrypted, and listed as `NONE` | ABS | Reject them in the DTO validation when `backup-mode` is `server`; the server has neither | `TestServerCompression` |
| Storage other than S3 is accepted | The configuration loads; every backup fails | ABS | Reject it in the DTO validation when `backup-mode` is `server` | `TestServerNonS3Storage` |
| No retention is applied | Every backup is kept | ABS | Delete the oldest server backups under the storage path, as the scan retention manager does for its folders | `TestServerRetention` |
| No cluster configuration is saved | `GET /v1/retrieve/configuration` finds none | ABS | Write the cluster configuration next to each server backup, under a key the configuration retriever can find | `TestServerClusterConfiguration` |
| Only one backup runs per cluster at a time | A routine triggered while another routine backs up the same cluster is skipped | ABS | Queue the second routine's backup until the cluster is free, instead of skipping it | `TestServerTwoRoutines` |
| Backups are not kept per routine | Routines sharing a storage list each other's backups | ABS | Put each routine's server backups under a path of its own, such as `<storage path>/<routine>` | `TestServerTwoRoutines` |
| Restore into the backed-up cluster is not supported | A fuzzy restore cannot write over the live namespace | server, ABS | The server ships `restore-clear-storage`, which is not in the server build yet (backup-go already sends it); ABS runs a cold restore: `restore-prepare`, `restore-clear-storage`, `restore` | none |
| `Lister.GetMetadata` ignores the lister's prefix (BKRS-474) | A lookup of a backup by its id fails under a storage path | backup-go | Read the backup under the prefix, together with backup-cli's list fallback; ABS addresses the backup by its full key until then | none |

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
| [`TestServerSetList`](server_backup_features_test.go) | Backs up with a routine set-list of one set and checks that only that set is restored into the destination cluster. |  |
| [`TestServerBinList`](server_backup_features_test.go) | Backs up with a routine bin-list of one bin and checks that only that bin is restored into the destination cluster. |  |
| [`TestServerFilterExpression`](server_backup_features_test.go) | Backs up with a routine filter-exp of age > 25 and checks that only the matching records are restored into the destination cluster. |  |
| [`TestServerNoIndexes`](server_backup_features_test.go) | Backs up with the policy's no-indexes set and checks that restoring the backup creates no index on the destination cluster. |  |
| [`TestServerRestoreNamespaceMapping`](server_backup_features_test.go) | Restores a backup of one namespace into another one of the destination cluster through the restore policy's namespace mapping. |  |
| [`TestServerRestoreSetList`](server_backup_features_test.go) | Restores a backup of two sets with a restore policy set-list of one, and checks that only that set is restored into the destination cluster. |  |
| [`TestServerBackupRecordsPerSecond`](server_backup_features_test.go) | Backs up with the policy's records-per-second and checks that the server job ran with it as its backup-rps. |  |
| [`TestServerBackupParallel`](server_backup_features_test.go) | Backs up with the policy's parallel and checks that the server job ran with it as its number of scan threads. |  |
| [`TestServerBackupParallelWrite`](server_backup_features_test.go) | Backs up with the policy's parallel-write and checks that the server job ran with it as its number of upload threads. |  |
| [`TestServerRestoreExtraTTL`](server_backup_features_test.go) | Restores a record that expires with the restore policy's extra-ttl and checks that its expiry moved out by that much on the destination cluster. |  |
| [`TestServerRestoreTps`](server_backup_features_test.go) | Restores with the restore policy's tps and checks that the server job ran with it as its restore-rps. |  |
| [`TestServerRestoreParallel`](server_backup_features_test.go) | Restores with the restore policy's parallel and checks that the server job ran with it as its number of worker threads. |  |
| [`TestServerMultipleNamespaces`](server_backup_features_test.go) | Backs up a routine of two namespaces, one server backup each, and restores both into the destination cluster by timestamp. |  |
| [`TestServerCompression`](server_backup_features_test.go) | Backs up with a ZSTD compression policy and checks that the backup is listed as compressed. |  |
| [`TestServerRetention`](server_backup_features_test.go) | Keeps one full backup by retention policy, runs two, and checks that only the newest is listed. |  |
| [`TestServerNonS3Storage`](server_backup_features_test.go) | Configures server backup mode with local storage, which the server cannot write to, and expects the configuration to be rejected when it loads. |  |
| [`TestServerTwoRoutines`](server_backup_features_test.go) | Triggers full backups of two routines on the same cluster and storage at once, and checks that each runs and lists its own backup only. |  |
| [`TestServerClusterConfiguration`](server_backup_features_test.go) | Runs a full backup and reads the cluster configuration saved with it through GET /v1/retrieve/configuration. |  |
| [`TestServerProgress`](server_backup_features_test.go) | Checks that a running full backup reports its progress through GET /v1/backups/currentBackup. |  |
| [`TestServerBackupCancel`](server_backup_features_test.go) | Cancels a running full backup through POST /v1/backups/cancel and checks that the server aborts the job and nothing is listed. |  |
| [`TestServerRestoreByPath`](server_backup_test.go) | Mirrors TestRestoreByPath with server-side backup: runs a full backup, restores it via POST /v1/restore/full into the destination cluster, and verifies the backup details, the restored data and the exposed restore metrics. |  |
| [`TestServerBackupRestoreWithIndexes`](server_backup_test.go) | Mirrors TestBackupRestoreWithIndexes with server-side backup: backs up a secondary index and a set index and checks that restoring the backup into the destination cluster creates them there. |  |
| [`TestServerRestoreByTimestampWorkflow`](server_backup_test.go) | Mirrors TestRestoreByTimestampWorkflow with server-side backup: runs full backup, incremental backup, empty incremental backup, then restores the namespace into the destination cluster to a point in time via POST /v1/restore/timestamp. Subtests share env, backup storage, and Aerospike state. | `full`<br>`incremental`<br>`empty_incremental` |
| [`TestServerRestoreByTimestampCumulativeWorkflow`](server_backup_test.go) | Mirrors TestRestoreByTimestampCumulativeWorkflow with server-side backup: runs full backup, incremental backup, another incremental backup, then restores the namespace into the destination cluster to a point in time. Since the mode is cumulative, the second incremental should cover the first one. | `full`<br>`incremental_1`<br>`incremental_2` |

<!-- /tag -->
