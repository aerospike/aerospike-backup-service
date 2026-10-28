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

// TestServerSetList backs up with a routine set-list of one set and checks that only that set
// is restored into the destination cluster.
func (s *ServerBackupSuite) TestServerSetList() {
	s.putRecord(s.client, "setA", as.BinMap{ageBin: 1})
	s.putRecord(s.client, "setB", as.BinMap{ageBin: 2})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupRoutines[routineName].SetList = []string{"setA"}
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.restoreIntoDestination(e, backup.Key)

	s.NotNil(s.recordBins(s.destinationClient, "setA", 0), "setA record not restored")
	s.Nil(s.recordBins(s.destinationClient, "setB", 0), "setB record backed up")
}

// TestServerBinList backs up with a routine bin-list of one bin and checks that only that bin
// is restored into the destination cluster.
func (s *ServerBackupSuite) TestServerBinList() {
	s.putRecord(s.client, setName, as.BinMap{ageBin: 1, "name": "one"})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupRoutines[routineName].BinList = []string{ageBin}
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.restoreIntoDestination(e, backup.Key)

	bins := s.recordBins(s.destinationClient, setName, 0)
	s.Require().NotNil(bins, "record not restored")
	s.Contains(bins, ageBin)
	s.NotContains(bins, "name", "bin outside bin-list backed up")
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
	s.gap("no_index", gapNoIndexes, func() {
		s.NotContains(s.destinationIndexes(), "age_sidx", "index backed up despite no-indexes")
	})
}

// TestServerRestoreNamespaceMapping restores a backup of one namespace into another one of the
// destination cluster through the restore policy's namespace mapping.
func (s *ServerBackupSuite) TestServerRestoreNamespaceMapping() {
	s.seedRecordsIn(s.client, serverNamespace, []int{5, 6})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.gap("restore", gapNamespaceMapping, func() {
		request := defaultRestoreRequest(backup.Key)
		request.DestinationClusterConfig.Name = destinationClusterName
		request.Policy = &dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{
			Namespace: &dto.RestoreNamespace{Source: serverNamespace, Destination: otherNamespace},
		}}
		s.restoreWithin(e, e.baseURL+"/v1/restore/full", request, serverJobTimeout)

		s.assertRecordsIn(s.destinationClient, otherNamespace, []int{5, 6})
	})
}

// TestServerRestoreSetList restores a backup of two sets with a restore policy set-list of one,
// and checks that only that set is restored into the destination cluster.
func (s *ServerBackupSuite) TestServerRestoreSetList() {
	s.putRecord(s.client, "setA", as.BinMap{ageBin: 1})
	s.putRecord(s.client, "setB", as.BinMap{ageBin: 2})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.restoreIntoDestinationWithPolicy(e, backup.Key,
		&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{SetList: []string{"setA"}}})

	s.NotNil(s.recordBins(s.destinationClient, "setA", 0), "setA record not restored")
	s.Nil(s.recordBins(s.destinationClient, "setB", 0), "setB record restored")
}

// TestServerBackupRecordsPerSecond backs up with the policy's records-per-second and checks that
// the server job ran with it as its backup-rps.
func (s *ServerBackupSuite) TestServerBackupRecordsPerSecond() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		// The server takes at least 5 records per second per scan thread, 4 by default.
		c.BackupPolicies[policyName].RecordsPerSecond = new(100)
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.Equal("100", s.backupJobFields(backup.Key)["backup-rps"])
}

// TestServerBackupParallel backs up with the policy's parallel and checks that the server job
// ran with it as its number of scan threads.
func (s *ServerBackupSuite) TestServerBackupParallel() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupPolicies[policyName].Parallel = new(2)
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.Equal("2", s.backupJobFields(backup.Key)["backup-worker-threads"])
}

// TestServerBackupParallelWrite backs up with the policy's parallel-write and checks that the
// server job ran with it as its number of upload threads.
func (s *ServerBackupSuite) TestServerBackupParallelWrite() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupPolicies[policyName].ParallelWrite = new(3)
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.Equal("3", s.backupJobFields(backup.Key)["backup-upload-threads"])
}

// TestServerRestoreExtraTTL restores a record that expires with the restore policy's extra-ttl
// and checks that its expiry moved out by that much on the destination cluster.
func (s *ServerBackupSuite) TestServerRestoreExtraTTL() {
	const (
		ttl      = time.Hour
		extraTTL = 24 * time.Hour
	)

	key, err := as.NewKey(serverNamespace, setName, 0)
	s.Require().NoError(err)
	s.Require().NoError(s.client.Put(as.NewWritePolicy(0, uint32(ttl.Seconds())), key, as.BinMap{ageBin: 1}))

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.restoreIntoDestinationWithPolicy(e, backup.Key,
		&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{ExtraTTL: new(int64(extraTTL.Seconds()))}})

	record, err := s.destinationClient.GetHeader(nil, key)
	s.Require().NoError(err)
	s.Greater(time.Duration(record.Expiration)*time.Second, extraTTL,
		"restored record expires in %ds, want more than the extra-ttl of %s", record.Expiration, extraTTL)
}

// TestServerRestoreTps restores with the restore policy's tps and checks that the server job ran
// with it as its restore-rps.
func (s *ServerBackupSuite) TestServerRestoreTps() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	before := s.restoreJobIDs()
	s.restoreIntoDestinationWithPolicy(e, backup.Key,
		&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{Tps: new(50)}})

	s.assertRecordsIn(s.destinationClient, serverNamespace, []int{1})
	s.Equal("50", s.newRestoreJobFields(before)["restore-rps"])
}

