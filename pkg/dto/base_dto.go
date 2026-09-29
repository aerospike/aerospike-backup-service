package dto

import (
	"io"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Validator interface for types that can be validated.
type Validator interface {
	// Validate validates the object.
	Validate() error
}

// NewFromReader reads and deserializes a T from r in the given format.
func NewFromReader[T any](r io.Reader, format decoder.SerializationFormat) (*T, error) {
	var v T
	if err := decoder.Deserialize(&v, r, format); err != nil {
		return nil, err
	}

	return &v, nil
}

// NewValidatedFromReader reads, deserializes, and validates a T from r in the given format.
// Only use this for a T whose Validate is self-contained; Config.Validate, for one, must run
// after the caller merges secrets back in, so Config is read with plain NewFromReader instead.
func NewValidatedFromReader[T any, PT interface {
	*T
	Validator
}](r io.Reader, format decoder.SerializationFormat) (*T, error) {
	v, err := NewFromReader[T](r, format)
	if err != nil {
		return nil, err
	}

	if err := PT(v).Validate(); err != nil {
		return nil, err
	}

	return v, nil
}

// ConvertModelsToDTO converts an array of models to an array of DTOs.
func ConvertModelsToDTO[M any, D any](models []M, dtoConstructor func(*M) D) []D {
	result := make([]D, len(models))
	for i := range models {
		result[i] = dtoConstructor(&models[i])
	}
	return result
}

// ConvertModelMapToDTO converts a map of models to a map of DTOs.
func ConvertModelMapToDTO[M any, D any](modelMap map[string]*M, dtoConstructor func(*M) *D) map[string]*D {
	result := make(map[string]*D, len(modelMap))
	for key, m := range modelMap {
		result[key] = dtoConstructor(m)
	}
	return result
}

// ConvertStorageMapToDTO converts a map of models to a map of DTOs.
func ConvertStorageMapToDTO(modelMap map[string]model.Storage, config *model.BackupConfig) map[string]*Storage {
	result := make(map[string]*Storage, len(modelMap))
	for key, s := range modelMap {
		result[key] = NewStorageFromModel(s, config)
	}
	return result
}

// canonicalEnum maps v to an allowed value, ignoring case and surrounding space.
// Empty or whitespace-only input returns the zero value and true.
func canonicalEnum[T ~string](v T, allowed []T) (T, bool) {
	s := strings.TrimSpace(string(v))
	if s == "" {
		var zero T
		return zero, true
	}
	for _, a := range allowed {
		if strings.EqualFold(s, string(a)) {
			return a, true
		}
	}

	return v, false
}
