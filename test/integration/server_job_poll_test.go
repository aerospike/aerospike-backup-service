//go:build integration

package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

// TestServerJobPoll starts an incremental backup by hand into the routine's path, as a person
// would from a CLI, and checks that backup-status reports where the job writes and its time range,
// and that ABS's poll ties the job to the routine.
func (s *ServerBackupSuite) TestServerJobPoll() {
	logFile := filepath.Join(s.T().TempDir(), "abs.log")
	s.setupEnv(s.serverBackupConfig, func(c *dto.Config) {
		c.ServiceConfig.Logger = &dto.LoggerConfig{
			Level:        "INFO",
			Format:       "JSON",
			StdoutWriter: new(false),
			FileWriter:   &dto.FileLoggerConfig{Filename: dto.Path(logFile)},
		}
	})

	// Job ids are single-use for the cluster's uptime, and the suite's cluster outlives one run.
	jobID := fmt.Sprintf("manual-%d", time.Now().UnixNano()%1e9)
	routinePath := path.Join(s.storagePath(), routineName)
	before := strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)
	after := strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	answer := s.info(s.client, fmt.Sprintf(
		"backup:namespace=%s;job-id=%s;object-storage-type=aws-s3;s3-bucket=%s;s3-region=%s;"+
			"s3-profile=%s;s3-endpoint=%s;path=%s;modified-after=%s;modified-before=%s",
		serverNamespace, jobID, s3Bucket, s3Region, s3Profile, s.s3Endpoint, routinePath, after, before))
	s.Require().False(strings.HasPrefix(answer, "ERROR"), "backup start: %s", answer)

	s.Require().True(s.eventually(serverJobTimeout, pollInterval, func() bool {
		return len(statusRows(s.info(s.client, "backup-status:job-id="+jobID))) == 1
	}), "job %s not in backup-status", jobID)
	row := s.backupJobFields(jobID)
	if _, ok := row["path"]; !ok {
		s.T().Skip("SERVER-2684: this server build does not report where a backup job writes")
	}
	s.Equal(s3Bucket, row["s3-bucket"])
	s.Equal(routinePath+"/", row["path"], "the server ends a path with /")
	s.Equal(after, row["modified-after"])
	s.Equal(before, row["modified-before"])

	var logged map[string]any
	s.Require().True(s.eventually(time.Minute, pollInterval, func() bool {
		logged = loggedJob(s.readFile(logFile), jobID)
		return logged != nil
	}), "the poll did not log job %s", jobID)
	s.Equal(routineName, logged["routine"])
	s.Equal(s3Bucket, logged["bucket"])
	s.Equal(routinePath+"/", logged["path"])
	s.Equal("incremental", logged["type"])
}

// loggedJob returns the attributes of the first "Server backup job" record of jobID in the JSON
// log, or nil if there is none.
func loggedJob(log []byte, jobID string) map[string]any {
	lines := bufio.NewScanner(bytes.NewReader(log))
	for lines.Scan() {
		var record map[string]any
		if json.Unmarshal(lines.Bytes(), &record) == nil &&
			record["msg"] == "Server backup job" && record["jobId"] == jobID {
			return record
		}
	}

	return nil
}

// readFile returns the content of file, or nothing while it does not exist.
func (s *ServerBackupSuite) readFile(file string) []byte {
	content, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil
	}
	s.Require().NoError(err)

	return content
}
