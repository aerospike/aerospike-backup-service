package fixture

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// FirstSuite is run first.
type FirstSuite struct{ suite.Suite }

// SecondSuite is run second.
type SecondSuite struct{ suite.Suite }

func TestSecond(t *testing.T) {
	suite.Run(t, new(SecondSuite))
}

func TestFirst(t *testing.T) {
	suite.Run(t, new(FirstSuite))
}
