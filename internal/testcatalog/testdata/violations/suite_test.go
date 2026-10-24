package fixture

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RunSuite struct{ suite.Suite }

func TestRun(t *testing.T) {
	suite.Run(t, new(RunSuite))
}

func (s *RunSuite) SetupTest() {}
