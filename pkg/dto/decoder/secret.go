package decoder

import "log/slog"

const RedactedSecret = "[secret]"

// Secret marks a string field as sensitive for redaction during API responses and logging.
type Secret string

// String redacts fmt "%s" and "%v" output. Empty secrets stay empty so logs do not
// imply a password is set when it is not.
func (s Secret) String() string {
	if s == "" {
		return ""
	}

	return RedactedSecret
}

// GoString redacts fmt "%#v" output used in debug prints and some test failure messages.
func (s Secret) GoString() string {
	if s == "" {
		return "decoder.Secret(\"\")"
	}

	return `decoder.Secret("[secret]")`
}

// LogValue redacts slog.Any("password", secret) without requiring ReplaceAttr on the handler.
func (s Secret) LogValue() slog.Value {
	if s == "" {
		return slog.StringValue("")
	}

	return slog.StringValue(RedactedSecret)
}
