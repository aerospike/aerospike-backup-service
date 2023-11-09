package util

import (
	"testing"
)

func Test_libLogRegex(t *testing.T) {
	tests := []string{
		"2023-01-01 00:00:00 UTC [INF] [74813] Starting backup for 4096 partitions from 0 to 4095",
		"2023-11-08 09:44:48 GMT [ERR] [   14] The max S3 object size is 5497558138880"}

	for _, e := range tests {
		match := libLogRegex.FindStringSubmatch(e)
		if len(match) != 5 {
			t.Fatal("libLogRegex error")
		}
	}
}
