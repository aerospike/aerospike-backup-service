package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackupPolicy_UpperBound(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		policy *BackupPolicy
		want   *time.Time
	}{
		{name: "sealed", policy: &BackupPolicy{Sealed: new(true)}, want: &start},
		{name: "not sealed", policy: &BackupPolicy{Sealed: new(false)}},
		{name: "unset: not sealed by default", policy: &BackupPolicy{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.policy.UpperBound(start))
		})
	}
}
