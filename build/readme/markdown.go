package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const openapi = "docs/openapi.json"
const docFolder = "docs/readme/dto"

var schemas = readSchemas()

func generateMarkdownFiles() {
	_ = os.RemoveAll(docFolder)
	_ = os.MkdirAll(docFolder, 0755)
	for dtoName := range schemas {
		fileName := filepath.Join(docFolder, strings.ToLower(dtoName)+".md")
		fileContent := generateMarkdownTable(dtoName)

		err := os.WriteFile(fileName, []byte(fileContent), 0600)
		if err != nil {
			panic(fmt.Errorf("failed to write markdown file %q: %w", fileName, err))
		}
		fmt.Printf("Generated %s\n", fileName)
	}
}

// Row represents a single row in the Markdown table.
type Row struct {
	Name           string
	Help           string
	Default        string // New field for default value
	PossibleValues string // New field for possible enum values
}

func generateMarkdownTable(dtoName string) string {
	schema, ok := schemas[dtoName]
	if !ok {
		panic(fmt.Errorf("schema %q not found", dtoName))
	}

	rows := schemaToRows(schema)

	// Determine if 'Default Value' or 'Possible Values' columns are needed
	hasDefaultColumn, hasPossibleValuesColumn := hasOptionalColumns(rows)

	maxName, maxHelp, maxDefault, maxPossibleValues := determineColumsWidth(rows)

	const quotes = 0
	var sb strings.Builder

	// Add header for the DTO
	sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", dtoName, schema.Description))

	// Write header
	sb.WriteString(fmt.Sprintf("| %-*s | %-*s ", maxName+quotes, "Field", maxHelp, "Description"))
	if hasDefaultColumn {
		sb.WriteString(fmt.Sprintf("| %-*s ", maxDefault, "Default Value"))
	}
	if hasPossibleValuesColumn {
		sb.WriteString(fmt.Sprintf("| %-*s ", maxPossibleValues, "Possible Values"))
	}
	sb.WriteString("|\n")

	// Write separator
	sb.WriteString(fmt.Sprintf("|-%s-|-%s-", strings.Repeat("-", maxName+quotes), strings.Repeat("-", maxHelp)))
	if hasDefaultColumn {
		sb.WriteString(fmt.Sprintf("|-%s-", strings.Repeat("-", maxDefault)))
	}
	if hasPossibleValuesColumn {
		sb.WriteString(fmt.Sprintf("|-%s-", strings.Repeat("-", maxPossibleValues)))
	}
	sb.WriteString("|\n")

	// Write rows
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("| %-*s | %-*s ", maxName+quotes, r.Name, maxHelp, r.Help))
		if hasDefaultColumn {
			sb.WriteString(fmt.Sprintf("| %-*s ", maxDefault, r.Default))
		}
		if hasPossibleValuesColumn {
			sb.WriteString(fmt.Sprintf("| %-*s ", maxPossibleValues, r.PossibleValues))
		}
		sb.WriteString("|\n")
	}

	if len(schema.Required) > 0 {
		sb.WriteString("\n🔴 = Required field")
	}

	return sb.String()
}

func hasOptionalColumns(rows []Row) (bool, bool) {
	hasDefaultColumn := false
	hasPossibleValuesColumn := false
	for _, r := range rows {
		if r.Default != "" {
			hasDefaultColumn = true
		}
		if r.PossibleValues != "" {
			hasPossibleValuesColumn = true
		}
		if hasDefaultColumn && hasPossibleValuesColumn {
			break // Both columns are needed, no need to check further
		}
	}

	return hasDefaultColumn, hasPossibleValuesColumn
}

func determineColumsWidth(rows []Row) (int, int, int, int) {
	maxName := len("Field")
	maxHelp := len("Description")
	maxDefault := len("Default Value")
	maxPossibleValues := len("Possible Values")

	for _, r := range rows {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		for _, line := range strings.Split(r.Help, "\n") {
			if len(line) > maxHelp {
				maxHelp = len(line)
			}
		}
		if len(r.Default) > maxDefault {
			maxDefault = len(r.Default)
		}
		if len(r.PossibleValues) > maxPossibleValues {
			maxPossibleValues = len(r.PossibleValues)
		}
	}

	return maxName, maxHelp, maxDefault, maxPossibleValues
}
func readSchemas() map[string]Schema {
	data, err := os.ReadFile(openapi)
	if err != nil {
		panic(fmt.Errorf("failed to read file: %w", err))
	}

	var api OpenAPI
	if err := json.Unmarshal(data, &api); err != nil {
		panic(fmt.Errorf("failed to unmarshal JSON: %w", err))
	}

	return api.Components.Schemas
}

