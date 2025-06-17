package main

import (
	"encoding/json"
	"fmt"
	"os"
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
