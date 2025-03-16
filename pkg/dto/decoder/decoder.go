package decoder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// SerializationFormat represents the format for serialization/deserialization.
type SerializationFormat int

const (
	JSON SerializationFormat = iota
	YAML
)

// Deserialize handles deserialization.
func Deserialize(v any, r io.Reader, format SerializationFormat) error {
	if r == nil {
		return fmt.Errorf("nil reader")
	}
	if v == nil {
		return fmt.Errorf("nil target")
	}
	var err error

	switch format {
	case JSON:
		dec := json.NewDecoder(r)
		dec.DisallowUnknownFields() // Strict mode for JSON
		err = dec.Decode(v)
		if err != nil {
			return enhanceJSONError(err)
		}
	case YAML:
		dec := yaml.NewDecoder(r)
		dec.KnownFields(true) // Strict mode for YAML
		err = dec.Decode(v)
		if err != nil {
			return enhanceYamlErrors(err)
		}
	default:
		return fmt.Errorf("unsupported format: %v", format)
	}

	return nil
}

// enhanceJSONError enhances json unmarshaling errors.
func enhanceJSONError(jsonError error) error {
	field, err := parseJSONError(jsonError.Error())
	if err != nil {
		slog.Warn("Failed to parse JSON error message", slog.Any("err", err))
		return jsonError // Can't enhance this error, so keep the original error as-is
	}

	// Check every struct in deprecatedFields to find matching field
	for _, structFields := range deprecatedFields {
		if depInfo, ok := structFields[field]; ok {
			return fmt.Errorf("field %q %s", field, depInfo)
		}
	}

	// If not deprecated, look for similar fields across all structs
	suggestion := findSimilarField(field, allFields)
	if suggestion != "" {
		return fmt.Errorf("unknown field %q - did you mean: %q?", field, suggestion)
	}

	// If no suggestion was found, just return the original error
	return jsonError
}

func parseJSONError(errMsg string) (string, error) {
	// Example:
	// json: unknown field "seed-noodes"
	re := regexp.MustCompile(`json: unknown field "([^"]+)"`)
	matches := re.FindStringSubmatch(errMsg)

	if len(matches) < 2 { // No match found, return the original error
		return "", fmt.Errorf("failed to parse error message: %s", errMsg)
	}

	return matches[1], nil
}

// EnhanceYamlErrors enhances yaml.TypeErrors by
// * providing deprecation information
// * suggesting possible replacements.
func enhanceYamlErrors(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) { // Yaml parser is so nice that it return it's own error type.
		return err
	}

	errs := make([]error, 0, len(typeErr.Errors))
	for _, errMsg := range typeErr.Errors {
		errs = append(errs, processYamlError(errMsg))
	}

	return errors.Join(errs...)
}

func processYamlError(errMsg string) error {
	line, field, dtoName, err := parseYamlErrorMessage(errMsg)
	if err != nil {
		slog.Warn("Failed to parse YAML error message", slog.String("errMsg", errMsg), slog.Any("err", err))
		return err // Cannot enhance this error, so keep the original error as-is
	}

	if depInfo, ok := deprecatedFields[dtoName][field]; ok {
		return fmt.Errorf("line %d: field %q %s", line, field, depInfo)
	}

	// Find similar field names
	fieldsForStruct, ok := fieldsByStruct[dtoName]
	if ok {
		suggestion := findSimilarField(field, fieldsForStruct)
		if suggestion != "" {
			return fmt.Errorf("line %d: field %q not found - did you mean: %q?", line, field, suggestion)
		}
	}

	// If no suggestion was found, just return the original error
	return errors.New(errMsg)
}

// parseYamlErrorMessage parses an error message and returns the line number, field, and dto name.
func parseYamlErrorMessage(errMsg string) (line int, field string, dtoName string, err error) {
	// Regular expressions to match field error patterns
	// Example:
	// line 34: field retry-delay not found in type dto.BackupPolicy
	regex := regexp.MustCompile(`line (\d+):\s*field ([a-zA-Z0-9-_]+) not found in type (.+)`)

	match := regex.FindStringSubmatch(errMsg)
	if len(match) < 4 {
		err = fmt.Errorf("failed to parse error message: %s", errMsg)
		return
	}

	line, err = strconv.Atoi(match[1])
	if err != nil {
		err = fmt.Errorf("failed to convert line number to integer: %w", err)
		return
	}
	field = match[2]
	dtoName = match[3]

	return
}
