package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixSwaggerFile(t *testing.T) {
	const swagger = `{
		"swagger": "2.0",
		"info": {"title": "test", "version": "1"},
		"paths": {
			"/restore": {
				"post": {
					"responses": {
						"202": {
							"description": "accepted",
							"schema": {"type": "int64"}
						}
					}
				}
			}
		}
	}`

	dir := t.TempDir()
	swaggerPath := filepath.Join(dir, "swagger.json")
	require.NoError(t, os.WriteFile(swaggerPath, []byte(swagger), 0600))

	require.NoError(t, fixSwaggerFile(swaggerPath))

	swaggerData, err := os.ReadFile(swaggerPath)
	require.NoError(t, err)
	doc, err := parseJSONObject(swaggerData)
	require.NoError(t, err)
	paths, ok := asJSONObject(doc.values["paths"])
	require.True(t, ok)
	restore, ok := asJSONObject(paths.values["/restore"])
	require.True(t, ok)
	post, ok := asJSONObject(restore.values["post"])
	require.True(t, ok)
	responses, ok := asJSONObject(post.values["responses"])
	require.True(t, ok)
	accepted, ok := asJSONObject(responses.values["202"])
	require.True(t, ok)
	schema, ok := asJSONObject(accepted.values["schema"])
	require.True(t, ok)
	assert.Equal(t, "integer", schema.values["type"])
	assert.Equal(t, "int64", schema.values["format"])
	assert.Equal(t, []string{"type", "format"}, schema.keys)
}

func TestFinalizeOpenAPI(t *testing.T) {
	const openAPI = `{
		"openapi": "3.0.0",
		"components": {
			"schemas": {
				"dto.Config": {
					"type": "object",
					"properties": {
						"mode": {
							"allOf": [{"$ref": "#/components/schemas/dto.Mode"}]
						}
					}
				},
				"dto.Mode": {
					"type": "string",
					"enum": ["ONE", "TWO"]
				}
			}
		}
	}`

	dir := t.TempDir()
	openAPIPath := filepath.Join(dir, "openapi.json")
	configSchemaPath := filepath.Join(dir, "config.schema.json")
	require.NoError(t, os.WriteFile(openAPIPath, []byte(openAPI), 0600))

	require.NoError(t, finalizeOpenAPI(openAPIPath, configSchemaPath))

	configSchemaData, err := os.ReadFile(configSchemaPath)
	require.NoError(t, err)
	configSchema, err := parseJSONObject(configSchemaData)
	require.NoError(t, err)
	assert.Equal(t, "http://json-schema.org/draft-07/schema#", configSchema.values["$schema"])
	assert.Equal(t, "object", configSchema.values["type"])

	properties, ok := asJSONObject(configSchema.values["properties"])
	require.True(t, ok)
	mode, ok := asJSONObject(properties.values["mode"])
	require.True(t, ok)
	assert.Equal(t, []any{"ONE", "TWO"}, mode.values["enum"])
}

func TestConvertSwaggerToOpenAPI_usesPinnedConverter(t *testing.T) {
	orig := runCommand
	t.Cleanup(func() { runCommand = orig })

	var gotName string
	var gotArgs []string
	runCommand = func(name string, args ...string) error {
		gotName = name
		gotArgs = args
		return nil
	}

	require.NoError(t, convertSwaggerToOpenAPI("swagger.json", "openapi.json"))
	assert.Equal(t, "npx", gotName)
	assert.Equal(t, []string{"--yes", swagger2OpenAPIPackage, "swagger.json", "-o", "openapi.json"}, gotArgs)
}

func TestFixInt64Responses_ignoresOtherSchemas(t *testing.T) {
	doc, err := parseJSONObject([]byte(`{
		"paths": {
			"/restore": {
				"post": {
					"responses": {
						"200": {"schema": {"type": "int64"}},
						"202": {"schema": {"type": "string"}}
					}
				}
			}
		}
	}`))
	require.NoError(t, err)

	fixInt64Responses(doc)

	encoded, err := marshalJSONValue(doc)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"paths": {
			"/restore": {
				"post": {
					"responses": {
						"200": {"schema": {"type": "int64"}},
						"202": {"schema": {"type": "string"}}
					}
				}
			}
		}
	}`, string(encoded))
}
