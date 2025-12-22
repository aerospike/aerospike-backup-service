package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactingWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := newRedactingWriter(&buf)

	input := `{"user":"alice","privateKey":"-----BEGIN PRIVATE KEY-----abc123-----END PRIVATE KEY-----"}`
	expected := fmt.Sprintf(`{"user":"alice","privateKey":"%s"}`, redactedPlaceholder)

	_, err := writer.Write([]byte(input))
	require.NoError(t, err)
	require.Equal(t, expected, buf.String())
}

func TestRedactingWriter_MultilineKey(t *testing.T) {
	var buf bytes.Buffer
	writer := newRedactingWriter(&buf)

	input := `{"user":"alice","privateKey":"-----BEGIN PRIVATE KEY-----
abc123
-----END PRIVATE KEY-----"}`
	expected := fmt.Sprintf(`{"user":"alice","privateKey":"%s"}`, redactedPlaceholder)

	_, err := writer.Write([]byte(input))
	require.NoError(t, err)
	require.Equal(t, expected, buf.String())
}

func TestRedactingWriter_NoRedactionNeeded(t *testing.T) {
	var buf bytes.Buffer
	writer := newRedactingWriter(&buf)

	input := `{"user":"alice","role":"admin"}`

	_, err := writer.Write([]byte(input))
	require.NoError(t, err)
	require.Equal(t, input, buf.String())
}

const (
	totalLogLines      = 100_000 // total log calls per benchmark iteration
	redactEveryNthLine = 100     // redact pattern every N lines
)

func BenchmarkLoggerComparison(b *testing.B) {
	runLineByLineLogging(b, "RedactingWriter", newRedactingWriter)

	runLineByLineLogging(b, "PlainWriter", func(w io.Writer) io.Writer {
		return w
	})
}

func generateLogLine(i int) []byte {
	if i%redactEveryNthLine == 0 {
		return fmt.Appendf(nil, "WARN %d: key found -----BEGIN PRIVATE KEY-----\nKEY_%d\n-----END PRIVATE KEY-----\n", i, i)
	}
	switch i % 5 {
	case 0:
		return fmt.Appendf(nil, "INFO %d: user logged in from IP 192.168.%d.%d\n", i, i%255, (i+50)%255)
	case 1:
		return fmt.Appendf(nil, "DEBUG %d: health check passed for service=db latency=%dms\n", i, i%100)
	case 2:
		return fmt.Appendf(nil, "WARN %d: unusual login time detected for user_id=%d\n", i, i*3)
	case 3:
		return fmt.Appendf(nil, "TRACE %d: payload received {\"req_id\":%d,\"action\":\"ping\"}\n", i, 10000+i)
	case 4:
		return fmt.Appendf(nil, "ERROR %d: failed to decode stream: %%x=0x%X ☠️\n", i, i*7)
	default:
		return fmt.Appendf(nil, "line %d: fallback log line\n", i)
	}
}

func runLineByLineLogging(b *testing.B, name string, wrapWriter func(io.Writer) io.Writer) {
	b.Helper()

	b.Run(name, func(b *testing.B) {
		for range b.N {
			f, err := os.CreateTemp("", name+"_*.log")
			if err != nil {
				b.Fatal(err)
			}

			writer := wrapWriter(f)

			for line := range totalLogLines {
				line := generateLogLine(line)
				_, err := writer.Write(line)
				if err != nil {
					b.Fatal(err)
				}
			}

			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	})
}
