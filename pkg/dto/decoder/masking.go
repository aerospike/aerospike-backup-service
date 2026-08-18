package decoder

import (
	"log/slog"
	"reflect"
	"time"
)

var (
	secretType = reflect.TypeFor[Secret]()
	timeType   = reflect.TypeFor[time.Time]()
)

// RedactSecrets returns a deep copy of v with all Secret-typed values replaced by RedactedSecret.
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

//nolint:gocognit,funlen // recursive reflect walk over nested DTO values
func redactValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	if v.Type() == secretType {
		s, ok := v.Interface().(Secret)
		if ok && s.IsRef() {
			return v
		}

		return reflect.ValueOf(Secret(RedactedSecret))
	}

	if v.Type() == timeType {
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
