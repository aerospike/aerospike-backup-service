package decoder

import (
	"reflect"
)

// MergeSecrets mutates incoming in place, replacing RedactedSecret sentinel values
// with the corresponding Secret values from existing.
//
// It walks incoming recursively and, for every Secret field equal to RedactedSecret,
// copies the value from the matching field in existing.
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
func MergeSecrets(incoming, existing any) {
	if incoming == nil || existing == nil {
		return
	}

	mergeValue(reflect.ValueOf(incoming), reflect.ValueOf(existing))
}

//nolint:gocognit,gocyclo,funlen // recursive reflect walk over nested DTO values
func mergeValue(incoming, existing reflect.Value) {
	if !incoming.IsValid() || !existing.IsValid() {
		return
	}

	if incoming.Type() == secretType {
		inSecret, ok := incoming.Interface().(Secret)
		if !ok || !inSecret.IsRedacted() {
			return
		}

		if existing.Type() != secretType || !incoming.CanSet() {
			return
		}

		incoming.Set(existing)

		return
	}

	if incoming.Type() == timeType {
		return
	}

	switch incoming.Kind() {
	case reflect.Pointer:
		if incoming.IsNil() {
			return
		}

		incomingElem := incoming.Elem()
		if existing.Kind() == reflect.Pointer {
			if existing.IsNil() {
				return
			}

			mergeValue(incomingElem, existing.Elem())
			return
		}

		mergeValue(incomingElem, existing)

	case reflect.Interface:
		if incoming.IsNil() {
			return
		}

		if existing.IsNil() {
			return
		}

		mergeValue(incoming.Elem(), existing.Elem())

	case reflect.Struct:
		if existing.Kind() != reflect.Struct || incoming.Type() != existing.Type() {
			return
		}

		for i := 0; i < incoming.NumField(); i++ {
			mergeValue(incoming.Field(i), existing.Field(i))
		}

	case reflect.Map:
		if existing.Kind() != reflect.Map || incoming.Type() != existing.Type() {
			return
		}

		for _, key := range incoming.MapKeys() {
			existingVal := existing.MapIndex(key)
			if !existingVal.IsValid() {
				continue
			}

			incomingVal := incoming.MapIndex(key)
			if incomingVal.Kind() != reflect.Pointer && !incomingVal.CanSet() {
				mutable := reflect.New(incomingVal.Type()).Elem()
				mutable.Set(incomingVal)
				mergeValue(mutable, existingVal)
				incoming.SetMapIndex(key, mutable)
				continue
			}

			mergeValue(incomingVal, existingVal)
		}

	case reflect.Slice:
		if existing.Kind() != reflect.Slice {
			return
		}

		n := min(existing.Len(), incoming.Len())

		for i := range n {
			mergeValue(incoming.Index(i), existing.Index(i))
		}

	case reflect.Array:
		if existing.Kind() != reflect.Array || incoming.Type() != existing.Type() {
			return
		}

		for i := 0; i < incoming.Len(); i++ {
			mergeValue(incoming.Index(i), existing.Index(i))
		}

	default:
		return
	}
}
