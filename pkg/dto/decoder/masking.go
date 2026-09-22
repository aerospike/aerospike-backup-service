package decoder

import (
	"log/slog"
	"reflect"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

var (
	redactableType = reflect.TypeFor[redact.Redactable]()

	// containsRedactableCache memoizes containsRedactable. A type's answer never changes,
	// and the walk asks the same question for the same types on every log line.
	containsRedactableCache sync.Map // reflect.Type -> bool
)

type visitKey struct {
	ptr uintptr
	typ reflect.Type
}

// RedactSecrets returns a deep copy of v with all Secret-typed values replaced by redact.Placeholder.
// Values that hold no such secret are returned as they are: rebuilding them through reflection
// would drop the state their unexported fields hold. The walk handles cyclical value graphs safely.
func RedactSecrets(v any) any {
	if v == nil {
		return nil
	}

	visited := make(map[visitKey]reflect.Value)
	return redactValue(reflect.ValueOf(v), visited).Interface()
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

// redactedCredential replaces a credential with its safe display form, keeping the value's own
// type so it can be stored back into the field, map entry or slice element it came from.
// The caller establishes that v is one, through isRedactable.
func redactedCredential(v reflect.Value) reflect.Value {
	r, ok := v.Interface().(redact.Redactable)
	if !ok {
		return v
	}

	dst := reflect.New(v.Type()).Elem()
	dst.SetString(r.DisplayString())

	return dst
}

//nolint:gocognit,funlen // recursive reflect walk over nested DTO values
func redactValue(v reflect.Value, visited map[visitKey]reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	// A value read through an unexported field is read-only: reflect refuses to assign it
	// anywhere, so copying it into the rebuilt struct panics. The walk substitutes the zero
	// value, which is the only thing reflect allows here and the safe direction for a value
	// that may carry a credential. This is what keeps types that hold private state -
	// time.Time, sync.Mutex, a wrapped error - out of the walk's way without naming any of them.
	if !v.CanInterface() {
		return reflect.Zero(v.Type())
	}

	if isRedactable(v) {
		return redactedCredential(v) // this value IS the credential
	}

	if !needsRedaction(v) {
		return v // nothing inside is one — don't bother
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}

		key := visitKey{ptr: v.Pointer(), typ: v.Type()}
		if dst, ok := visited[key]; ok {
			return dst
		}

		dst := reflect.New(v.Elem().Type())
		visited[key] = dst

		dst.Elem().Set(redactValue(v.Elem(), visited))

		return dst

	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}

		return redactValue(v.Elem(), visited)

	case reflect.Struct:
		dst := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			redacted := redactValue(v.Field(i), visited)
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

		key := visitKey{ptr: v.Pointer(), typ: v.Type()}
		if dst, ok := visited[key]; ok {
			return dst
		}

		dst := reflect.MakeMapWithSize(v.Type(), v.Len())
		visited[key] = dst

		for _, mapKey := range v.MapKeys() {
			dst.SetMapIndex(
				redactValue(mapKey, visited),
				redactValue(v.MapIndex(mapKey), visited),
			)
		}

		return dst

	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}

		if v.Pointer() != 0 {
			key := visitKey{ptr: v.Pointer(), typ: v.Type()}
			if dst, ok := visited[key]; ok {
				return dst
			}

			dst := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
			visited[key] = dst

			for i := 0; i < v.Len(); i++ {
				dst.Index(i).Set(redactValue(v.Index(i), visited))
			}

			return dst
		}

		dst := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
		for i := 0; i < v.Len(); i++ {
			dst.Index(i).Set(redactValue(v.Index(i), visited))
		}

		return dst

	case reflect.Array:
		dst := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			dst.Index(i).Set(redactValue(v.Index(i), visited))
		}

		return dst

	default:
		return v
	}
}

// needsRedaction reports whether v holds a credential that redacts itself, anywhere inside.
// Only such a value is worth the deep copy: every other value is returned as it stands, so
// a logged struct keeps the state its unexported fields hold.
//
// The type of v answers the question on its own unless an interface stands in the way, in
// which case the dynamic value behind it decides.
func needsRedaction(v reflect.Value) bool {
	return needsRedactionVisited(v, make(map[visitKey]bool))
}

