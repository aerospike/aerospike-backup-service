//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

// BackupSuite runs backup and restore scenarios against one shared Aerospike node
// with security off.
type BackupSuite struct {
	PlainClusterSuite
}

// StorageSuite backs up to each cloud storage emulator, one auth option at a time.
// The cluster is not under test, so it is the same shared plain node as BackupSuite.
type StorageSuite struct {
	PlainClusterSuite
}

// AuthSuite backs up from Aerospike nodes with security enabled, one per
// transport/authentication profile.
type AuthSuite struct {
	SecuredClusterSuite
}

// TestMain pulls every image up front, in parallel, before the suites run one
// after another.
func TestMain(m *testing.M) {
	if err := pullImages(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestBackup(t *testing.T) {
	suite.Run(t, new(BackupSuite))
}

func TestStorage(t *testing.T) {
	suite.Run(t, new(StorageSuite))
}

func TestClusterAuth(t *testing.T) {
	suite.Run(t, new(AuthSuite))
}
