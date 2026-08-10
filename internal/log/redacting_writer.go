package log

import (
	"io"
	"regexp"
)

const redactedPlaceholder = "[REDACTED]"

var privateKeyRegex = regexp.MustCompile(`(?s)-----BEGIN PRIVATE KEY.*?END PRIVATE KEY-----`)
var patterns = []*regexp.Regexp{
	privateKeyRegex,
}

type redactingWriter struct {
	underlying io.Writer
}

func newRedactingWriter(w io.Writer) io.Writer {
	return &redactingWriter{
		underlying: w,
	}
}

func (rw *redactingWriter) Write(p []byte) (int, error) {
	output := p
	for _, re := range patterns {
		output = re.ReplaceAll(output, []byte(redactedPlaceholder))
	}

	// The redacted output may differ in length from p, but io.Writer callers
	// expect the returned count to refer to the original slice. Report len(p)
	// on a full underlying write so slog does not treat it as a short write.
	if _, err := rw.underlying.Write(output); err != nil {
		return 0, err
	}

	return len(p), nil
}
