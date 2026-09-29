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

// RedactSecrets returns a deep copy of v with all Secret-typed values replaced by redact.Placeholder.
// Values that hold no such secret are returned as they are: rebuilding them through reflection
// would drop the state their unexported fields hold. A pointer, map or slice reached twice -
// through a cycle or through two paths - is copied once, so the copy keeps the shape of the
// original and the walk ends on any value graph.
func RedactSecrets[T any](v T) T {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return v
	}

	// The walk preserves the type of what it is handed, so the copy is a T again.
	return redactValue(rv, copies{}).Interface().(T)
}

// RedactSecretsReplaceAttr returns a slog ReplaceAttr function that redacts Secret-typed values.
func RedactSecretsReplaceAttr() func(groups []string, a slog.Attr) slog.Attr {
	return func(_ []string, a slog.Attr) slog.Attr {
		return slog.Any(a.Key, RedactSecrets(a.Value.Any()))
	}
}

// reference identifies what a non-nil pointer, map or slice refers to, so a walk recognizes
// one it has met before. The type is part of the identity: a struct and its first field share
// an address. A slice is identified by its view - data pointer, length and capacity - because
// two slices over one array are different values: s and s[:1] must not share a copy, and a
// secret past the end of the shorter one must still be found.
type reference struct {
	ptr      uintptr
	len, cap int
	typ      reflect.Type
}

// referenceTo returns the identity of v. The caller establishes that v is a non-nil pointer,
// map or slice.
func referenceTo(v reflect.Value) reference {
	ref := reference{ptr: v.Pointer(), typ: v.Type()}
	if v.Kind() == reflect.Slice {
		ref.len, ref.cap = v.Len(), v.Cap()
	}

	return ref
}

// copies remembers the copy made for each reference a redaction walk has descended into, so
// a reference reached again resolves to that same copy. A copy is registered before its
// contents are filled in, which is what closes a cycle: the back-edge finds a copy that is
// still being built, and since pointers, maps and slices are reference types it is complete
// by the time anyone reads it.
type copies map[reference]reflect.Value

// isRedactable reports whether v's type is a credential that redacts itself. A pointer to one
// is not: its method set carries the value's methods, but Redacted would hand back the value,
// not a pointer, so the walk follows the pointer and redacts the credential where it lives.
// The answer comes from the type alone, so it holds for a value reflect will not hand out.
func isRedactable(v reflect.Value) bool {
	return v.Kind() != reflect.Pointer && v.Type().Implements(redactableType)
}

// asRedactable returns v as the credential it is, or false when it is none. The caller
// establishes that v can be read, through CanInterface.
func asRedactable(v reflect.Value) (redact.Redactable, bool) {
	if v.Kind() == reflect.Pointer {
		return nil, false
	}

	r, ok := v.Interface().(redact.Redactable)

	return r, ok
}

// redactValue returns v with every credential inside it redacted, or v itself when it holds
// none. needsRedaction answers false for a nil pointer, map, slice or interface, so the
// per-kind copies below only ever see non-nil ones.
func redactValue(v reflect.Value, seen copies) reflect.Value {
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

	if r, ok := asRedactable(v); ok {
		return reflect.ValueOf(r.Redacted()) // this value IS the credential
	}

	if !needsRedaction(v) {
		return v // nothing inside is one — don't bother
	}

	switch v.Kind() {
	case reflect.Pointer:
		return redactPointer(v, seen)

	case reflect.Interface:
		return redactValue(v.Elem(), seen)

	case reflect.Struct:
		return redactStruct(v, seen)

	case reflect.Map:
		return redactMap(v, seen)

	case reflect.Slice:
		return redactSlice(v, seen)

	case reflect.Array:
		return redactArray(v, seen)

	default:
		return v
	}
}

func redactPointer(v reflect.Value, seen copies) reflect.Value {
	ref := referenceTo(v)
	if dst, ok := seen[ref]; ok {
		return dst
	}

	dst := reflect.New(v.Elem().Type())
	seen[ref] = dst

	dst.Elem().Set(redactValue(v.Elem(), seen))

	return dst
}

