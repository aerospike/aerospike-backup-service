package doccheck_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/doccheck"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A default is declared twice: once as the `default:` struct tag on the DTO
// field, which is what the DTO tables, docs/openapi.json and
// docs/config.schema.json publish, and once in the code that fills the value in
// when the field is omitted. Two declarations of the same fact drift, and the
// drift is invisible — the published number is simply wrong, and the only way to
// find out is to run the service and look.
//
// The table below is the bridge between the two. It never repeats a value: every
// entry reads the applied default out of the code that decides it — the
// ...OrDefault accessor, the library that fills the gap when a field is left
// nil, or the zero value that is simply what downstream receives. So the only
// thing kept by hand is which code implements which tag; the values come from
// the code on every run.
//
// appliedDefault is one such bridge.
type appliedDefault struct {
	// applied renders the default the service applies, in the same form the tag uses.
	applied func() string
	// reason explains an entry that cannot be checked, and is empty otherwise.
	reason string
}

// applied maps "DTOType.Field" to the code that decides that field's default.
//
//nolint:gochecknoglobals // a table, read by the test below.
var applied = map[string]appliedDefault{
	// Fields the service substitutes a value for, read through the accessor
	// that does the substituting.
	"BackupPolicy.Parallel":             {applied: value((&model.BackupPolicy{}).GetParallelOrDefault())},
	"BackupPolicy.ParallelWrite":        {applied: value((&model.BackupPolicy{}).GetParallelWriteOrDefault())},
	"BackupPolicy.FileLimit":            {applied: value((&model.BackupPolicy{}).GetFileLimitOrDefault())},
	"BackupPolicy.SocketTimeout":        {applied: millis((&model.BackupPolicy{}).GetSocketTimeoutOrDefault())},
	"BackupPolicy.Sealed":               {applied: value((&model.BackupPolicy{}).IsSealedOrDefault())},
	"BackupPolicy.Compact":              {applied: value((&model.BackupPolicy{}).CompactOrDefault())},
	"BaseRestorePolicy.Parallel":        {applied: value((&model.RestorePolicy{}).GetParallelOrDefault())},
	"BaseRestorePolicy.MaxAsyncBatches": {applied: value((&model.RestorePolicy{}).GetMaxAsyncBatchesOrDefault())},
	"BaseRestorePolicy.BatchSize":       {applied: value((&model.RestorePolicy{}).GetBatchSizeOrDefault())},
	"BaseRestorePolicy.SocketTimeout":   {applied: millis((&model.RestorePolicy{}).GetSocketTimeoutOrDefault())},
	"Credentials.AuthMode":              {applied: value((&model.Credentials{}).AuthModeOrDefault())},
	"ServerConfigHTTP.Port":             {applied: value((&model.ServerConfigHTTP{}).GetPortOrDefault())},
	"ServerConfigHTTPS.Port":            {applied: value((&model.ServerConfigHTTPS{}).GetPortOrDefault())},
	"ServerConfigHTTPS.MinVersion":      {applied: value((&model.ServerConfigHTTPS{}).GetMinVersionOrDefault())},
	"ServerConfigHTTPS.ClientAuth":      {applied: value((&model.ServerConfigHTTPS{}).GetClientAuthOrDefault())},
	"ListenerConfig.Address":            {applied: value((&model.ListenerConfig{}).GetAddressOrDefault())},
	"ListenerConfig.ContextPath":        {applied: value((&model.ListenerConfig{}).GetContextPathOrDefault())},
	"ListenerConfig.Timeout":            {applied: millis((&model.ListenerConfig{}).GetTimeoutOrDefault())},
	"ListenerConfig.ReadTimeout":        {applied: millis((&model.ListenerConfig{}).GetReadTimeoutOrDefault())},
	"ListenerConfig.WriteTimeout":       {applied: millis((&model.ListenerConfig{}).GetWriteTimeoutOrDefault())},
	"ListenerConfig.IdleTimeout":        {applied: millis((&model.ListenerConfig{}).GetIdleTimeoutOrDefault())},
	"RateLimiterConfig.Tps":             {applied: value((&model.RateLimiterConfig{}).GetTpsOrDefault())},
	"RateLimiterConfig.Size":            {applied: value((&model.RateLimiterConfig{}).GetSizeOrDefault())},
	"LoggerConfig.Level":                {applied: value((&model.LoggerConfig{}).GetLevelOrDefault())},
	"LoggerConfig.Format":               {applied: value((&model.LoggerConfig{}).GetFormatOrDefault())},
	"LoggerConfig.StdoutWriter":         {applied: value((&model.LoggerConfig{}).GetStdoutWriterOrDefault())},
	"FileLoggerConfig.MaxSize":          {applied: value((&model.FileLoggerConfig{}).GetMaxSizeOrDefault())},
	"S3Storage.MinPartSize":             {applied: value((&model.S3Storage{}).GetPartSizeOrDefault())},
	"GcpStorage.MinPartSize":            {applied: value((&model.GcpStorage{}).GetPartSizeOrDefault())},
	"AzureStorage.MinPartSize":          {applied: value((&model.AzureStorage{}).GetPartSizeOrDefault())},

	// Fields a library decides. Left nil, the value never reaches the library's
	// config at all, so the library's own default is what an operator gets.
	"AerospikeCluster.ConnTimeout": {applied: millis(as.NewClientPolicy().Timeout)},
	// backup-go substitutes 1000ms for a nil timeout (secret_agent.go). It is a
	// literal there rather than an exported constant, so this is the one value
	// in the table that is copied: if backup-go changes it, nothing here notices.
	"SecretAgent.Timeout": {applied: value(1000)},

	// Fields nothing substitutes anything for. What reaches lumberjack or
	// backup-go is the zero value, so that is what the tag has to publish.
	"AerospikeCluster.UseServicesAlternate": {applied: value(false)},
	"BackupPolicy.TotalTimeout":             {applied: millis(0)},
	"BackupPolicy.NoRecords":                {applied: value(false)},
	"BackupPolicy.NoIndexes":                {applied: value(false)},
	"BackupPolicy.NoUdfs":                   {applied: value(false)},
	"BackupPolicy.WithClusterConfig":        {applied: value(false)},
	"BaseRestorePolicy.TotalTimeout":        {applied: millis(0)},
	"BaseRestorePolicy.NoRecords":           {applied: value(false)},
	"BaseRestorePolicy.NoIndexes":           {applied: value(false)},
	"BaseRestorePolicy.NoUdfs":              {applied: value(false)},
	"BaseRestorePolicy.DisableBatchWrites":  {applied: value(false)},
	"BaseRestorePolicy.Replace":             {applied: value(false)},
	"BaseRestorePolicy.Unique":              {applied: value(false)},
	"BaseRestorePolicy.NoGeneration":        {applied: value(false)},
	"BaseRestorePolicy.ExtraTTL":            {applied: value(0)},
	"BackupRoutine.Disabled":                {applied: value(false)},
	"CompressionPolicy.Level":               {applied: value(0)},
	"ListenerConfig.Disabled":               {applied: value(false)},
	"FileLoggerConfig.Compress":             {applied: value(false)},
	"SecretAgent.IsBase64":                  {applied: value(false)},

	// Modes whose default is the absence of the policy block: omitting it means
	// no compression and no encryption, which is exactly what NONE names. A
	// block that is present must state a valid mode, so this is the only case
	// in which the default applies.
	"CompressionPolicy.Mode":        {applied: value(model.CompressionModeNone)},
	"RestoreCompressionPolicy.Mode": {applied: value(model.CompressionModeNone)},
	"EncryptionPolicy.Mode":         {applied: value(model.EncryptionModeNone)},

	// Rotation settings that nothing applies, so lumberjack keeps every log
	// file forever. The field descriptions say so; the tags used to disagree.
	"FileLoggerConfig.MaxAge":     {applied: value(0)},
	"FileLoggerConfig.MaxBackups": {applied: value(0)},

	// Entries with no value to compare against.
	"LoggerConfig.FileWriter": {
		reason: "the tag is empty: it marks the section as optional rather than publishing a value",
	},
	"RestoreTimestampRequest.DisableReordering": {
		reason: "a request field consumed by restore_time_runner, not a configuration default",
	},
	"S3Storage.S3LogLevel": {
		reason: "model.S3Storage.S3LogLevel is read nowhere in pkg/service, so no default is " +
			"applied and there is nothing to compare the published FATAL against",
	},
}

