package dto

import (
	"encoding/json"
	"fmt"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"io"

	"gopkg.in/yaml.v3"
)

// SerializationFormat represents the format for serialization/deserialization
type SerializationFormat int

const (
	JSON SerializationFormat = iota
	YAML
)

// Serialize handles serialization
func Serialize(v any, format SerializationFormat) ([]byte, error) {
	var (
		data []byte
		err  error
	)

	switch format {
	case JSON:
		data, err = json.MarshalIndent(v, "", "    ") // pretty print
	case YAML:
		data, err = yaml.Marshal(v)
	default:
		return nil, fmt.Errorf("unsupported format: %v", format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// Deserialize handles deserialization
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

// ConvertModelsToDTO converts an array of models to an array of DTOs
func ConvertModelsToDTO[M any, D any](models []M, dtoConstructor func(*M) D) []D {
	result := make([]D, len(models))
	for i, model := range models {
		result[i] = dtoConstructor(&model)
	}
	return result
}

// ConvertModelMapToDTO converts a map of models to a map of DTOs
func ConvertModelMapToDTO[M any, D any](modelMap map[string]*M, dtoConstructor func(*M) *D) map[string]*D {
	result := make(map[string]*D, len(modelMap))
	for key, model := range modelMap {
		result[key] = dtoConstructor(model)
	}
	return result
}

// ConvertStorageMapToDTO converts a map of models to a map of DTOs
func ConvertStorageMapToDTO(modelMap map[string]model.Storage) map[string]*Storage {
	result := make(map[string]*Storage, len(modelMap))
	for key, s := range modelMap {
		result[key] = NewStorageFromModel(s)
	}
	return result
}
