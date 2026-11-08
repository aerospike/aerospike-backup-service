//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// TestServerSetList backs up with a routine set-list of two of three sets and checks that only
// those sets are restored into the destination cluster.
func (s *ServerBackupSuite) TestServerSetList() {
	s.putRecord(s.client, "setA", as.BinMap{ageBin: 1})
	s.putRecord(s.client, "setB", as.BinMap{ageBin: 2})
	s.putRecord(s.client, "setC", as.BinMap{ageBin: 3})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupRoutines[routineName].SetList = []string{"setA", "setC"}
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.restoreIntoDestination(e, backup.Key)

	s.Equal(as.BinMap{ageBin: 1}, s.recordBins(s.destinationClient, "setA", 0), "setA record")
	s.Equal(as.BinMap{ageBin: 3}, s.recordBins(s.destinationClient, "setC", 0), "setC record")
	s.Nil(s.recordBins(s.destinationClient, "setB", 0), "setB record backed up")
}

// TestServerBinList backs up with a routine bin-list of two of three bins and checks that only
// those bins are restored into the destination cluster.
func (s *ServerBackupSuite) TestServerBinList() {
	s.putRecord(s.client, setName, as.BinMap{ageBin: 1, "name": "one", "city": "Haifa"})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupRoutines[routineName].BinList = []string{ageBin, "city"}
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.restoreIntoDestination(e, backup.Key)

	bins := s.recordBins(s.destinationClient, setName, 0)
	s.Require().NotNil(bins, "record not restored")
	s.Equal(as.BinMap{ageBin: 1, "city": "Haifa"}, bins, "bins outside bin-list backed up, or listed bins lost")
}

// TestServerFilterExpression backs up with a routine filter-exp of age > 25 and checks that only
// the matching records are restored into the destination cluster.
func (s *ServerBackupSuite) TestServerFilterExpression() {
	ages := []int{10, 20, 30, 40}

	filterExpression, err := as.ExpGreater(as.ExpIntBin(ageBin), as.ExpIntVal(25)).Base64()
	s.Require().NoError(err)

	s.seedRecordsIn(s.client, serverNamespace, ages)

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupRoutines[routineName].FilterExpression = filterExpression
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.restoreIntoDestination(e, backup.Key)

	for key, age := range ages {
		bins := s.recordBins(s.destinationClient, setName, key)
		if age > 25 {
			s.NotNil(bins, "matching record %d not restored", key)
			continue
		}

		s.Nil(bins, "record %d with age %d does not match the filter but was backed up", key, age)
	}
}

// TestServerNoIndexes backs up with the policy's no-indexes set and checks that restoring the
// backup creates no index on the destination cluster.
func (s *ServerBackupSuite) TestServerNoIndexes() {
	task, err := s.client.CreateIndex(nil, serverNamespace, setName, "age_sidx", ageBin, as.NUMERIC)
	s.Require().NoError(err)
	s.Require().NoError(<-task.OnComplete())
	s.T().Cleanup(func() {
		for _, client := range []*as.Client{s.client, s.destinationClient} {
			// Missing on the destination when the backup left the index out.
			_ = client.DropIndex(nil, serverNamespace, setName, "age_sidx")
		}
	})
	// A record keeps the backup from being empty, which the server cannot restore.
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupPolicies[policyName].NoIndexes = new(true)
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.restoreIntoDestination(e, backup.Key)

	s.assertRecordsIn(s.destinationClient, serverNamespace, []int{1})
	s.NotContains(s.destinationIndexes(), "age_sidx", "index backed up despite no-indexes")
}

// TestServerRestoreNamespaceMapping restores a backup of one namespace into another one of the
// destination cluster through the restore policy's namespace mapping.
func (s *ServerBackupSuite) TestServerRestoreNamespaceMapping() {
	s.seedRecordsIn(s.client, serverNamespace, []int{5, 6})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.gap("restore", gapNamespaceMapping, func() {
		s.restoreIntoDestinationWithPolicy(e, backup.Key, &dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{
			Namespace: &dto.RestoreNamespace{Source: serverNamespace, Destination: otherNamespace},
		}})

		s.assertRecordsIn(s.destinationClient, otherNamespace, []int{5, 6})
	})
}