// TestDefaultTagsMatchAppliedDefaults compares every published default against
// the one the service applies.
//
// It is the check that would have caught socket-timeout, where the tag publishes
// 60000 ms in three generated documents while the runtime substitutes ten
// minutes, and the file-logger rotation settings, which publish 7 days and 3
// backups while nothing ever applies them, so logs are in fact kept forever.
func TestDefaultTagsMatchAppliedDefaults(t *testing.T) {
	for _, tag := range doccheck.DefaultTags(t, doccheck.Root(t), "pkg/dto") {
		entry, ok := applied[tag.ID()]
		if !ok || entry.reason != "" {
			continue // covered by TestEveryDefaultTagIsAccountedFor.
		}

		t.Run(tag.ID(), func(t *testing.T) {
			assert.Equalf(t, entry.applied(), tag.Value,
				"%s publishes default:%q for %q, but the service applies %q",
				tag.Pos, tag.Value, tag.Key, entry.applied())
		})
	}
}

// TestEveryDefaultTagIsAccountedFor makes the table above self-maintaining: a
// field that grows a `default:` tag is not silently unchecked, it fails here
// until someone says which code applies it.
func TestEveryDefaultTagIsAccountedFor(t *testing.T) {
	tags := doccheck.DefaultTags(t, doccheck.Root(t), "pkg/dto")
	require.NotEmpty(t, tags, "no default tags found; the scan is broken, not the code")

	known := make(map[string]bool, len(tags))

	for _, tag := range tags {
		known[tag.ID()] = true

		assert.Containsf(t, applied, tag.ID(),
			"%s declares default:%q but internal/doccheck/defaults_test.go does not say "+
				"which code applies it; add an entry to the applied table",
			tag.Pos, tag.Value)
	}

	for id := range applied {
		assert.Truef(t, known[id],
			"the applied table has an entry for %s, which no longer declares a default tag; remove it",
			id)
	}
}

// value renders a default in the form the tag uses.
func value(v any) func() string {
	return func() string {
		if stringer, ok := v.(fmt.Stringer); ok {
			return stringer.String()
		}

		return fmt.Sprintf("%v", v)
	}
}

// millis renders a duration the way the tags express timeouts, in milliseconds.
func millis(d time.Duration) func() string {
	return func() string { return strconv.FormatInt(d.Milliseconds(), 10) }
}
