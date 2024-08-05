package service

import (
	"fmt"
	"time"
)

type BackupNotFoundError struct {
	Time time.Time
}

func (e BackupNotFoundError) Error() string {
	return fmt.Sprintf("no full backup found at %v", e.Time)
}

func NewBackupNotFoundError(t time.Time) error {
	return BackupNotFoundError{
		Time: t,
	}
}
