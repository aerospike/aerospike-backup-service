package service

import (
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

const (
	metadataFile                 = "metadata.yaml"
	configExt                    = ".conf"
	incrementalBackupDirectory   = "incremental"
	fullBackupDirectory          = "backup"
	configurationBackupDirectory = "configuration"
	dataDirectory                = "data"
	configPrefix                 = "aerospike"
)

// PathService defines the interface for path-related operations.
type PathService interface {
	// GetTimestampPath returns a timestamped path for a backup.
	// The path is composed of {routineName}/{backupType}/{timestamp}.
	GetTimestampPath(routineName string, timestamp time.Time, backupType jobType) string

	// GetBackupPath returns the path for a specific namespace backup.
	// The path is composed of {routineName}/{backupType}/{timestamp}/data/{namespace}.
	GetBackupPath(routineName string, backupType jobType, namespace string, timestamp time.Time) string

	// GetConfigurationPath returns the path for a configuration backup.
	// The path is composed of {routineName}/backup/{timestamp}/configuration.
	GetConfigurationPath(routineName string, timestamp time.Time) string

	// GetConfigurationFilePath returns the path for a specific configuration file within a configuration backup.
	// The path is composed of {routineName}/backup/{timestamp}/configuration/{configFile}.
	GetConfigurationFilePath(routineName string, timestamp time.Time, index int) string

	// ExtractTimestampFromPath extracts the timestamp string from a given path.
	ExtractTimestampFromPath(path string) string
}

// PathServiceImpl implements the PathService interface.
type PathServiceImpl struct {
	format           *model.TimestampFormat
	timestampPattern *regexp.Regexp
}

// NewPathService creates a new PathServiceImpl.
func NewPathService(format *model.TimestampFormat) *PathServiceImpl {
	return &PathServiceImpl{
		format: format,
		timestampPattern: regexp.MustCompile(
			fmt.Sprintf(`(?:[^/]+/)?[^/]+/(%s|%s)/(\d{13})(?:_[^/]*)?/`,
				fullBackupDirectory,
				incrementalBackupDirectory)),
	}
}

// GetTimestampPath returns the timestamped path for a backup.
func (s *PathServiceImpl) GetTimestampPath(routineName string, timestamp time.Time, backupType jobType) string {
	return path.Join(backupRootPath(routineName, backupType), s.formatTimestamp(timestamp))
}

// GetBackupPath returns the path for a specific namespace backup.
func (s *PathServiceImpl) GetBackupPath(
	routineName string,
	backupType jobType,
	namespace string,
	timestamp time.Time,
) string {
	return path.Join(s.GetTimestampPath(routineName, timestamp, backupType), dataDirectory, namespace)
}

// GetConfigurationPath returns the path for the configuration backup.
func (s *PathServiceImpl) GetConfigurationPath(routineName string, timestamp time.Time) string {
	return path.Join(routineName, fullBackupDirectory, s.formatTimestamp(timestamp), configurationBackupDirectory)
}

// GetConfigurationFilePath returns the path for a specific configuration file.
func (s *PathServiceImpl) GetConfigurationFilePath(routineName string, timestamp time.Time, index int) string {
	return path.Join(s.GetConfigurationPath(routineName, timestamp), configFileName(index))
}

// FormatTimestamp formats a timestamp into a string.
func (s *PathServiceImpl) formatTimestamp(t time.Time) string {
	timestamp := strconv.FormatInt(t.UnixMilli(), 10)
	if s.format == nil {
		return timestamp
	}

	return timestamp + "_" + t.Format(model.TimestampFormatPresets[*s.format])
}

// ExtractTimestampFromPath extracts the timestamp part from a path.
func (s *PathServiceImpl) ExtractTimestampFromPath(path string) string {
	matches := s.timestampPattern.FindStringSubmatch(path)
	if len(matches) >= 3 {
		return matches[2] // The timestamp is in the second capturing group
	}

	slog.Warn("Failed to extract timestamp", slog.String("path", path))
	return ""
}

// backupRootPath returns the root path for a backup.
func backupRootPath(routineName string, backupType jobType) string {
	if backupType == jobTypeFull {
		return path.Join(routineName, fullBackupDirectory)
	}

	return path.Join(routineName, incrementalBackupDirectory)
}

// configFileName returns the name of a configuration file based on an index.
func configFileName(index int) string {
	return fmt.Sprintf("%s_%d%s", configPrefix, index, configExt)
}

// extractBackupDirFromKey return backup folder
// input: "storage/test-routine/backup/1609632000000/data/test-ns"
// output: "storage/test-routine/backup/1609632000000"
func extractBackupDirFromKey(key string) string {
	return path.Dir(path.Dir(path.Clean(key)))
}
