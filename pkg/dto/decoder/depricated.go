package decoder

// Map of known deprecated fieldsByStruct with their replacement info.
//
//nolint:lll
var deprecatedFields = map[string]map[string]string{
	"dto.BackupPolicy": {
		"max-retries":  "is deprecated. Use retry-policy (see https://aerospike.com/docs/tools/backup-service/configuration#backup-policies.POLICY_NAME.retry-policy )",
		"retry-delay":  "is deprecated. Use retry-policy (see https://aerospike.com/docs/tools/backup-service/configuration#backup-policies.POLICY_NAME.retry-policy )",
		"remove-files": "is deprecated. Use retention (see https://aerospike.com/docs/tools/backup-service/configuration#backup-policies.POLICY_NAME.retention )",
		"namespace":    "not supported. Use namespaces (see https://aerospike.com/docs/tools/backup-service/configuration#backup-routines.ROUTINE_NAME.namespaces )",
	},
	"dto.Storage": {
		"type": "is deprecated, use nested storage objects instead (see https://aerospike.com/docs/tools/backup-service/release-notes/3.0.0-abs-release-notes#storage )",
	},
}
