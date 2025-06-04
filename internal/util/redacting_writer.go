package util

import (
	"io"
	"regexp"
)

const redactedPlaceholder = "[REDACTED]"

var privateKeyRegex = regexp.MustCompile(`-----BEGIN PRIVATE KEY-----[\s\S]+?-----END PRIVATE KEY-----`)

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
	return rw.underlying.Write(output)
}
