package dto

import (
	"encoding/json"
	"log/slog"
	"reflect"
)

// secretStringType is the reflect.Type of SecretString, used to detect secret
// leaves while walking the DTO type graph.
var secretStringType = reflect.TypeFor[SecretString]()

// RedactCopy returns a deep copy of v in which every SecretString has been
// replaced by a redacted marker. The input is never mutated.
//
// It keys on the SecretString type (not field names or tags), so any new secret
// field is protected automatically. It is the single redaction primitive shared
// by the log handler's ReplaceAttr sink and by LogValue implementations that
// must serialize a value themselves (e.g. Config.LogValue).
func RedactCopy[T any](v T) T {
	redacted, changed := redactReflect(reflect.ValueOf(v), map[uintptr]struct{}{})
	if !changed {
		return v
	}

	out, ok := redacted.Interface().(T)
	if !ok {
		return v
	}

	return out
}

// LogValue renders the configuration as redacted JSON so it logs as complete,
// human-readable output under any handler (the text handler would otherwise
// print nested pointers as addresses). Redaction happens here because slog
// resolves LogValue before the handler's ReplaceAttr sink runs.
func (c Config) LogValue() slog.Value {
	safe := RedactCopy(c)

	data, err := json.Marshal(safe)
	if err != nil {
		return slog.StringValue("failed to render configuration: " + err.Error())
	}

	return slog.StringValue(string(data))
}

func redactReflect(src reflect.Value, seen map[uintptr]struct{}) (reflect.Value, bool) {
	if !src.IsValid() {
		return src, false
	}

	if src.Type() == secretStringType {
		return reflect.ValueOf(NewSecretString(secretRedactedPlaceholder)), true
	}

	switch src.Kind() {
	case reflect.Pointer:
		if src.IsNil() {
			return src, false
		}
		if _, ok := seen[src.Pointer()]; ok {
			return src, false
		}
		seen[src.Pointer()] = struct{}{}

		elem, changed := redactReflect(src.Elem(), seen)
		if !changed {
			return src, false
		}
		out := reflect.New(src.Elem().Type())
		out.Elem().Set(elem)

		return out, true

	case reflect.Interface:
		if src.IsNil() {
			return src, false
		}
		elem, changed := redactReflect(src.Elem(), seen)
		if !changed {
			return src, false
		}

		return elem, true

	case reflect.Struct:
		return redactStructReflect(src, seen)

	case reflect.Map:
		return redactMapReflect(src, seen)

	case reflect.Slice, reflect.Array:
		return redactSequenceReflect(src, seen)

	default:
		return src, false
	}
}

func redactStructReflect(src reflect.Value, seen map[uintptr]struct{}) (reflect.Value, bool) {
	t := src.Type()
	out := reflect.New(t).Elem()
	out.Set(src)
	changed := false
	for i := range t.NumField() {
		if !t.Field(i).IsExported() {
			continue
		}
		field, c := redactReflect(src.Field(i), seen)
		if c {
			out.Field(i).Set(field)
			changed = true
		}
	}
	if !changed {
		return src, false
	}

	return out, true
}

func redactMapReflect(src reflect.Value, seen map[uintptr]struct{}) (reflect.Value, bool) {
	if src.IsNil() {
		return src, false
	}
	out := reflect.MakeMapWithSize(src.Type(), src.Len())
	changed := false
	iter := src.MapRange()
	for iter.Next() {
		val, c := redactReflect(iter.Value(), seen)
		if c {
			changed = true
		}
		out.SetMapIndex(iter.Key(), val)
	}
	if !changed {
		return src, false
	}

	return out, true
}

func redactSequenceReflect(src reflect.Value, seen map[uintptr]struct{}) (reflect.Value, bool) {
	if src.Kind() == reflect.Slice && src.IsNil() {
		return src, false
	}

	var out reflect.Value
	if src.Kind() == reflect.Array {
		out = reflect.New(src.Type()).Elem()
	} else {
		out = reflect.MakeSlice(src.Type(), src.Len(), src.Len())
	}

	changed := false
	for i := range src.Len() {
		elem, c := redactReflect(src.Index(i), seen)
		if c {
			changed = true
		}
		out.Index(i).Set(elem)
	}
	if !changed {
		return src, false
	}

	return out, true
}
