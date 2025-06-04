package util

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactingWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := newRedactingWriter(&buf)

	input := `{"user":"alice","privateKey":"-----BEGIN PRIVATE KEY-----abc123-----END PRIVATE KEY-----"}`
	expected := `{"user":"alice","privateKey":"[REDACTED]"}`

	_, err := writer.Write([]byte(input))
	assert.NoError(t, err)
	assert.Equal(t, expected, buf.String())
}

func TestRedactingWriter_NoRedactionNeeded(t *testing.T) {
	var buf bytes.Buffer
	writer := newRedactingWriter(&buf)

	input := `{"user":"alice","role":"admin"}`

	_, err := writer.Write([]byte(input))
	assert.NoError(t, err)
	assert.Equal(t, input, buf.String())
}