// TestServerRestoreParallel restores with the restore policy's parallel and checks that the
// server job ran with it as its number of worker threads.
func (s *ServerBackupSuite) TestServerRestoreParallel() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	before := s.restoreJobIDs()
	s.restoreIntoDestinationWithPolicy(e, backup.Key,
		&dto.RestorePolicy{BaseRestorePolicy: dto.BaseRestorePolicy{Parallel: new(3)}})

	s.assertRecordsIn(s.destinationClient, serverNamespace, []int{1})
	s.Equal("3", s.newRestoreJobFields(before)["restore-worker-threads"])
}

// TestServerMultipleNamespaces backs up a routine of two namespaces, a full and an incremental
// server backup each, and restores both chains into the destination cluster by timestamp. Restore
// by timestamp runs the namespaces at the same time, so the two cold restores overlap.
func (s *ServerBackupSuite) TestServerMultipleNamespaces() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})
	s.seedRecordsIn(s.client, otherNamespace, []int{2})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupRoutines[routineName].Namespaces = new([]dto.NamespaceName{serverNamespace, otherNamespace})
	})
	s.triggerFullBackup(e)
	s.waitForFinishedBackup(e, model.BackupTypeFull)

	backups := s.getBackups(e, model.BackupTypeFull)
	namespaces := make([]string, 0, len(backups))
	for _, backup := range backups {
		namespaces = append(namespaces, backup.Namespace)
	}
	s.ElementsMatch([]string{serverNamespace, otherNamespace}, namespaces)
	s.waitForLastBackup(e, model.BackupTypeFull, backups[len(backups)-1].Created)

	s.seedRecordsIn(s.client, serverNamespace, []int{3})
	s.seedRecordsIn(s.client, otherNamespace, []int{4})
	s.triggerIncrementalBackup(e)
	s.waitForFinishedBackup(e, model.BackupTypeIncremental)

	s.restoreIntoDestinationByTimestamp(e, time.Now())
	s.assertRecordsIn(s.destinationClient, serverNamespace, []int{3})
	s.assertRecordsIn(s.destinationClient, otherNamespace, []int{4})
}

// TestServerCompression backs up with a ZSTD compression policy and checks that the backup is
// listed as compressed.
func (s *ServerBackupSuite) TestServerCompression() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupPolicies[policyName].CompressionPolicy = &dto.CompressionPolicy{
			Mode:  dto.CompressionModeZSTD,
			Level: new(int32(1)),
		}
	})
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.gap("compressed", gapCompression, func() { s.Equal("ZSTD", backup.Compression) })
}

// TestServerRetention keeps one full backup by retention policy, runs two, and checks that only
// the newest is listed.
func (s *ServerBackupSuite) TestServerRetention() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.BackupPolicies[policyName].RetentionPolicy = &dto.RetentionPolicy{FullBackups: new(1)}
	})
	s.triggerFullBackup(e)
	s.waitForFinishedBackup(e, model.BackupTypeFull)
	s.triggerFullBackup(e)
	newest := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.gap("older_deleted", gapRetention, func() {
		ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
			backups := s.getBackups(e, model.BackupTypeFull)
			return len(backups) == 1 && backups[0].Key == newest.Key
		})
		s.True(ok, "retention kept %d full backups, want only %s",
			len(s.getBackups(e, model.BackupTypeFull)), newest.Key)
	})
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

	s.gap("rejected", gapStorage, func() {
		_, err := app.InitComponents(s.T().Context(), configPath, false)
		s.Error(err, "server backup mode with local storage was accepted")
	})
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

	before := s.metricBackupSuccessEventCount(e, model.BackupTypeFull)
	for _, routine := range []string{routineName, secondRoutine} {
		status, body := s.do(e, http.MethodPost, fmt.Sprintf("%s/v1/backups/full/%s", e.baseURL, routine), nil)
		s.Require().Equal(http.StatusAccepted, status, "trigger %s: %s", routine, body)
	}

	ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
		return s.metricBackupSuccessEventCount(e, model.BackupTypeFull) > before
	})
	s.Require().True(ok, "no full backup succeeded")

	s.gap("both_run", gapRoutineGate, func() {
		ok := s.eventually(serverJobTimeout, pollInterval, func() bool {
			return s.metricBackupSuccessEventCount(e, model.BackupTypeFull) >= before+2
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

// TestServerClusterConfiguration runs a full backup and reads the cluster configuration saved
// with it through GET /v1/retrieve/configuration.
func (s *ServerBackupSuite) TestServerClusterConfiguration() {
	s.seedRecordsIn(s.client, serverNamespace, []int{1})

	e := s.setupEnv(s.serverBackupConfig)
	s.triggerFullBackup(e)
	backup := s.waitForFinishedBackup(e, model.BackupTypeFull)

	s.gap("retrieved", gapClusterConfig, func() {
		url := fmt.Sprintf("%s/v1/retrieve/configuration/%s/%d", e.baseURL, routineName, backup.Created.UnixMilli())
		status, body := s.do(e, http.MethodGet, url, nil)
		s.Equal(http.StatusOK, status, "retrieve configuration: %s", body)
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
// that the server aborts the job and nothing is listed.
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
	s.Empty(s.getBackups(e, model.BackupTypeFull), "a canceled backup is listed")
}
