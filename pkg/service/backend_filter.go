package service

import (
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupFilter is an interface that all filter types must implement.
type BackupFilter interface {
	// timeBounds returns time bounds specified for current filter.
	timeBounds() model.TimeBounds
}

// BaseFilter contains common filter attributes.
type BaseFilter struct {
	FromTime *time.Time
	ToTime   *time.Time
}

// RoutineFilter for filtering by routine and job type.
type RoutineFilter struct {
	BaseFilter
	routine *model.BackupRoutine
	JobType model.BackupType

	onlyLast bool // return last backup only
}

// NewFullBackupFilter creates a filter for full backups.
func NewFullBackupFilter(routine *model.BackupRoutine) *RoutineFilter {
	return &RoutineFilter{
		routine: routine,
		JobType: model.BackupTypeFull,
	}
}

// NewIncrementalBackupFilter creates a filter for incremental backups.
func NewIncrementalBackupFilter(routine *model.BackupRoutine) *RoutineFilter {
	return &RoutineFilter{
		routine: routine,
		JobType: model.BackupTypeIncremental,
	}
}

// Last limits results to backups with the latest Created timestamp
// among the backups that match the current filter constraints.
func (f *RoutineFilter) Last() *RoutineFilter {
	f.onlyLast = true
	return f
}

// WithFromTime sets the lower bound for Created timestamp filtering.
func (f *RoutineFilter) WithFromTime(fromTime time.Time) *RoutineFilter {
	f.FromTime = &fromTime
	return f
}

// WithToTime sets the upper bound for Created timestamp filtering.
// Returned backups will be finished by given toTime.
func (f *RoutineFilter) WithToTime(toTime time.Time) *RoutineFilter {
	f.ToTime = &toTime
	return f
}

// WithTimeBounds sets both Created timestamp bounds at once.
func (f *RoutineFilter) WithTimeBounds(bounds model.TimeBounds) *RoutineFilter {
	f.FromTime = bounds.FromTime
	f.ToTime = bounds.ToTime
	return f
}

// String returns a readable filter representation for logging/debugging.
func (f *RoutineFilter) String() string {
	return fmt.Sprintf("routine: %v type: %v last: %v timebounds: %s",
		f.routine, f.JobType, f.onlyLast, f.timeBounds().String())
}

// getPath returns the storage root path for this routine and job type.
func (f *RoutineFilter) getPath() string {
	return backupRootPath(f.routine.Name, f.JobType)
}

// getUpperBoundary returns the storage path prefix upper bound derived from ToTime.
// If ToTime is unset, all timestamps are allowed.
func (f *RoutineFilter) getUpperBoundary(pathService PathService) string {
	if f.ToTime != nil {
		return pathService.GetTimestampPath(f.routine.Name, *f.ToTime, f.JobType)
	}

	return "\uffff"
}

// PathFilter for filtering by explicit path.
type PathFilter struct {
	BaseFilter
	path    string
	storage model.Storage
}

// NewPathFilter creates a filter for retrieving backups directly from a given storage path.
func NewPathFilter(path string, storage model.Storage) *PathFilter {
	return &PathFilter{
		path:    path,
		storage: storage,
	}
}

// TimeBounds returns time bounds for a base filter.
func (b *BaseFilter) timeBounds() model.TimeBounds {
	return model.TimeBounds{
		FromTime: b.FromTime,
		ToTime:   b.ToTime,
	}
}

// WithToTime sets the upper bound for Created timestamp filtering.
// Returned backups will be finished by given toTime.
func (p *PathFilter) WithToTime(toTime time.Time) *PathFilter {
	p.ToTime = &toTime
	return p
}

// WithFromTime sets the lower bound for Created timestamp filtering.
func (p *PathFilter) WithFromTime(fromTime time.Time) *PathFilter {
	p.FromTime = &fromTime
	return p
}

// WithTimeBounds sets both Created timestamp bounds at once.
func (p *PathFilter) WithTimeBounds(bounds model.TimeBounds) *PathFilter {
	p.FromTime = bounds.FromTime
	p.ToTime = bounds.ToTime
	return p
}

func (p *PathFilter) String() string {
	return fmt.Sprintf("path: %v storage: %s timebounds: %s",
		p.path, p.storage.String(), p.timeBounds().String())
}
