package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
)

type OpenAPI struct {
	Components Components `json:"components"`
}

type Components struct {
	Schemas map[string]Schema `json:"schemas"`
}

type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
}

type Property struct {
	Description string      `json:"description,omitempty"`
	Type        string      `json:"type,omitempty"`
	AllOf       []Reference `json:"allOf"`
	Items       Reference   `json:"items"`
}

type Reference struct {
	Ref string `json:"$ref,omitempty"`
}

func generateMarkdownTable(openapiPath, dtoName string) (string, error) {
	data, err := os.ReadFile(openapiPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	var api OpenAPI
	if err := json.Unmarshal(data, &api); err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	for _, x := range jsonExamples {
		annotatedJSON, err := GenerateAnnotatedJSON(x, api)
		if err != nil {
			panic(err)
		}

		println(annotatedJSON)
	}

	schema, ok := api.Components.Schemas[dtoName]
	if !ok {
		return "", fmt.Errorf("schema %q not found", dtoName)
	}

	// Flatten properties recursively
	rows := dtoToRows(api.Components.Schemas, schema)

	// Determine column widths
	maxName := len("Field")
	maxHelp := len("Description")
	for _, r := range rows {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		for _, line := range strings.Split(r.Help, "\n") {
			if len(line) > maxHelp {
				maxHelp = len(line)
			}
		}
	}

	const quotes = 2
	var sb strings.Builder

	// Write header
	sb.WriteString(fmt.Sprintf("| %-*s | %-*s |\n", maxName+quotes, "Field", maxHelp, "Description"))
	sb.WriteString(fmt.Sprintf("|-%s-|-%s-|\n", strings.Repeat("-", maxName+quotes), strings.Repeat("-", maxHelp)))

	// Write rows
	for _, r := range rows {
		name := "`" + r.Name + "`"
		desc := strings.ReplaceAll(r.Help, "\n", "<br>")
		sb.WriteString(fmt.Sprintf("| %-*s | %-*s |\n", maxName+quotes, name, maxHelp, desc))
	}

	return sb.String(), nil
}

func dtoToRows(all map[string]Schema, input Schema) []Row {
	var rows []Row
	collectFields(all, input, "", &rows)
	sort.SliceStable(rows, func(i, j int) bool {
		depthI := strings.Count(rows[i].Name, ".")
		depthJ := strings.Count(rows[j].Name, ".")
		if depthI != depthJ {
			return depthI < depthJ
		}

		return false
	})

	return rows
}

func collectFields(schemas map[string]Schema, schema Schema, prefix string, out *[]Row) {
	for fieldName, prop := range schema.Properties {
		fullName := fieldName
		if prefix != "" {
			fullName = prefix + "." + fieldName
		}

		if prop.Type == "object" {
			// Check if it's a reference via allOf
			for _, ref := range prop.AllOf {
				if ref.Ref != "" {
					refName := extractRefName(ref.Ref)
					if refSchema, ok := schemas[refName]; ok {
						collectFields(schemas, refSchema, fullName, out)
					}
				}
			}
			continue // Don't print the top-level object
		}

		// Normal field
		*out = append(*out, Row{
			Name: fullName,
			Help: strings.ReplaceAll(prop.Description, "\n", "<br>"),
		})
	}
}

// Example: "#/components/schemas/dto.RunningJob" → "dto.RunningJob"
func extractRefName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

// GenerateAnnotatedJSON creates JSON with comments before fields
func GenerateAnnotatedJSON(dto interface{}, openAPI OpenAPI) (string, error) {
	// Get the struct type name
	structType := reflect.TypeOf(dto)
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	structName := structType.String()

	// Find the corresponding schema in OpenAPI
	schema, exists := openAPI.Components.Schemas[structName]
	if !exists {
		return "", fmt.Errorf("schema not found for struct: %s", structName)
	}

	// Serialize struct directly to JSON
	jsonBytes, err := json.MarshalIndent(dto, "", "  ")
	if err != nil {
		return "", err
	}

	// Add comments before fields
	result := addCommentsBeforeFields(string(jsonBytes), schema, openAPI)

	return result, nil
}

// addCommentsBeforeFields adds comment lines before field definitions
func addCommentsBeforeFields(jsonStr string, schema Schema, openAPI OpenAPI) string {
	lines := strings.Split(jsonStr, "\n")
	var result []string
	var currentPath []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " ")))

		// Handle opening braces
		if trimmed == "{" {
			result = append(result, line)
			continue
		}

		// Handle closing braces
		if trimmed == "}" || trimmed == "}," {
			result = append(result, line)
			// Pop from path when closing an object
			if len(currentPath) > 0 {
				currentPath = currentPath[:len(currentPath)-1]
			}
			continue
		}

		// Handle field lines
		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			fieldName := strings.Trim(strings.TrimSpace(parts[0]), "\"")
			value := strings.TrimSpace(parts[1])

			// Get the appropriate schema for current nesting level
			currentSchema := getSchemaForPath(schema, currentPath, openAPI)

			// Add comment before the field if description exists
			if prop, exists := currentSchema.Properties[fieldName]; exists && prop.Description != "" {
				commentLine := indent + "// " + prop.Description
				result = append(result, commentLine)
			}

			// Add the original field line
			result = append(result, line)

			// If this field's value is an object (starts with {), add to path
			if strings.HasPrefix(value, "{") {
				currentPath = append(currentPath, fieldName)
			}
		} else {
			// Non-field lines (like array elements)
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// getSchemaForPath traverses the schema based on the current JSON path
func getSchemaForPath(rootSchema Schema, path []string, openAPI OpenAPI) Schema {
	currentSchema := rootSchema

	for _, fieldName := range path {
		if prop, exists := currentSchema.Properties[fieldName]; exists {
			// Handle $ref references
			if len(prop.AllOf) != 0 {
				schemaName := extractSchemaNameFromRef(prop.AllOf[0].Ref)
				if nestedSchema, found := openAPI.Components.Schemas[schemaName]; found {
					currentSchema = nestedSchema
				}
			}
		}
	}

	return currentSchema
}

// extractSchemaNameFromRef extracts schema name from $ref like "#/components/schemas/JobDetails"
func extractSchemaNameFromRef(ref string) string {
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
