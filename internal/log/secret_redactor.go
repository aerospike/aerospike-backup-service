package log

import (
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
)

// RedactSecretsAttr is a slog ReplaceAttr filter that deep-walks each attribute
// value and replaces every dto.SecretString it finds with a redacted marker.
//
// It works on the typed attribute graph before slog formats it, so it is
// immune to the false positives and escaping problems of byte-stream redaction.
// Because it keys on the SecretString type rather than field names or tags, any
// new secret field is protected automatically as long as it uses SecretString.
//
// Redaction reuses dto.RedactCopy so the sink and any self-serializing LogValue
// (e.g. dto.Config.LogValue) share a single redaction implementation.
func RedactSecretsAttr(_ []string, a slog.Attr) slog.Attr {
	v := a.Value.Any()
	if v == nil {
		return a
	}

	redacted := dto.RedactCopy(v)
	return slog.Any(a.Key, redacted)
}
