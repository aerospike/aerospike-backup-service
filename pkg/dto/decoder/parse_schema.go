package decoder

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Parse the OpenAPI spec and build a map of struct types to their fieldsByStruct.
// result map: struct name -> list of field names.
func parseOpenAPISpec(yamlSpec string) (map[string][]string, error) {
	var spec map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlSpec), &spec)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return nil, err
	}

	// Extract schemas from definitions
	definitions, ok := spec["definitions"].(map[string]interface{})
	if !ok {
		fmt.Println("No definitions section found in spec")
		return nil, fmt.Errorf("no definitions section found in spec")
	}

	result := make(map[string][]string)

	for schemaName, schemaData := range definitions {
		schemaObj, ok := schemaData.(map[string]interface{})
		if !ok {
			continue
		}

		properties, ok := schemaObj["properties"].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract field names
		for fieldName := range properties {
			result[schemaName] = append(result[schemaName], fieldName)
		}
	}

	// Print the result
	for structName, fields := range result {
		fmt.Printf("%s: %v\n", structName, fields)
	}

	return result, nil
}
