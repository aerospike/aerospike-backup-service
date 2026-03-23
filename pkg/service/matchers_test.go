package service

import (
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"go.uber.org/mock/gomock"
)

type timeMatcher struct {
	expectedFromTime time.Time
}

func (m timeMatcher) Matches(x any) bool {
	if t, ok := x.(time.Time); ok {
		return t.Equal(m.expectedFromTime)
	}
	return false
}

func (m timeMatcher) String() string {
	return m.expectedFromTime.String()
}

func newTimeMatcher(fromTime time.Time) gomock.Matcher {
	return timeMatcher{expectedFromTime: fromTime}
}

// fullBackupFilterMatcher matches RoutineFilter for full backups with ToTime.
type fullBackupFilterMatcher struct{ toTime time.Time }

func (m fullBackupFilterMatcher) Matches(x any) bool {
	rf, ok := x.(*RoutineFilter)
	return ok && rf.backupType == model.BackupTypeFull &&
		rf.ToTime != nil && rf.ToTime.Equal(m.toTime)
}

func (m fullBackupFilterMatcher) String() string {
	return "full backup filter with ToTime"
}

// incrementalFilterMatcher matches RoutineFilter for incrementals with FromTime and ToTime.
type incrementalFilterMatcher struct{ fromTime, toTime time.Time }

func (m incrementalFilterMatcher) Matches(x any) bool {
	rf, ok := x.(*RoutineFilter)
	return ok && rf.backupType == model.BackupTypeIncremental &&
		rf.FromTime != nil && rf.FromTime.Equal(m.fromTime) &&
		rf.ToTime != nil && rf.ToTime.Equal(m.toTime)
}

func (m incrementalFilterMatcher) String() string {
	return "incremental backup filter with FromTime and ToTime"
}

// restoreRequestPathMatcher matches *model.RestoreRequest by BackupDataPath.
type restoreRequestPathMatcher struct{ expectedPath string }

func (m restoreRequestPathMatcher) Matches(x any) bool {
	req, ok := x.(*model.RestoreRequest)
	return ok && req != nil && req.BackupDataPath == m.expectedPath
}

func (m restoreRequestPathMatcher) String() string {
	return fmt.Sprintf("RestoreRequest with BackupDataPath=%q", m.expectedPath)
}
