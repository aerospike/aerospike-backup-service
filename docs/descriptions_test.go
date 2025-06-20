package docs

import (
	"encoding/json"
	"os"
	"testing"
)

const (
	filePath = "openapi.json"

	descriptionTag = "description"
	propertiesTag  = "properties"
	componentsTag  = "components"
	schemasTag     = "schemas"
	requiredTag    = "required"
	defaultTag     = "default"
	allOfTag       = "allOf"
	nullableTag    = "nullable"
	typeTag        = "type"
)

// TestOpenAPIDescriptions verifies that every DTO and field has a description,
// and that every field is either required or has a default value.
func TestOpenAPIDescriptions(t *testing.T) {
	schemas := readSchemas(t, filePath)

	for schemaName, rawSchema := range schemas {
		schema, ok := rawSchema.(map[string]any)
		if !ok {
			t.Errorf("Invalid schema format for: %s", schemaName)
			continue
		}

		if _, ok := schema[descriptionTag]; !ok {
			t.Errorf("Object '%s' is missing a description", schemaName)
		}

		assertAllPropertiesValid(t, schema, schemaName)
	}
}

func readSchemas(t *testing.T, filePath string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read OpenAPI file: %v", err)
	}

	var openapi map[string]any
	if err := json.Unmarshal(data, &openapi); err != nil {
		t.Fatalf("Failed to parse OpenAPI JSON: %v", err)
	}

	components, ok := openapi[componentsTag].(map[string]any)
	if !ok {
		t.Fatalf("Missing 'components' in OpenAPI file")
	}

	schemas, ok := components[schemasTag].(map[string]any)
	if !ok {
		t.Fatalf("Missing 'schemas' in OpenAPI components")
	}
	return schemas
}

var skipValidation = map[string]bool{
	"dto.RetryPolicy": true, // retry policy has different values for different use-cases
	// output DTOs:
	"dto.RestoreJobStatus": true,
	"dto.RoutineState":     true,
	"dto.Metrics":          true,
	"dto.BackupDetails":    true,
	"dto.RunningJob":       true,
}

func assertAllPropertiesValid(t *testing.T, schema map[string]any, schemaName string) {
	t.Helper()

	properties, hasProps := schema[propertiesTag].(map[string]any)
	if !hasProps {
		return
	}

	requiredSet := map[string]bool{}
	if requiredList, ok := schema[requiredTag].([]any); ok {
		for _, r := range requiredList {
			if rStr, ok := r.(string); ok {
				requiredSet[rStr] = true
			}
		}
	}

	for propName, rawProp := range properties {
		prop, ok := rawProp.(map[string]any)
		if !ok {
			t.Errorf("Invalid property format: %s.%s", schemaName, propName)
			continue
		}

		// Check for description
		if _, ok := prop[descriptionTag]; !ok {
			t.Errorf("Property '%s.%s' is missing a description", schemaName, propName)
		}

		if _, ok := prop[allOfTag]; ok {
			continue // this is an object
		}

		if prop[typeTag] == "object" {
			continue // this is a map
		}

		if skipValidation[schemaName] {
			continue // produced objects don't have defaults
		}

		// Check for required or default
		_, isRequired := requiredSet[propName]
		_, hasDefault := prop[defaultTag]
		_, isNullable := prop[nullableTag] // nullable fields means that default value is nil

		if !(isRequired || hasDefault || isNullable) {
			t.Errorf("Property '%s.%s' is neither required nor has a default value", schemaName, propName)
		}
	}
}