func needsRedactionVisited(v reflect.Value, visited map[visitKey]bool) bool {
	if !v.IsValid() {
		return false
	}

	if isRedactable(v) {
		return true
	}

	if !containsRedactable(v.Type()) {
		return false
	}

	return walkTree(v, visited)
}

func walkTree(v reflect.Value, visited map[visitKey]bool) bool {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return false
		}
		key := visitKey{ptr: v.Pointer(), typ: v.Type()}
		if visited[key] {
			return false
		}
		visited[key] = true

		return needsRedactionVisited(v.Elem(), visited)

	case reflect.Interface:
		if v.IsNil() {
			return false
		}

		return needsRedactionVisited(v.Elem(), visited)

	case reflect.Struct:
		return anyFieldNeedsRedaction(v, visited)

	case reflect.Map:
		if v.IsNil() {
			return false
		}
		if v.Pointer() != 0 {
			key := visitKey{ptr: v.Pointer(), typ: v.Type()}
			if visited[key] {
				return false
			}
			visited[key] = true
		}

		return anyEntryNeedsRedaction(v, visited)

	case reflect.Slice:
		if v.IsNil() {
			return false
		}
		if v.Pointer() != 0 {
			key := visitKey{ptr: v.Pointer(), typ: v.Type()}
			if visited[key] {
				return false
			}
			visited[key] = true
		}

		return anyElementNeedsRedaction(v, visited)

	case reflect.Array:
		return anyElementNeedsRedaction(v, visited)

	default:
		return false
	}
}

func anyFieldNeedsRedaction(v reflect.Value, visited map[visitKey]bool) bool {
	for i := range v.NumField() {
		if needsRedactionVisited(v.Field(i), visited) {
			return true
		}
	}

	return false
}

func anyEntryNeedsRedaction(v reflect.Value, visited map[visitKey]bool) bool {
	if v.IsNil() {
		return false
	}

	for _, key := range v.MapKeys() {
		if needsRedactionVisited(key, visited) || needsRedactionVisited(v.MapIndex(key), visited) {
			return true
		}
	}

	return false
}

func anyElementNeedsRedaction(v reflect.Value, visited map[visitKey]bool) bool {
	for i := range v.Len() {
		if needsRedactionVisited(v.Index(i), visited) {
			return true
		}
	}

	return false
}

// containsRedactable reports whether a value of type t can reach a credential that redacts
// itself, through any field, element, key or pointer. An interface member is unknown here, so
// it counts as reachable and leaves the decision to needsRedaction's walk over the value.
func containsRedactable(t reflect.Type) bool {
	if cached, ok := containsRedactableCache.Load(t); ok {
		reachable, _ := cached.(bool)

		return reachable
	}

	reachable := reachesRedactable(t, map[reflect.Type]struct{}{})
	containsRedactableCache.Store(t, reachable)

	return reachable
}

// reachesRedactable answers containsRedactable for t. A type already on the path reaches
// nothing its first visit has not reached already, which is what keeps a recursive type from
// recursing here.
func reachesRedactable(t reflect.Type, onPath map[reflect.Type]struct{}) bool {
	if t.Kind() == reflect.String && t.Implements(redactableType) {
		return true
	}

	if _, visiting := onPath[t]; visiting {
		return false
	}
	onPath[t] = struct{}{}
	defer delete(onPath, t)

	switch t.Kind() {
	case reflect.Interface:
		return true

	case reflect.Pointer, reflect.Slice, reflect.Array:
		return reachesRedactable(t.Elem(), onPath)

	case reflect.Map:
		return reachesRedactable(t.Key(), onPath) || reachesRedactable(t.Elem(), onPath)

	case reflect.Struct:
		for i := range t.NumField() {
			if reachesRedactable(t.Field(i).Type, onPath) {
				return true
			}
		}

		return false

	default:
		return false
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
