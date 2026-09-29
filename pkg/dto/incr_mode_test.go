package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestIncrMode_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mode    IncrMode
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"differential is valid", IncrModeDifferential, false},
		{"cumulative is valid", IncrModeCumulative, false},
		{"invalid mode", "invalid", true},
		{"case insensitive", "CUMULATIVE", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mode.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIncrMode_ToModel(t *testing.T) {
	tests := []struct {
		name     string
		mode     IncrMode
		expected model.IncrMode
	}{
		{"empty to empty", "", ""},
		{"differential to differential", IncrModeDifferential, model.IncrModeDifferential},
		{"cumulative to cumulative", IncrModeCumulative, model.IncrModeCumulative},
		{"case insensitive to lowercase", "CUMULATIVE", model.IncrModeCumulative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.ToModel())
		})
	}
}

func TestNewIncrModeFromModel(t *testing.T) {
	tests := []struct {
		name     string
		mode     model.IncrMode
		expected IncrMode
	}{
		{"empty to empty", "", ""},
		{"differential to differential", model.IncrModeDifferential, IncrModeDifferential},
		{"cumulative to cumulative", model.IncrModeCumulative, IncrModeCumulative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NewIncrModeFromModel(tt.mode))
		})
	}
}