type OpenAPI struct {
	Components Components `json:"components"`
}

type Components struct {
	Schemas map[string]Schema `json:"schemas"`
}

type Schema struct {
	Description string              `json:"description"`
	Type        string              `json:"type"`
	Properties  map[string]Property `json:"properties"`
	Required    []string            `json:"required"`
}

type Property struct {
	Description string      `json:"description,omitempty"`
	Type        string      `json:"type,omitempty"`
	AllOf       []Reference `json:"allOf"` //nolint:tagliatelle
	Items       Reference   `json:"items"`
	Enum        []string    `json:"enum"`
	Default     any         `json:"default,omitempty"`
}

type Reference struct {
	Ref string `json:"$ref,omitempty"`
}

// schemaToRows generates rows for a single schema, adding links for referenced objects and marking required fields.
func schemaToRows(input Schema) []Row {
	var rows = make([]Row, 0, len(input.Properties))

	// Create a map for quick lookup of required fields
	requiredFields := make(map[string]bool)
	for _, req := range input.Required {
		requiredFields[req] = true
	}

	// Collect field names and create initial rows
	var fieldProps = make([]FieldProperty, 0, len(input.Properties))

	for fieldName, prop := range input.Properties {
		fieldProps = append(fieldProps, FieldProperty{Name: fieldName, Prop: prop})
	}

	// Sort fields: required first, then alphabetically
	sort.Slice(fieldProps, func(i, j int) bool {
		isReqI := requiredFields[fieldProps[i].Name]
		isReqJ := requiredFields[fieldProps[j].Name]

		if isReqI != isReqJ {
			return isReqI // True (required) comes before False (not required)
		}
		return strings.Compare(fieldProps[i].Name, fieldProps[j].Name) < 0
	})

	for _, fp := range fieldProps {
		rows = append(rows, makeRow(fp, requiredFields))
	}

	return rows
}

type FieldProperty struct {
	Name string
	Prop Property
}

func makeRow(fp FieldProperty, requiredFields map[string]bool) Row {
	fieldName := fp.Name
	prop := fp.Prop
	description := strings.ReplaceAll(prop.Description, "\n", "<br>")
	defaultValue := ""
	possibleValues := ""

	// Check if it's a reference via allOf first, as this is the primary indicator for linked objects
	if len(prop.AllOf) > 0 {
		for _, ref := range prop.AllOf {
			if ref.Ref != "" {
				refName := extractRefName(ref.Ref)
				linkedFileName := strings.ToLower(refName) + ".md"
				description = fmt.Sprintf("%s<br>See: [%s](%s)", description, refName, linkedFileName)
				break // Assume only one reference in allOf for simplicity
			}
		}
	} else if prop.Type == "array" && prop.Items.Ref != "" {
		// Handle arrays of referenced objects
		refName := extractRefName(prop.Items.Ref)
		linkedFileName := strings.ToLower(refName) + ".md"
		description = fmt.Sprintf("%s<br>Array of: [%s](%s)", description, refName, linkedFileName)
	}

	defaultValue = formatValue(prop.Default)

	// Handle enum values
	if len(prop.Enum) > 0 {
		possibleValues = "`" + strings.Join(prop.Enum, "`, `") + "`"
	}

	nameWithAsterisk := "`" + fieldName + "`"
	if requiredFields[fieldName] {
		// Add a red asterisk using HTML span
		nameWithAsterisk = fmt.Sprintf("🔴 %s", nameWithAsterisk)
	}

	row := Row{
		Name:           nameWithAsterisk,
		Help:           description,
		Default:        defaultValue,
		PossibleValues: possibleValues,
	}

	return row
}

// Example: "#/components/schemas/dto.RunningJob" → "dto.RunningJob".
func extractRefName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func formatValue(x any) string {
	if x == nil {
		return ""
	}
	switch v := x.(type) {
	case float32:
		return fmt.Sprintf("`%g`", v)
	case float64:
		return fmt.Sprintf("`%g`", v)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("`%d`", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("`%d`", v)
	case string:
		return fmt.Sprintf("`%s`", v)
	case bool:
		return fmt.Sprintf("`%t`", v)
	default:
		return fmt.Sprintf("`%v`", v)
	}
}
