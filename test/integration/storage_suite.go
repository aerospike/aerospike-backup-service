//go:build integration

package integration

import "github.com/aerospike/aerospike-backup-service/v3/pkg/dto"

// StorageSuite runs backup connectivity tests against the cloud storage backends
// (S3, Azure, GCP), one backend and one auth option at a time. It needs a working
// Aerospike cluster to back up from, but the cluster itself is not what is under
// test here - only the storage side varies.
type StorageSuite struct {
	Suite
}

// SetupSuite starts the shared Aerospike container. Each storage backend under test
// gets its own container, started lazily by the Test* method that needs it.
func (s *StorageSuite) SetupSuite() {
	s.startAerospike()
}

// storageBackup wires the given storage into a fresh ABS instance, runs a full backup
// of a handful of records, and asserts it lands.
func (s *StorageSuite) storageBackup(storage *dto.Storage) {
	s.seedRecords([]int{10, 20, 30})

	e := s.setupEnv(func(c *dto.Config) {
		c.Storage[storageName] = storage
	})

	s.triggerFullBackup(e)
	backup := s.waitForFullBackup(e)
	s.assertBackupDetails(backup, 3)
}
