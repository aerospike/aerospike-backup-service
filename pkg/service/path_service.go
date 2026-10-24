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
	// attemptSeparator joins a namespace and the attempt number in the folder of a retried
	// attempt. Namespace names consist of letters, digits, "_", "-" and "$" only.
	attemptSeparator = "."
	configPrefix     = "aerospike"
)

// PathService defines the canonical storage layout for backup data, metadata, and cluster configuration.
type PathService interface {
	// GetTimestampPath returns a timestamped path for a backup.
	// The path is composed of {routineName}/{backupType}/{timestamp}.
	GetTimestampPath(routineName string, timestamp time.Time, backupType model.BackupType) string

	// GetBackupPath returns the path for a specific namespace backup.
	// The path is composed of {routineName}/{backupType}/{timestamp}/data/{namespace}.
	GetBackupPath(routineName string, backupType model.BackupType, namespace string, timestamp time.Time) string

	// GetBackupAttemptPath returns the path one attempt of a namespace backup writes to. The first
	// attempt writes to GetBackupPath; attempt n > 1 writes to a sibling folder
	// {routineName}/{backupType}/{timestamp}/data/{namespace}.{n}, so a retry never shares a folder
	// with the attempt it replaces. The separator is not valid in a namespace name, so an attempt
	// folder never coincides with the folder of another namespace.
	GetBackupAttemptPath(
		routineName string, backupType model.BackupType, namespace string, timestamp time.Time, attempt int,
	) string

	// GetConfigurationPath returns the path for a configuration backup.
	// The path is composed of {routineName}/backup/{timestamp}/configuration.
	GetConfigurationPath(routineName string, timestamp time.Time) string

	// GetConfigurationFilePath returns the path for a specific configuration file within a configuration backup.
	// The path is composed of {routineName}/backup/{timestamp}/configuration/{configFile}.
	GetConfigurationFilePath(routineName string, timestamp time.Time, index int) string

	// ExtractTimestampFromPath extracts the timestamp string from a given path.
	ExtractTimestampFromPath(inputPath string) string
}

type pathService struct {
	format           *model.TimestampFormat
	timestampPattern *regexp.Regexp
}

var _ PathService = (*pathService)(nil)

// NewPathService returns a PathService. A nil format means plain epoch-millisecond timestamps.
func NewPathService(format *model.TimestampFormat) PathService {
	return &pathService{
		format: format,
		// path contains full or incremental backup folder tag, followed by 13 digits timestamp of the Unix-milliseconds.
		timestampPattern: regexp.MustCompile(
			fmt.Sprintf(`(?:[^/]+/)?[^/]+/(%s|%s)/(\d{13})(?:_[^/]*)?/`,
				fullBackupDirectory,
				incrementalBackupDirectory)),
	}
}

// GetTimestampPath returns the timestamped path for a backup.
func (s *pathService) GetTimestampPath(
	routineName string,
	timestamp time.Time,
	backupType model.BackupType,
) string {
	return path.Join(backupRootPath(routineName, backupType), s.formatTimestamp(timestamp))
}

// GetBackupPath returns the path for a specific namespace backup.
func (s *pathService) GetBackupPath(
	routineName string,
	backupType model.BackupType,
	namespace string,
	timestamp time.Time,
) string {
	return path.Join(s.GetTimestampPath(routineName, timestamp, backupType), dataDirectory, namespace)
}

// GetBackupAttemptPath returns the path for one attempt of a namespace backup.
func (s *pathService) GetBackupAttemptPath(
	routineName string,
	backupType model.BackupType,
	namespace string,
	timestamp time.Time,
	attempt int,
) string {
	folder := s.GetBackupPath(routineName, backupType, namespace, timestamp)
	if attempt <= 1 {
		return folder
	}

	return folder + attemptSeparator + strconv.Itoa(attempt)
}

// GetConfigurationPath returns the path for the configuration backup.
func (s *pathService) GetConfigurationPath(routineName string, timestamp time.Time) string {
	return path.Join(routineName, fullBackupDirectory, s.formatTimestamp(timestamp), configurationBackupDirectory)
}

// GetConfigurationFilePath returns the path for a specific configuration file.
func (s *pathService) GetConfigurationFilePath(routineName string, timestamp time.Time, index int) string {
	return path.Join(s.GetConfigurationPath(routineName, timestamp), configFileName(index))
}

// FormatTimestamp formats a timestamp into a string.
func (s *pathService) formatTimestamp(t time.Time) string {
	timestamp := strconv.FormatInt(t.UnixMilli(), 10)
	if s.format == nil {
		return timestamp
	}

	return timestamp + "_" + t.UTC().Format(model.TimestampFormatPresets[*s.format])
}

// ExtractTimestampFromPath extracts the timestamp part from a path.
func (s *pathService) ExtractTimestampFromPath(inputPath string) string {
	matches := s.timestampPattern.FindStringSubmatch(inputPath)
	if len(matches) >= 3 {
		return matches[2] // The timestamp is in the second capturing group
	}

	slog.Warn("Failed to extract timestamp", slog.String("path", inputPath))
	return ""
}

// backupRootPath returns the root path for a backup.
func backupRootPath(routineName string, backupType model.BackupType) string {
	if backupType == model.BackupTypeFull {
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
