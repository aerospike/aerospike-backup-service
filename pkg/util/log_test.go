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
