package dto

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"gopkg.in/yaml.v3"
)

// SerializationFormat represents the format for serialization/deserialization.
type SerializationFormat int

const (
	JSON SerializationFormat = iota
	YAML
)

// Validator interface for types that can be validated.
type Validator interface {
	Validate() error
}

// Deserialize handles deserialization.
func Deserialize(v any, r io.Reader, format SerializationFormat) error {
	var err error

	switch format {
	case JSON:
		err = json.NewDecoder(r).Decode(v)
	case YAML:
		err = yaml.NewDecoder(r).Decode(v)
	default:
		return fmt.Errorf("unsupported format: %v", format)
	}

	if err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	return nil
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
