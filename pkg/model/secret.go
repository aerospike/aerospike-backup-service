package model

import "github.com/aerospike/aerospike-backup-service/v3/pkg/redact"

// Secret is the credential type for every sensitive field in this package. Declaring a field
// with it is what marks the value as secret-bearing: it then redacts itself in logs and fmt
// output, and the literal is read only through Reveal.
//
// It is an alias rather than a defined type, so it stays identical to redact.Secret. The
// reflective walks in pkg/dto/decoder recognize it, and moving a value between a DTO and a
// model costs no conversion.
type Secret = redact.Secret
