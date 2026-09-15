package decoder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateHolder mimics model.BackupTime: all state is unexported and exposed via String().
type stateHolder struct {
	full *time.Time
}

func (s *stateHolder) String() string {
	if s.full == nil {
		return "never"
	}
	return s.full.Format(time.RFC3339)
}

