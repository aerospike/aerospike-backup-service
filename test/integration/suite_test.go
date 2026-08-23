//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestIntegration(t *testing.T) {
	suite.Run(t, new(BackupSuite))
}

func TestClusterAuth(t *testing.T) {
	suite.Run(t, new(AuthSuite))
}
