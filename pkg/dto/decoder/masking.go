package decoder

import (
	"log/slog"
	"reflect"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

var (
	redactableType  = reflect.TypeFor[redact.Redactable]()
	timeType        = reflect.TypeFor[time.Time]()
	timePtrType     = reflect.PointerTo(timeType)
	locationType    = reflect.TypeFor[time.Location]()
	locationPtrType = reflect.PointerTo(locationType)
	errType         = reflect.TypeFor[error]()

	skipDeepCopyTypes = []reflect.Type{
		timeType,
		timePtrType,
		locationType,
		locationPtrType,
	}
)

// RedactSecrets returns a deep copy of v with all Secret-typed values replaced by redact.Placeholder.
func RedactSecrets(v any) any {
	if v == nil {
		return nil
	}

	return redactValue(reflect.ValueOf(v)).Interface()
}

// RedactSecretsReplaceAttr returns a slog ReplaceAttr function that redacts Secret-typed values.
func RedactSecretsReplaceAttr() func(groups []string, a slog.Attr) slog.Attr {
	return func(_ []string, a slog.Attr) slog.Attr {
		return slog.Any(a.Key, RedactSecrets(a.Value.Any()))
	}
}

// isRedactable reports whether v is a credential-bearing string value that redacts itself.
// A Redactable that is not a string cannot hold its own DisplayString, so it is left to the
// ordinary walk, which still redacts every Secret nested inside it.
func isRedactable(v reflect.Value) bool {
	return v.Kind() == reflect.String && v.Type().Implements(redactableType)
}

// redactedValue replaces a credential with its safe display form, keeping the value's own
// type so it can be stored back into the field, map entry or slice element it came from.
func redactedValue(v reflect.Value) (reflect.Value, bool) {
	if !isRedactable(v) {
		return v, false
	}

	r, ok := v.Interface().(redact.Redactable)
	if !ok {
		return v, false
	}

	dst := reflect.New(v.Type()).Elem()
	dst.SetString(r.DisplayString())

	return dst, true
}

//nolint:gocognit,funlen // recursive reflect walk over nested DTO values
func redactValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	if redacted, ok := redactedValue(v); ok {
		return redacted
	}

	if shouldSkipDeepCopy(v) {
		return v
	}

	if v.Type().Implements(errType) {
		return v
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}

		dst := reflect.New(v.Elem().Type())
		dst.Elem().Set(redactValue(v.Elem()))

		return dst

	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}

		return redactValue(v.Elem())

	case reflect.Struct:
		dst := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			redacted := redactValue(v.Field(i))
			dstField := dst.Field(i)
			if dstField.CanSet() && redacted.IsValid() {
				setField(dstField, redacted)
			}
		}

		return dst

	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}

		dst := reflect.MakeMapWithSize(v.Type(), v.Len())
		for _, key := range v.MapKeys() {
			dst.SetMapIndex(key, redactValue(v.MapIndex(key)))
		}

		return dst

	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}

		dst := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			dst.Index(i).Set(redactValue(v.Index(i)))
		}

		return dst

	case reflect.Array:
		dst := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			dst.Index(i).Set(redactValue(v.Index(i)))
		}

		return dst

	default:
		return v
	}
}

func setField(dst, src reflect.Value) {
	if src.Type().AssignableTo(dst.Type()) {
		dst.Set(src)
		return
	}

	if src.Type().ConvertibleTo(dst.Type()) {
		dst.Set(src.Convert(dst.Type()))
	}
}

func shouldSkipDeepCopy(t reflect.Value) bool {
	return slices.Contains(skipDeepCopyTypes, t.Type())
}