// TestServerBackupLimits backs up with the policy's records-per-second, parallel and
// parallel-write, and checks that the server job ran with them as its backup-rps and its numbers
// of scan and upload threads.
func (s *ServerBackupSuite) TestServerBackupLimits() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		policy := c.BackupPolicies[policyName]
		// The server takes at least 5 records per second per scan thread.
		policy.RecordsPerSecond = new(100)
		policy.Parallel = new(2)
		policy.ParallelWrite = new(3)
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	fields := s.backupJobFields(backup.Key)
	s.Equal("100", fields["backup-rps"])
	s.Equal("2", fields["backup-worker-threads"])
	s.Equal("3", fields["backup-upload-threads"])
}

// TestServerRestoreOptions restores one backup several times, each with one restore policy
// option, and checks its effect: tps and parallel become the server job's restore-rps and number
// of worker threads, extra-ttl moves a restored record's expiry out by that much, and set-list
// restores only the sets it names: two of the backup's three. Each strict restore clears the
// destination namespace first. Last, it restores into a truncated namespace, which the server
// refuses until the truncate is undone with the command the error names.
func (s *ServerBackupSuite) TestServerRestoreOptions() {
	const (
		ttl      = time.Hour
		extraTTL = 24 * time.Hour
	)

	key, err := as.NewKey(serverNamespace, setName, 0)
	s.Require().NoError(err)
	s.Require().NoError(s.client.Put(as.NewWritePolicy(0, uint32(ttl.Seconds())), key, as.BinMap{ageBin: 1}))
	s.putRecord(s.client, "setA", as.BinMap{ageBin: 2})
	s.putRecord(s.client, "setB", as.BinMap{ageBin: 3})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.Run("tps", func() {
		before := s.restoreJobIDs()
		s.restoreIntoDestinationWithPolicy(e, backup.Key,
			&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{Tps: new(50)}})

		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{1})
		s.Equal("50", s.newRestoreJobFields(before)["restore-rps"])
	})

	s.Run("parallel", func() {
		before := s.restoreJobIDs()
		s.restoreIntoDestinationWithPolicy(e, backup.Key,
			&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{Parallel: new(3)}})

		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{1})
		s.Equal("3", s.newRestoreJobFields(before)["restore-worker-threads"])
	})

	s.Run("extra_ttl", func() {
		s.restoreIntoDestinationWithPolicy(e, backup.Key,
			&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{ExtraTTL: new(int64(extraTTL.Seconds()))}})

		// The extra TTL adds to what remained of the record's own TTL; time passed since it was
		// written, so it expires a little earlier than both together.
		record, err := s.destinationClient.GetHeader(nil, key)
		s.Require().NoError(err)
		expiry := time.Duration(record.Expiration) * time.Second
		s.LessOrEqual(expiry, ttl+extraTTL, "restored record expires in %s", expiry)
		s.Greater(expiry, ttl+extraTTL-10*time.Minute,
			"restored record expires in %s, want its TTL of %s plus the extra-ttl of %s", expiry, ttl, extraTTL)
	})

	s.Run("set_list", func() {
		s.restoreIntoDestinationWithPolicy(e, backup.Key,
			&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{SetList: []string{"setA", setName}}})

		s.NotNil(s.recordBins(s.destinationClient, "setA", 0), "setA record not restored")
		s.NotNil(s.recordBins(s.destinationClient, setName, 0), setName+" record not restored")
		s.Nil(s.recordBins(s.destinationClient, "setB", 0), "setB record restored")
	})

	s.Run("truncated_namespace", func() {
		undo := "truncate-namespace-undo:namespace=" + serverNamespace
		s.Require().NoError(s.destinationClient.Truncate(nil, serverNamespace, "", nil))
		// Later tests restore into the namespace too.
		s.T().Cleanup(func() { s.info(s.destinationClient, undo) })

		request := defaultRestoreRequest(backup.Key)
		request.DestinationClusterConfig.Name = destinationClusterName
		request.Policy = &dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{Mode: dto.RestoreModeStrict}}
		job := s.waitForRestoreEnd(e, s.startRestore(e, e.baseURL+"/v1/restore/full", request), serverJobTimeout)
		s.Equal(dto.RestoreFailure, job.Status)
		s.Contains(job.Error, "'"+undo+"'")

		s.Equal("ok", s.info(s.destinationClient, undo))
		s.restoreIntoDestinationWithPolicy(e, backup.Key, &dto.RestorePolicy{})
		s.assertRecordsIn(s.destinationClient, serverNamespace, []int{1})
	})
}

