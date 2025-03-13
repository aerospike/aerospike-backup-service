package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_RetrieveConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		routine   string
		timestamp time.Time
		wantErr   bool
	}{
		{
			name:      "normal",
			routine:   "routine",
			timestamp: time.UnixMilli(10),
			wantErr:   false,
		},
		{
			name:      "wrong time",
			routine:   "routine",
			timestamp: time.UnixMilli(1),
			wantErr:   true,
		},
		{
			name:      "wrong routine",
			routine:   "routine_fail_read",
			timestamp: time.UnixMilli(10),
			wantErr:   true,
		},
		{
			name:      "routine not found",
			routine:   "routine not found",
			timestamp: time.UnixMilli(10),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := restoreService.RetrieveConfiguration(context.Background(), tt.routine, &tt.timestamp)
			assert.Equal(t, tt.wantErr, err != nil, "Unexpected error presence, got: %v", err)

			if !tt.wantErr {
				assert.NotNil(t, res, "Expected non-nil result, got nil.")
			} else {
				assert.Nil(t, res, "Expected nil result as an error was expected.")
			}
		})
	}
}
