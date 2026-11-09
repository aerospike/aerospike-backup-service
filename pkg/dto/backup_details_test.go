package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFinishTimestampAndDuration(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 16, 14, 20, 0, 0, time.UTC)

	tests := map[string]struct {
		// finished is zero for a backup whose finish time is unknown.
		finished     time.Time
		wantDuration uint
	}{
		"finished":       {finished: created.Add(90 * time.Second), wantDuration: 90},
		"sub-second run": {finished: created.Add(400 * time.Millisecond)},
		"finish unknown": {},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			timestamp, duration := finishTimestampAndDuration(created, tt.finished)

			assert.Equal(t, tt.wantDuration, duration)
			if tt.finished.IsZero() {
				assert.Zero(t, timestamp)
				return
			}
			assert.Equal(t, tt.finished.UnixMilli(), timestamp)
		})
	}
}
