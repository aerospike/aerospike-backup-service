package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlattenEnumRefs(t *testing.T) {
	doc, err := parseJSONObject([]byte(`{
		"components": {
			"schemas": {
				"dto.CompressionMode": {
					"type": "string",
					"enum": ["NONE", "ZSTD"]
				},
				"dto.CompressionPolicy": {
					"type": "object",
					"properties": {
						"mode": {
							"description": "compression mode",
							"default": "NONE",
							"allOf": [{ "$ref": "#/components/schemas/dto.CompressionMode" }]
						},
						"level": {
							"type": "integer",
							"default": 0
						}
					}
				},
				"dto.S3StorageClass": {
					"type": "object",
					"properties": {
						"data": {
							"nullable": true,
							"allOf": [{ "$ref": "#/components/schemas/dto.S3DataClass" }]
						}
					}
				},
				"dto.S3DataClass": {
					"type": "string",
					"enum": ["STANDARD", "GLACIER"]
				},
				"dto.BackupPolicy": {
					"type": "object",
					"properties": {
						"compression": {
							"allOf": [{ "$ref": "#/components/schemas/dto.CompressionPolicy" }]
						}
					}
				}
			}
		}
	}`))
	require.NoError(t, err)
	FlattenEnumRefs(doc)

	components, ok := asJSONObject(doc.values["components"])
	require.True(t, ok)
	schemas, ok := asJSONObject(components.values["schemas"])
	require.True(t, ok)

	policy, ok := asJSONObject(schemas.values["dto.CompressionPolicy"])
	require.True(t, ok)
	properties, ok := asJSONObject(policy.values["properties"])
	require.True(t, ok)
	mode, ok := asJSONObject(properties.values["mode"])
	require.True(t, ok)

	assert.Equal(t, openAPIStringType, mode.values["type"])
	assert.Equal(t, []any{"NONE", "ZSTD"}, mode.values["enum"])
	assert.Equal(t, "NONE", mode.values["default"])
	assert.Equal(t, "compression mode", mode.values["description"])
	assert.Equal(t, []string{"description", "default", "type", "enum"}, mode.keys)
	assert.NotContains(t, mode.values, "allOf")

	storageClass, ok := asJSONObject(schemas.values["dto.S3StorageClass"])
	require.True(t, ok)
	storageProps, ok := asJSONObject(storageClass.values["properties"])
	require.True(t, ok)
	data, ok := asJSONObject(storageProps.values["data"])
	require.True(t, ok)
	assert.Equal(t, true, data.values["nullable"])
	assert.Equal(t, []any{"STANDARD", "GLACIER"}, data.values["enum"])
	assert.Equal(t, []string{"nullable", "type", "enum"}, data.keys)

	_, hasCompressionMode := schemas.values["dto.CompressionMode"]
	assert.False(t, hasCompressionMode)
	_, hasS3DataClass := schemas.values["dto.S3DataClass"]
	assert.False(t, hasS3DataClass)

	backupPolicy, ok := asJSONObject(schemas.values["dto.BackupPolicy"])
	require.True(t, ok)
	backupProps, ok := asJSONObject(backupPolicy.values["properties"])
	require.True(t, ok)
	compression, ok := asJSONObject(backupProps.values["compression"])
	require.True(t, ok)
	require.Contains(t, compression.values, "allOf")
}

