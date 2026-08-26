package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/swaggo/swag"
	"github.com/swaggo/swag/gen"
)

const swagger2OpenAPIPackage = "swagger2openapi@7.0.8"

func generate(workspace string) error {
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(workspace); err != nil {
		return fmt.Errorf("chdir workspace: %w", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	docsDir := filepath.Join(workspace, "docs")
	swaggerPath := filepath.Join(docsDir, "swagger.json")
	openAPIPath := filepath.Join(docsDir, "openapi.json")
	configSchemaPath := filepath.Join(docsDir, "config.schema.json")

	if err := generateSwagger(workspace, docsDir); err != nil {
		return fmt.Errorf("generate swagger: %w", err)
	}
	if err := fixSwaggerFile(swaggerPath); err != nil {
		return err
	}
	if err := convertSwaggerToOpenAPI(swaggerPath, openAPIPath); err != nil {
		return err
	}
	if err := finalizeOpenAPI(openAPIPath, configSchemaPath); err != nil {
		return err
	}

	_ = removeFile(filepath.Join(docsDir, "swagger.yaml"))
	if err := removeFile(swaggerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove swagger.json: %w", err)
	}

	return nil
}

func generateSwagger(workspace, outputDir string) error {
	return gen.New().Build(&gen.Config{
		SearchDir: filepath.Join(workspace, "internal/server/handlers") + "," +
			filepath.Join(workspace, "pkg/dto"),
		MainAPIFile:        "info.go",
		OutputDir:          outputDir,
		OutputTypes:        []string{"go", "json"},
		PropNamingStrategy: swag.CamelCase,
		ParseDepth:         100,
		ParseGoList:        true,
		CollectionFormat:   "csv",
		OverridesFile:      gen.DefaultOverridesFile,
		LeftTemplateDelim:  "{{",
		RightTemplateDelim: "}}",
	})
}

func convertSwaggerToOpenAPI(swaggerPath, openAPIPath string) error {
	if err := runCommand("npx", "--yes", swagger2OpenAPIPackage, swaggerPath, "-o", openAPIPath); err != nil {
		return fmt.Errorf("convert swagger to OpenAPI: %w", err)
	}

	return nil
}

var runCommand = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func fixSwaggerFile(swaggerPath string) error {
	swaggerData, err := readFile(swaggerPath)
	if err != nil {
		return fmt.Errorf("read swagger: %w", err)
	}

	swaggerDoc, err := parseJSONObject(swaggerData)
	if err != nil {
		return fmt.Errorf("parse swagger: %w", err)
	}
	fixInt64Responses(swaggerDoc)

	if err := writeJSONObject(swaggerPath, swaggerDoc, "    "); err != nil {
		return fmt.Errorf("write fixed swagger: %w", err)
	}

	return nil
}

func finalizeOpenAPI(openAPIPath, configSchemaPath string) error {
	openAPIData, err := readFile(openAPIPath)
	if err != nil {
		return fmt.Errorf("read OpenAPI: %w", err)
	}
	openAPIDoc, err := parseJSONObject(openAPIData)
	if err != nil {
		return fmt.Errorf("parse OpenAPI: %w", err)
	}
	FlattenEnumRefs(openAPIDoc)

	if err := writeJSONObject(openAPIPath, openAPIDoc, "    "); err != nil {
		return fmt.Errorf("write OpenAPI: %w", err)
	}

	configSchema, err := newConfigSchema(openAPIDoc)
	if err != nil {
		return err
	}
	if err := writeJSONObject(configSchemaPath, configSchema, "  "); err != nil {
		return fmt.Errorf("write config schema: %w", err)
	}

	return nil
}

func fixInt64Responses(swagger *jsonObject) {
	paths, ok := asJSONObject(swagger.values["paths"])
	if !ok {
		return
	}

	for _, pathName := range paths.keys {
		path, ok := asJSONObject(paths.values[pathName])
		if !ok {
			continue
		}
		for _, method := range path.keys {
			operation, ok := asJSONObject(path.values[method])
			if !ok {
				continue
			}
			responses, ok := asJSONObject(operation.values["responses"])
			if !ok {
				continue
			}
			response, ok := asJSONObject(responses.values["202"])
			if !ok {
				continue
			}
			schema, ok := asJSONObject(response.values["schema"])
			if !ok || schema.values[jsonTypeKey] != "int64" {
				continue
			}

			typeIndex := schema.keyIndex(jsonTypeKey)
			schema.values[jsonTypeKey] = "integer"
			schema.insertKeyAt(typeIndex+1, "format", "int64")
		}
	}
}

func newConfigSchema(openAPI *jsonObject) (*jsonObject, error) {
	components, ok := asJSONObject(openAPI.values["components"])
	if !ok {
		return nil, errors.New("generate config schema: OpenAPI components not found")
	}
	schemas, ok := asJSONObject(components.values["schemas"])
	if !ok {
		return nil, errors.New("generate config schema: OpenAPI schemas not found")
	}
	config, ok := asJSONObject(schemas.values["dto.Config"])
	if !ok {
		return nil, errors.New("generate config schema: dto.Config not found")
	}
	properties, ok := asJSONObject(config.values["properties"])
	if !ok {
		return nil, errors.New("generate config schema: dto.Config properties not found")
	}

	configSchemas := schemas.clone()
	configSchemas.removeKey("dto.Config")

	return &jsonObject{
		keys: []string{"$schema", jsonTypeKey, "properties", "components"},
		values: map[string]any{
			"$schema":    "http://json-schema.org/draft-07/schema#",
			jsonTypeKey:  "object",
			"properties": properties,
			"components": &jsonObject{
				keys: []string{"schemas"},
				values: map[string]any{
					"schemas": configSchemas,
				},
			},
		},
	}, nil
}

func writeJSONObject(path string, doc *jsonObject, indent string) error {
	data, err := marshalJSONObjectIndent(doc, indent)
	if err != nil {
		return err
	}

	return writeFile(path, data)
}