func redactStruct(v reflect.Value, seen copies) reflect.Value {
	dst := reflect.New(v.Type()).Elem()
	for i := range v.NumField() {
		if field := dst.Field(i); field.CanSet() {
			setField(field, redactValue(v.Field(i), seen))
		}
	}

	return dst
}

// redactMap redacts keys as well as values. Two secret keys therefore collapse into one
// placeholder entry, which is the safe direction for output nobody is meant to read back.
func redactMap(v reflect.Value, seen copies) reflect.Value {
	ref := referenceTo(v)
	if dst, ok := seen[ref]; ok {
		return dst
	}

	dst := reflect.MakeMapWithSize(v.Type(), v.Len())
	seen[ref] = dst

	for _, key := range v.MapKeys() {
		dst.SetMapIndex(redactValue(key, seen), redactValue(v.MapIndex(key), seen))
	}

	return dst
}

func redactSlice(v reflect.Value, seen copies) reflect.Value {
	ref := referenceTo(v)
	if dst, ok := seen[ref]; ok {
		return dst
	}

	dst := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())
	seen[ref] = dst

	for i := range v.Len() {
		dst.Index(i).Set(redactValue(v.Index(i), seen))
	}

	return dst
}

func redactArray(v reflect.Value, seen copies) reflect.Value {
	dst := reflect.New(v.Type()).Elem()
	for i := range v.Len() {
		dst.Index(i).Set(redactValue(v.Index(i), seen))
	}

	return dst
}

// needsRedaction reports whether v holds a credential that redacts itself, anywhere inside.
// Only such a value is worth the deep copy: every other value is returned as it stands, so
// a logged struct keeps the state its unexported fields hold.
//
// The type of v answers the question on its own unless an interface stands in the way, in
// which case the dynamic value behind it decides.
func needsRedaction(v reflect.Value) bool {
	return holdsSecret(v, visited{})
}

// visited is the set of references a search has descended into.
type visited map[reference]struct{}

// enter records v, a non-nil pointer, map or slice, and reports whether this is its first visit.
func (s visited) enter(v reflect.Value) bool {
	ref := referenceTo(v)
	if _, seen := s[ref]; seen {
		return false
	}
	s[ref] = struct{}{}

	return true
}

// holdsSecret answers needsRedaction. A reference met a second time contributes nothing new:
// either it was explored to the end and found clean, or it is still on the path and its other
// branches decide - so it answers false, and the search is finite on any value graph.
func holdsSecret(v reflect.Value, seen visited) bool {
	if !v.IsValid() {
		return false
	}

	if isRedactable(v) {
		return true
	}

	if !containsRedactable(v.Type()) {
		return false
	}

	switch v.Kind() {
	case reflect.Pointer:
		return !v.IsNil() && seen.enter(v) && holdsSecret(v.Elem(), seen)

	case reflect.Interface:
		return !v.IsNil() && holdsSecret(v.Elem(), seen)

	case reflect.Struct:
		return anyFieldHoldsSecret(v, seen)

	case reflect.Map:
		return !v.IsNil() && seen.enter(v) && anyEntryHoldsSecret(v, seen)

	case reflect.Slice:
		return !v.IsNil() && seen.enter(v) && anyElementHoldsSecret(v, seen)

	case reflect.Array:
		return anyElementHoldsSecret(v, seen)

	default:
		return false
	}
}

func anyFieldHoldsSecret(v reflect.Value, seen visited) bool {
	for i := range v.NumField() {
		if holdsSecret(v.Field(i), seen) {
			return true
		}
	}

	return false
}

func anyEntryHoldsSecret(v reflect.Value, seen visited) bool {
	for _, key := range v.MapKeys() {
		if holdsSecret(key, seen) || holdsSecret(v.MapIndex(key), seen) {
			return true
		}
	}

	return false
}

func anyElementHoldsSecret(v reflect.Value, seen visited) bool {
	for i := range v.Len() {
		if holdsSecret(v.Index(i), seen) {
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
	if t.Kind() != reflect.Pointer && t.Implements(redactableType) {
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
