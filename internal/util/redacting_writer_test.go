package util

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactingWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := newRedactingWriter(&buf)

	input := `{"user":"alice","privateKey":"-----BEGIN PRIVATE KEY-----abc123-----END PRIVATE KEY-----"}`
	expected := fmt.Sprintf(`{"user":"alice","privateKey":"%s"}`, redactedPlaceholder)

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

func BenchmarkLoggerComparison(b *testing.B) {
	runLineByLineLogging(b, "RedactingWriter_LineByLine", func(w io.Writer) io.Writer {
		return newRedactingWriter(w)
	})

	runLineByLineLogging(b, "PlainWriter_LineByLine", func(w io.Writer) io.Writer {
		return w
	})
}

//nolint:lll
func generateLogLine(i int, redact bool) []byte {
	if redact {
		return []byte(fmt.Sprintf("WARN %d: key found -----BEGIN PRIVATE KEY-----\nKEY_%d\n-----END PRIVATE KEY-----\n", i, i))
	}
	switch i % 5 {
	case 0:
		return []byte(fmt.Sprintf("INFO %d: user logged in from IP 192.168.%d.%d\n", i, i%255, (i+50)%255))
	case 1:
		return []byte(fmt.Sprintf("DEBUG %d: health check passed for service=db latency=%dms\n", i, i%100))
	case 2:
		return []byte(fmt.Sprintf("WARN %d: unusual login time detected for user_id=%d\n", i, i*3))
	case 3:
		return []byte(fmt.Sprintf("TRACE %d: payload received {\"req_id\":%d,\"action\":\"ping\"}\n", i, 10000+i))
	case 4:
		return []byte(fmt.Sprintf("ERROR %d: failed to decode stream: %%x=0x%X ☠️\n", i, i*7))
	default:
		return []byte(fmt.Sprintf("line %d: fallback log line\n", i))
	}
}

const (
	totalLogLines      = 100_000 // total log calls per benchmark iteration
	redactEveryNthLine = 100     // redact pattern every N lines
)

func runLineByLineLogging(b *testing.B, name string, wrapWriter func(io.Writer) io.Writer) {
	b.Run(name, func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			f, err := os.CreateTemp("", name+"_*.log")
			if err != nil {
				b.Fatal(err)
			}
			defer os.Remove(f.Name())

			writer := wrapWriter(f)

			for j := 0; j < totalLogLines; j++ {
				line := generateLogLine(j, j%redactEveryNthLine == 0)
				_, err := writer.Write(line)
				if err != nil {
					b.Fatal(err)
				}
			}

			f.Close()
		}
	})
}
