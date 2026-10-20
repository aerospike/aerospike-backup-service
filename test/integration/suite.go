//go:build integration

package integration

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/suite"
)

const (
	// backupTimeout bounds how long a test waits for a backup, a restore or a metric.
	backupTimeout = 5 * time.Second
	// pollInterval is how often the service is asked again while waiting.
	pollInterval = 250 * time.Millisecond
)

// Suite is what every integration suite embeds: the Aerospike node ABS backs up
// from, a client connected to it, and every helper in this package.
type Suite struct {
	suite.Suite

	// seedNode is the node baseConfig points ABS at.
	seedNode dto.SeedNode
	// client is connected to seedNode, for seeding and checking data.
	client *as.Client
}

// PlainClusterSuite backs up from one Aerospike node with security off, started
// once for the whole suite. Use it when the cluster is not what is under test.
//
// All tests share the one namespace, so they must not call T().Parallel().
type PlainClusterSuite struct {
	Suite
}

// SetupSuite starts the shared Aerospike node.
func (s *PlainClusterSuite) SetupSuite() {
	s.startAerospike()
}

// SetupTest hands every test an empty namespace.
func (s *PlainClusterSuite) SetupTest() {
	s.truncateNamespace()
}

// truncateNamespace deletes every record in the test namespace.
func (s *Suite) truncateNamespace() {
	s.Require().NoError(s.client.Truncate(nil, namespace, "", nil))
}

// seedRecords writes one record per age into setName, keyed by its position in ages.
func (s *Suite) seedRecords(ages []int) {
	s.seedRecordsWith(s.client, ages)
}

func (s *Suite) seedRecordsWith(client *as.Client, ages []int) {
	s.seedRecordsIn(client, namespace, ages)
}

// seedRecordsIn writes one record per age into setName of ns, keyed by its position in ages.
func (s *Suite) seedRecordsIn(client *as.Client, ns string, ages []int) {
	for i, age := range ages {
		s.putRecordIn(client, ns, i, age)
	}
}

// putRecordIn writes one record with age into setName of ns under key.
func (s *Suite) putRecordIn(client *as.Client, ns string, key, age int) {
	asKey, err := as.NewKey(ns, setName, key)
	s.Require().NoError(err)
	s.Require().NoError(client.Put(as.NewWritePolicy(0, 0), asKey, as.BinMap{ageBin: age}))
}

// eventually calls check every interval until it returns true, and reports whether
// it did so within timeout. It stops early, returning false, when the test ends.
func (s *Suite) eventually(timeout, interval time.Duration, check func() bool) bool {
	ctx := s.T().Context()
	deadline := time.After(timeout)

	for {
		if check() {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-deadline:
			return false
		case <-time.After(interval):
		}
	}
}
