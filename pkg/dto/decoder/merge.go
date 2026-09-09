package decoder

import (
	"errors"
	"reflect"
)

// MergeSecrets mutates incoming in place, replacing redactedSecret sentinel values
// with the corresponding Secret values from existing.
//
// It walks incoming recursively and, for every Secret field equal to redactedSecret,
// copies the value from the matching field in existing.
//
// Returns an error if a redacted Secret is found in incoming but there is no
// corresponding existing value to restore from.
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

	return mergeValue(reflect.ValueOf(incoming), reflect.ValueOf(existing))
}

//nolint:gocognit,gocyclo,funlen // recursive reflect walk over nested DTO values
func mergeValue(incoming, existing reflect.Value) error {
	if !incoming.IsValid() {
		return nil
	}

	if incoming.Type() == secretType {
		return setSecretValue(incoming, existing)
	}

	if shouldSkipDeepCopy(incoming) {
		return nil
	}

	switch incoming.Kind() {
	case reflect.Pointer:
		if incoming.IsNil() {
			return nil
		}

		incomingElem := incoming.Elem()
		if existing.Kind() == reflect.Pointer {
			if !existing.IsValid() {
				return mergeValue(incomingElem, reflect.Value{})
			}

			if existing.IsNil() {
				return mergeValue(incomingElem, reflect.Value{})
			}

			return mergeValue(incomingElem, existing.Elem())
		}

		if !existing.IsValid() {
			return mergeValue(incomingElem, reflect.Value{})
		}

		return mergeValue(incomingElem, existing)

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

		return mergeValue(incoming.Elem(), existing.Elem())

	case reflect.Struct:
		if existing.IsValid() && (existing.Kind() != reflect.Struct || incoming.Type() != existing.Type()) {
			return nil
		}

		for i := 0; i < incoming.NumField(); i++ {
			var existingField reflect.Value
			if existing.IsValid() {
				existingField = existing.Field(i)
			}
			if err := mergeValue(incoming.Field(i), existingField); err != nil {
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

			if incomingVal.Kind() != reflect.Pointer && !incomingVal.CanSet() {
				mutable := reflect.New(incomingVal.Type()).Elem()
				mutable.Set(incomingVal)
				if err := mergeValue(mutable, existingVal); err != nil {
					return err
				}
				incoming.SetMapIndex(key, mutable)
				continue
			}

			if err := mergeValue(incomingVal, existingVal); err != nil {
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
			if err := mergeValue(incoming.Index(i), existing.Index(i)); err != nil {
				return err
			}
		}

		return nil

	case reflect.Array:
		if existing.Kind() != reflect.Array || incoming.Type() != existing.Type() {
			return nil
		}

		for i := 0; i < incoming.Len(); i++ {
			if err := mergeValue(incoming.Index(i), existing.Index(i)); err != nil {
				return err
			}
		}
		return nil

	default:
		return nil
	}
}

func setSecretValue(incoming reflect.Value, existing reflect.Value) error {
	inSecret, ok := incoming.Interface().(Secret)
	if !ok || !inSecret.IsRedacted() {
		return nil
	}

	if !incoming.CanSet() {
		return nil
	}

	if !existing.IsValid() || existing.Type() != secretType {
		return errors.New("cannot use redacted secret \"[secret]\" for a new entity with no existing value")
	}

	existingSecret := existing.Interface().(Secret)
	if existingSecret.IsRedacted() {
		return errors.New("cannot use redacted secret \"[secret]\" for a new entity with no existing value")
	}

	incoming.Set(existing)

	return nil
}
