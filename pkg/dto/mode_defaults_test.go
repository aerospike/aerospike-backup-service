package dto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A client generated from docs/openapi.json sends every field that has a schema default, set to
// that default. The tests below build such a request in each backup-mode and expect it to pass:
// a field only one mode accepts must be tagged with x-backup-mode, which moves its default out of
// the schema's `default`.

func TestRestorePolicy_PublishedDefaultsPass(t *testing.T) {
	t.Parallel()

	policy := &RestorePolicy{}
	fillPublishedDefaults(t, reflect.ValueOf(policy).Elem())

	require.NoError(t, policy.Validate())
}

func TestConfig_PublishedDefaultsPassEveryMode(t *testing.T) {
	for _, mode := range []BackupMode{BackupModeScan, BackupModeServer} {
		t.Run(string(mode), func(t *testing.T) {
			config := validConfig()
			config.Storage["storage1"] = &Storage{S3Storage: &S3Storage{
				Bucket: "bucket", S3Region: "eu-central-1", Path: "backups",
			}}
			delete(config.BackupRoutines, "routine2")
			delete(config.BackupPolicies, "policy2")
			config.ServiceConfig.Backup = &BackupCommonConfig{BackupMode: mode}
			fillPublishedDefaults(t, reflect.ValueOf(config.BackupPolicies["policy1"]).Elem())
			fillPublishedDefaults(t, reflect.ValueOf(config.BackupRoutines["routine1"]).Elem())

			require.NoError(t, config.Validate())
		})
	}
}

// fillPublishedDefaults sets every field of v that the spec publishes a `default` for, the way a
// generated client does.
func fillPublishedDefaults(t *testing.T, v reflect.Value) {
	t.Helper()

	for i := range v.NumField() {
		field := v.Type().Field(i)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			fillPublishedDefaults(t, v.Field(i))
			continue
		}

		value, published := field.Tag.Lookup("default")
		if !published || value == "" || strings.Contains(field.Tag.Get("extensions"), "x-backup-mode") {
			continue
		}
		if field.Type.Kind() == reflect.String ||
			field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.String {
			value = `"` + value + `"`
		}
		require.NoErrorf(t, json.Unmarshal([]byte(value), v.Field(i).Addr().Interface()),
			"field %s", field.Name)
	}
}
