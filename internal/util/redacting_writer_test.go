package util

import (
	"bytes"
	"testing"
)

func TestRedactingWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := newRedactingWriter(&buf)

	input := `{"user":"alice","privateKey":"-----BEGIN PRIVATE KEY-----abc123-----END PRIVATE KEY-----"}`
	expected := `{"user":"alice","privateKey":"[REDACTED]"}`

	_, err := writer.Write([]byte(input))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	output := buf.String()
	if output != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, output)
	}
}
