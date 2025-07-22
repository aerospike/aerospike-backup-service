package decoder

import (
	"log/slog"

	_ "github.com/aerospike/aerospike-backup-service/v3/docs" // need to ensure swagger is initialized before.
	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/swaggo/swag"
)

var (
	// all fields by struct name.
	fieldsByStruct = make(map[string][]string)
	// all fields in the spec.
	allFields []string
)

func init() {
	swaggerDoc, err := swag.ReadDoc("swagger")
	if err != nil {
		slog.Warn("Failed to read swagger doc", attr.Error(err))
		return
	}
	fieldsByStruct, err = parseOpenAPISpec(swaggerDoc)
	if err != nil {
		slog.Warn("Failed to parse swagger doc", attr.Error(err))
		return
	}

	allFields = flattenDistinct(fieldsByStruct)
}

func flattenDistinct(fields map[string][]string) []string {
	unique := map[string]struct{}{}
	for _, values := range fields {
		for _, v := range values {
			unique[v] = struct{}{}
		}
	}
	return keys(unique)
}

func keys(m map[string]struct{}) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	return k
}
