package util

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"log/slog"
)

var libLogRegex = regexp.MustCompile(`^(.+)\s\[(\D+)\]\s\[\s*(\d+)\]\s(.*)$`)
var statsRegex = regexp.MustCompile(`Backed up (\d+) record\(s\), (\d+) secondary index\(es\), (\d+) UDF file\(s\)`)
var pathRegex = regexp.MustCompile(`\) to (\S+)`)

// LogCaptured logs the captured std output from the shared libraries.
func LogCaptured(out string) {
	if out == "" {
		slog.Debug("No logs captured")
		return
	}
	entries := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	for _, entry := range entries {
		if groups := libLogRegex.FindStringSubmatch(entry); len(groups) == 5 {
			switch groups[2] {
			case "ERR":
				slog.Error(groups[4])
			case "INF":
				slog.Info(groups[4])
			default:
				slog.Debug(groups[4])
			}
		} else { // print to stderr
			fmt.Fprintln(os.Stderr, entry)
		}
	}
}

// BackupInfo struct to hold the extracted numbers
type BackupInfo struct {
	RecordCount         int
	SecondaryIndexCount int
	UDFFileCount        int
	Path                string
}

//2023-11-29 07:43:39 UTC [INF] [39904] Starting backup of localhost (namespace: source-ns1, set: [set1], bins: [all], after: 2023-11-29 09:43:30 IST, before: 1970-01-01 02:00:00 IST, no ttl only: false, limit: 0) to ./testout/incremental/1701243819063.asb
//2023-11-29 07:43:39 UTC [INF] [39904] Processing 1 node(s)
//2023-11-29 07:43:39 UTC [INF] [39904] Node ID             Objects        Replication
//2023-11-29 07:43:39 UTC [INF] [39904] BB9020011AC4202     0              1
//2023-11-29 07:43:39 UTC [INF] [39904] Set set1 contains 0 record(s)
//2023-11-29 07:43:39 UTC [INF] [39949] No secondary indexes
//2023-11-29 07:43:39 UTC [INF] [39949] Backing up 0 UDF file(s)
//2023-11-29 07:43:39 UTC [INF] [39949] Starting backup for 4096 partitions from 0 to 4095
//2023-11-29 07:43:39 UTC [INF] [39949] Completed backup for 4096 partitions from 0 to 4095, records: 0, size: 0 (~0 B/rec)
//2023-11-29 07:43:39 UTC [INF] [39904] Backed up 0 record(s), 0 secondary index(es), 0 UDF file(s), 48 byte(s) in total (~0 B/rec)

func ExtractBackupStats(input string) (*BackupInfo, error) {
	matches := statsRegex.FindStringSubmatch(input)

	if len(matches) != 4 {
		return nil, fmt.Errorf("no match found for backup info")
	}

	recordCount, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, err
	}

	secondaryIndexCount, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, err
	}

	udfFileCount, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, err
	}

	// Find matches for Path info
	pathMatches := pathRegex.FindStringSubmatch(input)

	// Check if there are matches
	if len(pathMatches) != 2 {
		return nil, fmt.Errorf("no match found for Path info")
	}

	// Extract Path
	path := pathMatches[1]

	return &BackupInfo{
		RecordCount:         recordCount,
		SecondaryIndexCount: secondaryIndexCount,
		UDFFileCount:        udfFileCount,
		Path:                path,
	}, nil
}
