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
  doc comment saying what it checks.
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
3. Write the doc comment first: what the scenario checks, and what it deliberately
   does not cover, and why.
4. Anything that is not the scenario itself (a container, a fixture, a helper two
   tests share) goes in a non-test file.

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
passes and server-side backup cannot pass yet is kept as a subtest skipped with the ticket that
tracks it: `s.T().Skip("BKRS-nnn: ...")`.