// TestServerMultipleNamespaces backs up a routine of two namespaces, a full and an incremental
// server backup each, and restores both chains into the destination cluster by timestamp. Restore
// by timestamp runs the namespaces at the same time, so the two cold restores overlap. The
// namespaces hold different numbers of records, and the routine state reports the server job of
// each namespace, matched to the backup it wrote.
func (s *ServerBackupSuite) TestServerMultipleNamespaces() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})
	s.seedRecordsIn(s.client, otherNamespace, []int{2, 5})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupRoutines[routineName].Namespaces = new([]dto.NamespaceName{serverNamespace, otherNamespace})
	})
	s.triggerFullBackup(e)
	s.waitForFinishedBackup(e, model.BackupTypeFull)

	fullBackups := s.getBackups(e, model.BackupTypeFull)
	s.Require().Len(fullBackups, 2, "one full backup per namespace")
	s.waitForLastBackup(e, model.BackupTypeFull, fullBackups[len(fullBackups)-1].Created)
	fullJobs := s.routineState(e).LastFullJobs
	s.assertCompleteJobs(fullJobs, fullBackups, map[string]uint64{serverNamespace: 1, otherNamespace: 2})

	s.seedRecordsIn(s.client, serverNamespace, []int{3})
	s.seedRecordsIn(s.client, otherNamespace, []int{4})
	s.triggerIncrementalBackup(e)
	s.waitForFinishedBackup(e, model.BackupTypeIncremental)

	state := s.routineState(e)
	s.assertCompleteJobs(state.LastIncrementalJobs, s.getBackups(e, model.BackupTypeIncremental),
		map[string]uint64{serverNamespace: 1, otherNamespace: 1})
	s.Equal(fullJobs, state.LastFullJobs, "an incremental backup replaced the full backup's jobs")

	s.restoreIntoDestinationByTimestamp(e, time.Now())
	s.assertRecordsIn(s.destinationClient, serverNamespace, []int{3})
	s.assertRecordsIn(s.destinationClient, otherNamespace, []int{4, 5})
}

// TestServerNonS3Storage configures server backup mode with local storage, which the server
// cannot write to, and expects the configuration to be rejected when it loads.
func (s *ServerBackupSuite) TestServerNonS3Storage() {
	config := s.baseConfig()
	s.serverBackupConfig(config)
	config.Storage[storageName] = &dto.Storage{LocalStorage: &dto.LocalStorage{Path: dto.Path(s.T().TempDir())}}

	configYAML, err := decoder.Marshal(config, decoder.YAML, false)
	s.Require().NoError(err)

	configPath := filepath.Join(s.T().TempDir(), "config.yml")
	s.Require().NoError(os.WriteFile(configPath, configYAML, 0o600))

	_, err = app.InitComponents(s.T().Context(), configPath, false)
	s.ErrorContains(err, "is not S3 storage")
}

// TestServerTwoRoutines triggers full backups of two routines on the same cluster and storage at
// once, and checks that each runs and lists its own backup only.
func (s *ServerBackupSuite) TestServerTwoRoutines() {
	const secondRoutine = "secondRoutine"

	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		second := *c.BackupRoutines[routineName]
		c.BackupRoutines[secondRoutine] = &second
	})

	before := s.routinesBackupSuccessEventCount(e, model.BackupTypeFull, routineName, secondRoutine)
	for _, routine := range []string{routineName, secondRoutine} {
		status, body := s.do(e, http.MethodPost, fmt.Sprintf("%s/v1/backups/full/%s", e.baseURL, routine), nil)
		s.Require().Equal(http.StatusAccepted, status, "trigger %s: %s", routine, body)
	}

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
		return s.routinesBackupSuccessEventCount(e, model.BackupTypeFull, routineName, secondRoutine) > before
	})
	s.Require().True(ok, "no full backup succeeded")

	s.gap("both_run", gapRoutineGate, func() {
		ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
			return s.routinesBackupSuccessEventCount(e, model.BackupTypeFull, routineName, secondRoutine) >= before+2
		})
		s.True(ok, "only one of the two routines backed up")
	})

	s.gap("own_backups", gapSharedStorage, func() {
		first, second := s.routineBackups(e, routineName), s.routineBackups(e, secondRoutine)
		s.Require().Len(first, 1)
		s.Require().Len(second, 1)
		s.NotEqual(first[0].Key, second[0].Key, "both routines list the same backup")
	})
}

