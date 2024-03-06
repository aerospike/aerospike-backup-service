package service

import (
	"testing"
	"time"

	"github.com/reugn/go-quartz/quartz"
)

func TestNeedToRunFullBackupNow(t *testing.T) {
	tests := []struct {
		name        string
		lastFullRun time.Time
		trigger     *quartz.CronTrigger
		expected    bool
	}{
		{
			name:        "NoPreviousBackup",
			lastFullRun: time.Time{},
			trigger:     newTrigger(""),
			expected:    true,
		},
		{
			name:        "DueForBackup",
			lastFullRun: time.Now().Add(-25 * time.Hour),
			trigger:     newTrigger("@daily"),
			expected:    true,
		},
		{
			name:        "NoNeedForBackup",
			lastFullRun: time.Now().Add(-10 * time.Second),
			trigger:     newTrigger("@daily"),
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needToRunFullBackupNow(tt.lastFullRun, tt.trigger); got != tt.expected {
				t.Errorf("needToRunFullBackupNow() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func newTrigger(expression string) *quartz.CronTrigger {
	trigger, _ := quartz.NewCronTrigger(expression)
	return trigger
}
