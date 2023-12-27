package util

import (
	"strings"
	"testing"
)

func Test_libLogRegex(t *testing.T) {
	tests := []string{
		"2023-01-01 00:00:00 UTC [INF] [74813] Starting backup for 4096 partitions from 0 to 4095",
		"2023-11-08 09:44:48 GMT [ERR] [   14] The max S3 object size is 5497558138880",
	}

	for _, e := range tests {
		match := libLogRegex.FindStringSubmatch(e)
		if len(match) != 5 {
			t.Fatal("libLogRegex error")
		}
	}
	LogCaptured(strings.Join(tests, "\n"))
}

func Test_ignoreLinesRegex(t *testing.T) {
	testStrings := []string{
		"time=2023-12-18T09:39:31.311Z level=ERROR source=/app/pkg/util/log.go:25 msg=\"[src/main/aerospike/as_pipe.c:210][read_file] Failed to open /proc/sys/net/core/rmem_max for reading\"",
		"[src/main/aerospike/as_pipe.c:269][get_buffer_size] Failed to read /proc/sys/net/core/rmem_max; should be at least 15728640. Please verify.",
		"Failed to open /proc/sys/net/core/wmem_max for reading",
		"Failed to read /proc/sys/net/core/wmem_max",
	}

	for _, testString := range testStrings {
		matches := matchesAnyPattern(testString, ignoredLinesInDocker)
		if !matches {
			t.Error("test string expected to match", "string", testString)
		}
	}
}