// TestServerProgress checks that a running full backup reports its progress through
// GET /v1/backups/currentBackup.
func (s *ServerBackupSuite) TestServerProgress() {
	ages := []int{1, 2, 3, 4, 5}
	s.seedRecordsIn(s.client, serverNamespace, ages)

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)

	var running dto.RunningJob

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
		state := s.routineState(e)
		if state.Full == nil || state.Full.DoneRecords == 0 {
			return false
		}

		running = *state.Full

		return true
	})
	s.Require().True(ok, "no progress reported for the running full backup")
	s.gap("total_records", gapProgress, func() { s.Equal(uint64(len(ages)), running.TotalRecords) })

	s.waitForFinishedBackup(e, model.BackupTypeFull)
}

// TestServerBackupCancel cancels a running full backup through POST /v1/backups/cancel and checks
// that the server aborts the job, the routine state reports the abort and nothing is listed.
func (s *ServerBackupSuite) TestServerBackupCancel() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool { return s.routineState(e).Full != nil })
	s.Require().True(ok, "the full backup never showed as running")

	status, body := s.do(e, http.MethodPost, fmt.Sprintf("%s/v1/backups/cancel/%s", e.baseURL, routineName), nil)
	s.Require().Equal(http.StatusAccepted, status, "cancel: %s", body)

	ok = s.eventually(serverJobTimeout, pollInterval, func() bool {
		return strings.Contains(s.info(s.client, "backup-status"), "state=ABORTED")
	})
	s.True(ok, "server job not aborted: backup-status %s", s.info(s.client, "backup-status"))

	var jobs []dto.ServerBackupJob
	ok = s.eventually(serverJobTimeout, pollInterval, func() bool {
		jobs = s.routineState(e).LastFullJobs
		return len(jobs) > 0
	})
	s.Require().True(ok, "no server job reported for the canceled backup")
	s.Require().Len(jobs, 1, "server jobs of the canceled backup")
	s.Equal(serverNamespace, jobs[0].Namespace)
	s.Equal("ABORTED", jobs[0].State)
	s.Equal("operator", jobs[0].AbortOrigin)
	s.NotNil(jobs[0].FinishTime, "the abort ended the job")
	s.Nil(s.routineState(e).Full, "the canceled backup still shows as running")

	s.Empty(s.getBackups(e, model.BackupTypeFull), "a canceled backup is listed")
}

// TestServerBackupFailure backs up to a bucket that does not exist. The backup fails: the
// failure is counted, nothing shows as running, no last full backup time is set, and the routine
// state reports the server's record of the job with its reason. The server aborts a job that
// fails on a node, so the job is ABORTED by the server, unlike a canceled one (operator).
func (s *ServerBackupSuite) TestServerBackupFailure() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.Storage[storageName].S3Storage.Bucket = "missing-bucket"
	})
	failures := s.metricBackupEventCount(e, model.BackupTypeFull, outcomeFailure)

	// Not triggerFullBackup: it lists the backups first, which the missing bucket fails.
	status, body := s.do(e, http.MethodPost, fmt.Sprintf("%s/v1/backups/full/%s", e.baseURL, routineName), nil)
	s.Require().Equal(http.StatusAccepted, status, "trigger: %s", body)

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
		return s.metricBackupEventCount(e, model.BackupTypeFull, outcomeFailure) > failures
	})
	s.Require().True(ok, "the backup did not fail")
	s.Zero(s.metricBackupSuccessEventCount(e, model.BackupTypeFull))

	state := s.routineState(e)
	s.Nil(state.Full, "the failed backup still shows as running")
	s.Nil(state.LastFull, "a failed backup set the last full backup time")
	s.Require().Len(state.LastFullJobs, 1, "server jobs of the failed backup")
	job := state.LastFullJobs[0]
	s.Equal(serverNamespace, job.Namespace)
	s.Equal("ABORTED", job.State)
	s.Equal("server", job.AbortOrigin)
	s.Contains(job.ErrorReason, "cannot reach or write to the backup destination")
	s.Zero(job.RecordsBackedUp)
	s.NotNil(job.FinishTime)
}
