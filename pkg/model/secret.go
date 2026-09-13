package model

import "github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"

// Secret marks a credential field. It is the same type the DTO layer uses, so a value
// keeps its redaction (String, GoString, LogValue and the slog ReplaceAttr walk) after it
// is converted to the model. Read the literal with Reveal() and hash it with Hash();
// fmt and plain string conversion are either redacted or invisible to a code search.
type Secret = decoder.Secret
