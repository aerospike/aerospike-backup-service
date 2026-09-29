package main

import (
	"strings"
)

// FlattenEnumRefs inlines enum-only component schemas referenced via a single allOf $ref.
func FlattenEnumRefs(doc *jsonObject) {
	components, ok := asJSONObject(doc.values["components"])
	if !ok {
		return
	}

	schemas, ok := asJSONObject(components.values["schemas"])
	if !ok {
		return
	}

	enumOnly := enumOnlySchemas(schemas)
	for _, key := range schemas.keys {
		schema, ok := asJSONObject(schemas.values[key])
		if !ok {
			continue
		}
		flattenProperties(schema, schemas, enumOnly)
	}

	for name := range enumOnly {
		schemas.removeKey(name)
	}
}

func enumOnlySchemas(schemas *jsonObject) map[string]bool {
	enumOnly := make(map[string]bool)
	for _, name := range schemas.keys {
		schema, ok := asJSONObject(schemas.values[name])
		if !ok {
			continue
		}
		if isEnumOnlySchema(schema) {
			enumOnly[name] = true
		}
	}

	return enumOnly
}

func isEnumOnlySchema(schema *jsonObject) bool {
	_, hasEnum := schema.values["enum"]
	_, hasProps := schema.values["properties"]
	return hasEnum && !hasProps
}

func flattenProperties(schema *jsonObject, schemas *jsonObject, enumOnly map[string]bool) {
	properties, ok := asJSONObject(schema.values["properties"])
	if !ok {
		return
	}

	for _, key := range properties.keys {
		prop, ok := asJSONObject(properties.values[key])
		if !ok {
			continue
		}
		inlineEnumRef(prop, schemas, enumOnly)
	}
}

func inlineEnumRef(prop *jsonObject, schemas *jsonObject, enumOnly map[string]bool) bool {
	allOf, ok := prop.values["allOf"].([]any)
	if !ok || len(allOf) != 1 {
		return false
	}

	refObj, ok := asJSONObject(allOf[0])
	if !ok {
		return false
	}

	ref, ok := refObj.values["$ref"].(string)
	if !ok {
		return false
	}

	schemaName := extractRefName(ref)
	if !enumOnly[schemaName] {
		return false
	}

	target, ok := asJSONObject(schemas.values[schemaName])
	if !ok {
		return false
	}

	enum, ok := target.values["enum"].([]any)
	if !ok || len(enum) == 0 {
		return false
	}

	allOfIndex := prop.keyIndex("allOf")
	if allOfIndex < 0 {
		return false
	}

	prop.removeKey("allOf")
	prop.insertKeyAt(allOfIndex, jsonTypeKey, schemaType(target))
	prop.insertKeyAt(allOfIndex+1, "enum", enum)

	return true
}

func schemaType(schema *jsonObject) string {
	schemaType, ok := schema.values[jsonTypeKey].(string)
	if !ok || schemaType == "" {
		return openAPIStringType
	}
	return schemaType
}

const (
	jsonTypeKey       = "type"
	openAPIStringType = "string"
)

func extractRefName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}