func TestFlattenEnumRefs_realOpenAPIFixture(t *testing.T) {
	const fixture = `{
	  "components": {
	    "schemas": {
	      "dto.LoggerConfig": {
	        "type": "object",
	        "properties": {
	          "level": {
	            "default": "INFO",
	            "allOf": [{ "$ref": "#/components/schemas/dto.LogLevel" }]
	          }
	        }
	      },
	      "dto.LogLevel": {
	        "type": "string",
	        "enum": ["TRACE", "INFO", "ERROR"]
	      }
	    }
	  }
	}`

	doc, err := parseJSONObject([]byte(fixture))
	require.NoError(t, err)
	FlattenEnumRefs(doc)

	components, ok := asJSONObject(doc.values["components"])
	require.True(t, ok)
	schemas, ok := asJSONObject(components.values["schemas"])
	require.True(t, ok)
	loggerConfig, ok := asJSONObject(schemas.values["dto.LoggerConfig"])
	require.True(t, ok)
	loggerProps, ok := asJSONObject(loggerConfig.values["properties"])
	require.True(t, ok)
	level, ok := asJSONObject(loggerProps.values["level"])
	require.True(t, ok)
	assert.Equal(t, []any{"TRACE", "INFO", "ERROR"}, level.values["enum"])
	assert.Equal(t, []string{"default", "type", "enum"}, level.keys)
	_, hasLogLevelSchema := schemas.values["dto.LogLevel"]
	assert.False(t, hasLogLevelSchema)
}

func TestFlattenEnumRefs_preservesSchemaKeyOrder(t *testing.T) {
	const fixture = `{
	  "components": {
	    "schemas": {
	      "dto.Zebra": { "type": "string", "enum": ["Z"] },
	      "dto.Alpha": {
	        "type": "object",
	        "properties": {
	          "mode": {
	            "description": "mode",
	            "allOf": [{ "$ref": "#/components/schemas/dto.Zebra" }]
	          }
	        }
	      }
	    }
	  }
	}`

	doc, err := parseJSONObject([]byte(fixture))
	require.NoError(t, err)
	FlattenEnumRefs(doc)

	components, ok := asJSONObject(doc.values["components"])
	require.True(t, ok)
	schemas, ok := asJSONObject(components.values["schemas"])
	require.True(t, ok)
	assert.Equal(t, []string{"dto.Alpha"}, schemas.keys)

	alpha, ok := asJSONObject(schemas.values["dto.Alpha"])
	require.True(t, ok)
	alphaProps, ok := asJSONObject(alpha.values["properties"])
	require.True(t, ok)
	mode, ok := asJSONObject(alphaProps.values["mode"])
	require.True(t, ok)
	assert.Equal(t, []string{"description", "type", "enum"}, mode.keys)
}

func TestOrderedJSONPreservesTopLevelKeyOrder(t *testing.T) {
	const fixture = `{"z": 1, "a": {"y": 2, "b": 3}}`

	doc, err := parseJSONObject([]byte(fixture))
	require.NoError(t, err)
	assert.Equal(t, []string{"z", "a"}, doc.keys)

	nested, ok := asJSONObject(doc.values["a"])
	require.True(t, ok)
	assert.Equal(t, []string{"y", "b"}, nested.keys)

	encoded, err := marshalJSONObjectIndent(doc, "    ")
	require.NoError(t, err)
	assert.JSONEq(t, "{\n    \"z\": 1,\n    \"a\": {\n        \"y\": 2,\n        \"b\": 3\n    }\n}\n", string(encoded))
}

func TestMarshalJSONValue_doesNotEscapeHTMLInStrings(t *testing.T) {
	doc, err := parseJSONObject([]byte(`{"description": "use <start>-<count> and <IP address>:<port>"}`))
	require.NoError(t, err)

	encoded, err := marshalJSONValue(doc)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "<start>-<count>")
	assert.Contains(t, string(encoded), "<IP address>:<port>")
	assert.NotContains(t, string(encoded), `\u003c`)
}

func TestOrderedJSONPreservesLargeIntegerPrecision(t *testing.T) {
	doc, err := parseJSONObject([]byte(`{"example": 1000000000000000008}`))
	require.NoError(t, err)

	encoded, err := marshalJSONValue(doc)
	require.NoError(t, err)
	assert.JSONEq(t, `{"example": 1000000000000000008}`, string(encoded))
	assert.Contains(t, string(encoded), "1000000000000000008")
}
