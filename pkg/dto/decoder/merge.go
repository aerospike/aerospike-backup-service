package decoder

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// errNoStoredSecret reports a redaction placeholder with no stored value behind it to keep.
var errNoStoredSecret = errors.New(`"[secret]" is the API's redaction placeholder and has no stored value to keep`)

// MergeSecrets mutates incoming in place, replacing redact.Placeholder sentinel values
// with the corresponding Secret values from existing.
//
// It walks incoming recursively and, for every Secret field equal to redact.Placeholder,
// copies the value from the matching field in existing.
//
// Returns an error if a redacted Secret is found in incoming but there is no
// corresponding existing value to restore from.
//
// incoming is a DTO tree freshly decoded from a request or built from the stored model, so
// the walk assumes it is acyclic;
//
// Example:
//
// Before:
// Incoming:
//
//	{
//	  "clusters": {
//	    "c1": {
//	      "password": "[secret]",
//	      "user": "new-user"
//	    }
//	  }
//	}
//
// Existing:
//
//	{
//	  "clusters": {
//	    "c1": {
//	      "password": "real-password"
//	    }
//	  }
//	}
//
// Incoming After:
//
//	{
//	  "clusters": {
//	    "c1": {
//	      "password": "real-password",
//	      "user": "new-user"
//	    }
//	  }
//	}
func MergeSecrets(incoming, existing any) error {
	if incoming == nil || existing == nil {
		return nil
	}

	return mergeValue(reflect.ValueOf(incoming), reflect.ValueOf(existing), "")
}

// mergeValue merges existing into incoming
//
//nolint:gocognit,gocyclo,funlen // recursive reflect walk over nested DTO values
func mergeValue(incoming, existing reflect.Value, path string) error {
	if !incoming.IsValid() {
		return nil
	}

	// A value read through an unexported field is read-only: reflect will neither hand it out
	// as an interface nor let anything be written to it, so there is no secret to merge into
	// it. This is what keeps types that hold private state - time.Time, time.Location, a
	// wrapped error - out of the walk's way without naming any of them.
	if !incoming.CanInterface() {
		return nil
	}

	if r, ok := asRedactable(incoming); ok {
		return setSecretValue(incoming, r, existing, path)
	}

	switch incoming.Kind() {
	case reflect.Pointer:
		if incoming.IsNil() {
			return nil
		}

		incomingElem := incoming.Elem()
		if existing.Kind() == reflect.Pointer {
			if !existing.IsValid() {
				return mergeValue(incomingElem, reflect.Value{}, path)
			}

			if existing.IsNil() {
				return mergeValue(incomingElem, reflect.Value{}, path)
			}

			return mergeValue(incomingElem, existing.Elem(), path)
		}

		if !existing.IsValid() {
			return mergeValue(incomingElem, reflect.Value{}, path)
		}

		return mergeValue(incomingElem, existing, path)

	case reflect.Interface:
		if incoming.IsNil() {
			return nil
		}

		if !existing.IsValid() {
			return nil
		}

		if existing.IsNil() {
			return nil
		}

		return mergeValue(incoming.Elem(), existing.Elem(), path)

	case reflect.Struct:
		if existing.IsValid() && (existing.Kind() != reflect.Struct || incoming.Type() != existing.Type()) {
			return nil
		}

		for i := 0; i < incoming.NumField(); i++ {
			var existingField reflect.Value
			if existing.IsValid() {
				existingField = existing.Field(i)
			}
			if err := mergeValue(incoming.Field(i), existingField, fieldPath(path, incoming.Type().Field(i))); err != nil {
				return err
			}
		}

		return nil

	case reflect.Map:
		if existing.Kind() != reflect.Map || incoming.Type() != existing.Type() {
			return nil
		}

		for _, key := range incoming.MapKeys() {
			incomingVal := incoming.MapIndex(key)
			existingVal := existing.MapIndex(key)
			keyPath := joinPath(path, fmt.Sprint(key.Interface()))

			if incomingVal.Kind() != reflect.Pointer && !incomingVal.CanSet() {
				mutable := reflect.New(incomingVal.Type()).Elem()
				mutable.Set(incomingVal)
				if err := mergeValue(mutable, existingVal, keyPath); err != nil {
					return err
				}
				incoming.SetMapIndex(key, mutable)
				continue
			}

			if err := mergeValue(incomingVal, existingVal, keyPath); err != nil {
				return err
			}
		}

		return nil

	case reflect.Slice:
		if existing.Kind() != reflect.Slice {
			return nil
		}

		n := min(existing.Len(), incoming.Len())

		for i := range n {
			if err := mergeValue(incoming.Index(i), existing.Index(i), path); err != nil {
				return err
			}
		}

		return nil

	case reflect.Array:
		if existing.Kind() != reflect.Array || incoming.Type() != existing.Type() {
			return nil
		}

		for i := 0; i < incoming.Len(); i++ {
			if err := mergeValue(incoming.Index(i), existing.Index(i), path); err != nil {
				return err
			}
		}
		return nil

	default:
		return nil
	}
}

// setSecretValue restores the credential behind incoming, which asRedactable read as inSecret,
// from existing, when incoming is the redaction placeholder.
func setSecretValue(incoming reflect.Value, inSecret redact.Redactable, existing reflect.Value, path string) error {
	if !inSecret.IsRedacted() || !incoming.CanSet() {
		return nil
	}

	if !existing.IsValid() || existing.Type() != incoming.Type() {
		return fmt.Errorf("%s: %w", path, errNoStoredSecret)
	}

	existingSecret, ok := asRedactable(existing)
	if !ok || existingSecret.IsRedacted() {
		return fmt.Errorf("%s: %w", path, errNoStoredSecret)
	}

	incoming.Set(existing)

	return nil
}

// fieldPath returns the path of field f of the struct at path, named by its YAML tag, else its
// JSON one. A field without a name - inlined or untagged - shares its parent's path.
func fieldPath(path string, f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" {
		tag = f.Tag.Get("json")
	}

	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return path
	}

	return joinPath(path, name)
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}
