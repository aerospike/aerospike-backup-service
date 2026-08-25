package dto

import (
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// Validator interface for types that can be validated.
type Validator interface {
	// Validate validates the object.
	Validate(opts ValidationOptions) error
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

func foldUpper(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}
