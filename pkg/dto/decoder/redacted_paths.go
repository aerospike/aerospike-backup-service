package decoder

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// RedactedPaths returns the location of every credential in v whose value is the redaction
// placeholder itself, named by the YAML (else JSON) field names a user writes, e.g.
// "aerospike-clusters.c1.credentials.password". The result is sorted and empty when v holds none.
//
// The placeholder only means something to MergeSecrets, which replaces it with the stored value
// on a GET-edit-PUT round trip. A value that still carries it was never merged - it came from a
// configuration file or a request that has no stored value to stand for - and would otherwise be
// used as the credential.
func RedactedPaths(v any) []string {
	var paths []string
	findRedacted(reflect.ValueOf(v), "", visited{}, &paths)
	slices.Sort(paths)

	return paths
}

// findRedacted appends to paths the location, under path, of every placeholder credential in v.
// A pointer, map or slice is entered once, so the search ends on any value graph.
func findRedacted(v reflect.Value, path string, seen visited, paths *[]string) {
	if !v.IsValid() || !v.CanInterface() {
		return
	}

	if r, ok := asRedactable(v); ok {
		if r.IsRedacted() {
			*paths = append(*paths, path)
		}

		return
	}

	if !containsRedactable(v.Type()) {
		return
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if v.IsNil() || !seen.enter(v) {
			return
		}
	default:
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		findRedacted(v.Elem(), path, seen, paths)

	case reflect.Struct:
		findRedactedInFields(v, path, seen, paths)

	case reflect.Map:
		for _, key := range v.MapKeys() {
			findRedacted(v.MapIndex(key), joinPath(path, fmt.Sprint(key.Interface())), seen, paths)
		}

	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			findRedacted(v.Index(i), fmt.Sprintf("%s[%d]", path, i), seen, paths)
		}

	default:
	}
}

// findRedactedInFields is findRedacted for the fields of the struct v. An inlined field shares
// its parent's path; a field excluded from serialization is not searched.
func findRedactedInFields(v reflect.Value, path string, seen visited, paths *[]string) {
	for i := range v.NumField() {
		name, inline := fieldName(v.Type().Field(i))
		if name == "-" {
			continue
		}

		fieldPath := joinPath(path, name)
		if inline {
			fieldPath = path
		}
		findRedacted(v.Field(i), fieldPath, seen, paths)
	}
}

// fieldName returns the name a user writes for f, and whether f is inlined into its parent.
func fieldName(f reflect.StructField) (string, bool) {
	for _, key := range []string{"yaml", "json"} {
		tag, ok := f.Tag.Lookup(key)
		if !ok {
			continue
		}

		name, options, _ := strings.Cut(tag, ",")
		if slices.Contains(strings.Split(options, ","), "inline") {
			return "", true
		}
		if name != "" {
			return name, false
		}
	}

	return f.Name, f.Anonymous
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}
