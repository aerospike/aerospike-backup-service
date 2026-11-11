package scanbackup

import (
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// fullBackupFilterMatcher matches RoutineFilter for full backups with ToTime.
type fullBackupFilterMatcher struct{ toTime time.Time }

func (m fullBackupFilterMatcher) Matches(x any) bool {
	rf, ok := x.(*service.RoutineFilter)
	return ok && rf.BackupType() == model.BackupTypeFull &&
		rf.ToTime != nil && rf.ToTime.Equal(m.toTime)
}

func (m fullBackupFilterMatcher) String() string {
	return "full backup filter with ToTime"
}

// incrementalFilterMatcher matches RoutineFilter for incrementals with FromTime and ToTime.
type incrementalFilterMatcher struct{ fromTime, toTime time.Time }

func (m incrementalFilterMatcher) Matches(x any) bool {
	rf, ok := x.(*service.RoutineFilter)
	return ok && rf.BackupType() == model.BackupTypeIncremental &&
		rf.FromTime != nil && rf.FromTime.Equal(m.fromTime) &&
		rf.ToTime != nil && rf.ToTime.Equal(m.toTime)
}

func (m incrementalFilterMatcher) String() string {
	return "incremental backup filter with FromTime and ToTime"
}

// restoreRequestPathMatcher matches *model.RestoreRequest by BackupDataPath.
type restoreRequestPathMatcher struct{ expectedPath string }

func (m restoreRequestPathMatcher) Matches(x any) bool {
	_, ok := matchRestoreRequest(x, m.expectedPath)
	return ok
}

// matchRestoreRequest returns x as a restore request if it is one for the backup at path.
func matchRestoreRequest(x any, path string) (*model.RestoreRequest, bool) {
	req, ok := x.(*model.RestoreRequest)
	if !ok || req == nil || req.BackupDataPath != path {
		return nil, false
	}

	return req, true
}

func (m restoreRequestPathMatcher) String() string {
	return fmt.Sprintf("RestoreRequest with BackupDataPath=%q", m.expectedPath)
}

// restoreRequestCompressionMatcher matches a restore request for the backup at expectedPath
// whose policy carries the expected compression mode.
type restoreRequestCompressionMatcher struct {
	expectedPath string
	expectedMode model.CompressionMode
}

func (m restoreRequestCompressionMatcher) Matches(x any) bool {
	req, ok := matchRestoreRequest(x, m.expectedPath)
	return ok && req.Policy.CompressionPolicy != nil && req.Policy.CompressionPolicy.Mode == m.expectedMode
}

func (m restoreRequestCompressionMatcher) String() string {
	return fmt.Sprintf("RestoreRequest with BackupDataPath=%q and compression %s", m.expectedPath, m.expectedMode)
}

// restoreRequestChainMatcher matches a restore request for the backup at expectedPath of a chain
// of expectedNamespace. Its disable-reordering flag and its unique policy must equal the expected
// values.
type restoreRequestChainMatcher struct {
	expectedPath              string
	expectedNamespace         string
	expectedDisableReordering bool
	expectedUnique            bool
}

func (m restoreRequestChainMatcher) Matches(x any) bool {
	req, ok := matchRestoreRequest(x, m.expectedPath)
	if !ok {
		return false
	}

	unique := req.Policy.Unique != nil && *req.Policy.Unique

	return req.Namespace == m.expectedNamespace &&
		req.DisableReordering == m.expectedDisableReordering && unique == m.expectedUnique
}

func (m restoreRequestChainMatcher) String() string {
	return fmt.Sprintf("RestoreRequest with BackupDataPath=%q, namespace %q, disable-reordering=%t and unique=%t",
		m.expectedPath, m.expectedNamespace, m.expectedDisableReordering, m.expectedUnique)
}
