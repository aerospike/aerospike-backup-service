package model

import (
	"fmt"
	"hash/fnv"
)

func hashValues(values ...any) uint64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%v", values)

	return h.Sum64()
}
