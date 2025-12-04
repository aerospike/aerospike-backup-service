package dto

import (
	"log/slog"
	"math"
	"reflect"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

// ToModeler is a constraint for any type that has a ToModel() method.
type ToModeler[M any] interface {
	ToModel() *M
	Validator
}

// Registry from concrete reflect.Type (or Kind) to a draw function.
// Each draw returns a reflect.Value settable to the field type.
type genRegistry map[reflect.Type]func(t *rapid.T) reflect.Value

const (
	Float32Min = 0.1
	Float32Max = 10.0
	Float64Min = 0.1
	Float64Max = 10.0
)

var (
	Int64InterestingValues = []int64{
		-1, 0, 1, 10, 100, 9_999_999, math.MaxInt64,
	}
	IntInterestingValues = []int{
		-1, 0, 1, 10, 100, 25_000, 9_999_999, math.MaxInt,
	}

	StringSamples = []string{
		"a", "b", "ZSTD", "NONE", "nonzero", "αβ", "",
	}
)

// RapidStruct builds a Rapid generator for any struct T using reflection.
// It fills ALL exported fields. If it hits an unsupported type, it panics.
//
//nolint:gocritic
func RapidStruct[T any](reg genRegistry) *rapid.Generator[T] {
	if reg == nil {
		reg = defaultRegistry()
	}
	return rapid.Custom(func(t *rapid.T) T {
		var zero T
		rt := reflect.TypeOf(zero)
		rv := reflect.New(rt).Elem()

		if rt.Kind() != reflect.Struct {
			t.Fatalf("RapidStruct[%T]: T must be a struct", zero)
		}

		for i := 0; i < rt.NumField(); i++ {
			sf := rt.Field(i)
			if !sf.IsExported() {
				continue // can't set unexported
			}
			if tag := sf.Tag.Get("testgen"); tag == "ignore" {
				continue
			}

			ft := sf.Type
			fv := rv.Field(i)
			val, ok := drawValue(t, reg, ft)
			if !ok {
				t.Fatalf("no generator registered for field %q of type %v", sf.Name, ft)
			}
			// Handle assignability (e.g., named types)
			if val.Type().AssignableTo(ft) {
				fv.Set(val)
			} else if val.Type().ConvertibleTo(ft) {
				fv.Set(val.Convert(ft))
			} else {
				t.Fatalf("cannot assign %v to field %q of type %v", val.Type(), sf.Name, ft)
			}
		}
		return rv.Interface().(T)
	})
}

func defaultRegistry() genRegistry {
	r := genRegistry{}

	r[reflect.TypeOf(int64(0))] = func(t *rapid.T) reflect.Value {
		v := rapid.SampledFrom(Int64InterestingValues).Draw(t, "int64")
		return reflect.ValueOf(v)
	}

	r[reflect.TypeOf(0)] = func(t *rapid.T) reflect.Value {
		v := rapid.SampledFrom(IntInterestingValues).Draw(t, "int")
		return reflect.ValueOf(v)
	}

	r[reflect.TypeOf(float64(0))] = func(t *rapid.T) reflect.Value {
		v := rapid.Float64Range(Float64Min, Float64Max).Draw(t, "float64")
		return reflect.ValueOf(v)
	}

	r[reflect.TypeOf(float32(0))] = func(t *rapid.T) reflect.Value {
		v := rapid.Float32Range(Float32Min, Float32Max).Draw(t, "float32")
		return reflect.ValueOf(v)
	}

	r[reflect.TypeOf("")] = func(t *rapid.T) reflect.Value {
		v := rapid.SampledFrom(StringSamples).Draw(t, "string")
		return reflect.ValueOf(v)
	}

	r[reflect.TypeOf(false)] = func(t *rapid.T) reflect.Value {
		v := rapid.Bool().Draw(t, "bool")
		return reflect.ValueOf(v)
	}

	return r
}

// drawValue covers basic kinds and uses the registry for exact types.
// Extend with slices, maps, pointers, nested structs as needed.
func drawValue(t *rapid.T, reg genRegistry, ft reflect.Type) (reflect.Value, bool) {
	// Exact match first (handles named types)
	if f, ok := reg[ft]; ok {
		return f(t), true
	}

	switch ft.Kind() {
	case reflect.Struct:
		// Recursively fill nested structs
		gen := RapidStructWithType(ft, reg)
		// gen returns reflect.Value for that struct type
		return gen(t), true
	case reflect.Ptr:
		// Optionally: sometimes nil, sometimes non-nil
		if rapid.Bool().Draw(t, "ptrNil") {
			return reflect.Zero(ft), true // nil
		}
		elem := ft.Elem()
		elemVal, ok := drawValue(t, reg, elem)
		if !ok {
			return reflect.Value{}, false
		}
		p := reflect.New(ft.Elem())
		p.Elem().Set(elemVal)
		return p, true

	case reflect.Slice:
		// Small slices; ensure non-empty often to catch missed mappings
		n := rapid.IntRange(0, 3).Draw(t, "len")
		s := reflect.MakeSlice(ft, n, n)
		for i := 0; i < n; i++ {
			ev, ok := drawValue(t, reg, ft.Elem())
			if !ok {
				return reflect.Value{}, false
			}
			s.Index(i).Set(ev)
		}
		return s, true
	case reflect.Map:
		n := rapid.IntRange(0, 3).Draw(t, "mlen")
		m := reflect.MakeMapWithSize(ft, n)
		for i := 0; i < n; i++ {
			kv, ok1 := drawValue(t, reg, ft.Key())
			vv, ok2 := drawValue(t, reg, ft.Elem())
			if !ok1 || !ok2 {
				return reflect.Value{}, false
			}
			m.SetMapIndex(kv, vv)
		}
		return m, true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		// Default small-ish non-zero biased ints
		v := rapid.IntRange(-5, 20).Draw(t, "intN")
		return reflect.ValueOf(v).Convert(ft), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := rapid.Uint64Range(0, 100).Draw(t, "uintN")
		return reflect.ValueOf(v).Convert(ft), true
	case reflect.Float32:
		v := rapid.Float32Range(0.1, 10.0).Draw(t, "f32")
		return reflect.ValueOf(v).Convert(ft), true
	case reflect.Float64:
		v := rapid.Float64Range(0.1, 10.0).Draw(t, "f64")
		return reflect.ValueOf(v).Convert(ft), true
	case reflect.String:
		v := rapid.SampledFrom([]string{"a", "b", "xyz", "nonzero"}).Draw(t, "str")
		return reflect.ValueOf(v), true
	case reflect.Bool:
		v := rapid.Bool().Draw(t, "bool")
		return reflect.ValueOf(v), true
	default:
		return reflect.Value{}, false
	}
}

// Helper to generate a struct for an arbitrary reflect.Type (used for nested structs).
//
//nolint:gocritic
func RapidStructWithType(rt reflect.Type, reg genRegistry) func(t *rapid.T) reflect.Value {
	return func(t *rapid.T) reflect.Value {
		rv := reflect.New(rt).Elem()
		for i := 0; i < rt.NumField(); i++ {
			sf := rt.Field(i)
			if !sf.IsExported() {
				continue
			}
			if tag := sf.Tag.Get("testgen"); tag == "ignore" {
				continue
			}
			val, ok := drawValue(t, reg, sf.Type)
			if !ok {
				t.Fatalf("no generator for nested field %q of type %v", sf.Name, sf.Type)
			}
			if val.Type().AssignableTo(sf.Type) {
				rv.Field(i).Set(val)
			} else if val.Type().ConvertibleTo(sf.Type) {
				rv.Field(i).Set(val.Convert(sf.Type))
			} else {
				t.Fatalf("cannot assign %v to nested field %q of type %v", val.Type(), sf.Name, sf.Type)
			}
		}
		return rv
	}
}

// CheckRoundTrip runs a Rapid property that:
//  1. draws a DTO with the provided generator,
//  2. rejects invalid samples via Validate(),
//  3. converts DTO -> Model -> DTO,
//  4. asserts equality (using cmp.Diff).
//  5. does it at least 100 times for different draws.
func TestRetryPolicy_RoundTrip(t *testing.T) {
	genDTO := RapidStruct[RetryPolicy](defaultRegistry())

	rapid.Check(t, func(t *rapid.T) {
		dto := ptr.Of(genDTO.Draw(t, "retry-policy"))

		// Optional: keep only valid, but try to be valid-by-construction first.
		if err := dto.Validate(); err != nil {
			t.SkipNow()
		}

		rt := newRetryPolicyFromModel(dto.ToModel())
		if diff := cmp.Diff(dto, rt); diff != "" {
			t.Fatalf("round-trip mismatch:\n%s", diff)
		}
	})
}

func runRoundTripTest[T ToModeler[M], M any](t *testing.T,
	makeDTO func(rt *rapid.T) T,
	fromModel func(*M) T,
) {
	t.Helper()

	rapid.Check(t, func(t *rapid.T) {
		dto := makeDTO(t)

		if err := dto.Validate(); err != nil {
			t.SkipNow() // skip invalid samples
		}

		model := dto.ToModel()
		rt := fromModel(model)
		if diff := cmp.Diff(dto, rt); diff != "" {
			t.Fatalf("round-trip mismatch:\n%s", diff)
		} else {
			slog.Info("valid sample:", slog.Any("dto", dto))
		}
	})
}
