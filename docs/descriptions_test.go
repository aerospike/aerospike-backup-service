package docs

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	filePath = "openapi.json"

	descriptionTag = "description"
	propertiesTag  = "properties"
	componentsTag  = "components"
	schemasTag     = "schemas"
)

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

		assertAllPropertiesHaveDescription(t, schema, schemaName)
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

func assertAllPropertiesHaveDescription(t *testing.T, schema map[string]any, schemaName string) {
	properties, hasProps := schema[propertiesTag].(map[string]any)
	if hasProps {
		for propName, rawProp := range properties {
			prop, ok := rawProp.(map[string]any)
			if !ok {
				t.Errorf("Invalid property format: %s.%s", schemaName, propName)
				continue
			}

			_, ok = prop[descriptionTag]
			assert.True(t, ok, "Property '%s' in '%s' is missing a description", propName, schemaName)
		}
	}
}
