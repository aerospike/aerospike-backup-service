// Generic helpers for slog.LogValuer implementations on DTO types.
//
// Convention: types that may hold secrets implement LogValue on the struct in the
// same file as the type definition. Type-specific log attr builders live with their
// types (e.g. secret agent attrs in secret_agent.go). When you add a struct field:
//   - update LogValue to include it if the value is safe to log;
//   - never log raw secrets — use appendRedactedText or appendRedactedTextPtr
//     (they emit logRedactedPlaceholder) for passwords, keys, tokens, and paths
//     that locate secret material.
//
// If LogValue omits a field, it will not appear in structured logs.
package dto

import (
	"log/slog"
)

const logRedactedPlaceholder = "[REDACTED]"

func appendStringPtr(attrs []slog.Attr, key string, value *string) []slog.Attr {
	if value != nil {
		return append(attrs, slog.String(key, *value))
	}

	return attrs
}

func appendString(attrs []slog.Attr, key, value string) []slog.Attr {
	if value != "" {
		return append(attrs, slog.String(key, value))
	}

	return attrs
}

func appendIntPtr(attrs []slog.Attr, key string, value *int) []slog.Attr {
	if value != nil {
		return append(attrs, slog.Int(key, *value))
	}

	return attrs
}

func appendInt64Ptr(attrs []slog.Attr, key string, value *int64) []slog.Attr {
	if value != nil {
		return append(attrs, slog.Int64(key, *value))
	}

	return attrs
}

func appendBoolPtr(attrs []slog.Attr, key string, value *bool) []slog.Attr {
	if value != nil {
		return append(attrs, slog.Bool(key, *value))
	}

	return attrs
}

func appendRedactedTextPtr(attrs []slog.Attr, key string, value *string) []slog.Attr {
	if hasText(value) {
		return append(attrs, slog.String(key, logRedactedPlaceholder))
	}

	return attrs
}

func appendRedactedText(attrs []slog.Attr, key, value string) []slog.Attr {
	if value != "" {
		return append(attrs, slog.String(key, logRedactedPlaceholder))
	}

	return attrs
}

func appendNamedLogGroup(attrs []slog.Attr, key string, named []slog.Attr) []slog.Attr {
	if len(named) == 0 {
		return attrs
	}

	return append(attrs, slog.Attr{Key: key, Value: slog.GroupValue(named...)})
}

func slogAnyMap[T slog.LogValuer](items map[string]T) []slog.Attr {
	var attrs []slog.Attr
	for name, item := range items {
		if any(item) == nil {
			continue
		}
		attrs = append(attrs, slog.Any(name, item))
	}

	return attrs
}
