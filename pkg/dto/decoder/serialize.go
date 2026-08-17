package decoder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Marshal serializes v to JSON or YAML bytes.
// When redact is true, Secret-typed fields are replaced before serialization.
func Marshal(v any, format SerializationFormat, redact bool) ([]byte, error) {
	if v == nil {
		return nil, errors.New("nil value")
	}

	if redact {
		v = RedactSecrets(v)
	}

	switch format {
	case JSON:
		return json.Marshal(v)
	case YAML:
		return yaml.Marshal(v)
	default:
		return nil, fmt.Errorf("unsupported format: %v", format)
	}
}

// Serialize writes v to w as JSON or YAML.
// When redact is true, Secret-typed fields are replaced before serialization.
func Serialize(w io.Writer, v any, format SerializationFormat, redact bool) error {
	if w == nil {
		return errors.New("nil writer")
	}

	if v == nil {
		return errors.New("nil value")
	}

	if redact {
		v = RedactSecrets(v)
	}

	switch format {
	case JSON:
		return json.NewEncoder(w).Encode(v)
	case YAML:
		data, err := yaml.Marshal(v)
		if err != nil {
			return err
		}

		_, err = w.Write(data)

		return err
	default:
		return fmt.Errorf("unsupported format: %v", format)
	}
}
