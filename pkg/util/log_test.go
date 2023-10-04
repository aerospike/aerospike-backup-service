package util

import (
	"testing"
)

func Test_libLogRegex(t *testing.T) {
	e := "2023-01-01 00:00:00 UTC [INF] [74813] Starting backup for 4096 partitions from 0 to 4095"
	match := libLogRegex.FindStringSubmatch(e)
	if len(match) != 5 {
		t.Fatal("libLogRegex error")
	}
}
