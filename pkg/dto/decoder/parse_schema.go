package decoder

import (
	"gopkg.in/yaml.v3"
)

// OpenAPISpec represents the structure of an OpenAPI specification.
type OpenAPISpec struct {
	Definitions map[string]SchemaObject `yaml:"definitions"`
}

// SchemaObject represents a schema definition in OpenAPI.
type SchemaObject struct {
	Properties map[string]struct{} `yaml:"properties"`
}

// Parse the OpenAPI spec and build a map of struct types to their fields.
// result map: struct name -> list of field names.
func parseOpenAPISpec(yamlSpec string) (map[string][]string, error) {
	var spec OpenAPISpec
	err := yaml.Unmarshal([]byte(yamlSpec), &spec)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for schemaName, schemaObj := range spec.Definitions {
		for fieldName := range schemaObj.Properties {
			result[schemaName] = append(result[schemaName], fieldName)
		}
	}

	return result, nil
}
