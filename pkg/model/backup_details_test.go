package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackupDetails_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		details BackupDetails
		want    bool
	}{
		{name: "counted files", details: BackupDetails{BackupMetadata: BackupMetadata{FileCount: 1}}},
		{name: "no files", details: BackupDetails{}, want: true},
		{name: "uncounted", details: BackupDetails{Uncounted: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.details.IsEmpty())
		})
	}
}
