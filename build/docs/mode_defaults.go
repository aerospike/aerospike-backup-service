package main

const (
	// backupModeExtension marks a field that only one backup-mode accepts. The other mode rejects
	// the request or configuration that sets it.
	backupModeExtension = "x-backup-mode"
	// modeDefaultExtension carries the default of a field marked with backupModeExtension.
	modeDefaultExtension = "x-default"
)

// ScopeModeDefaults moves the default of every field that only one backup-mode accepts from
// `default` to `x-default`. Clients generated from the spec send every field that has a
// `default`, so the other mode would reject each request they build. Code generators ignore
// `x-` keys, while the default stays machine-readable for the docs.
func ScopeModeDefaults(doc *jsonObject) {
	for _, schema := range componentSchemas(doc) {
		properties, ok := asJSONObject(schema.values["properties"])
		if !ok {
			continue
		}
		for _, key := range properties.keys {
			prop, ok := asJSONObject(properties.values[key])
			if !ok {
				continue
			}
			scopeDefault(prop)
		}
	}
}

func scopeDefault(prop *jsonObject) {
	if _, scoped := prop.values[backupModeExtension]; !scoped {
		return
	}
	index := prop.keyIndex("default")
	if index < 0 {
		return
	}

	value := prop.values["default"]
	prop.removeKey("default")
	prop.insertKeyAt(index, modeDefaultExtension, value)
}

func componentSchemas(doc *jsonObject) []*jsonObject {
	components, ok := asJSONObject(doc.values["components"])
	if !ok {
		return nil
	}
	schemas, ok := asJSONObject(components.values["schemas"])
	if !ok {
		return nil
	}

	result := make([]*jsonObject, 0, len(schemas.keys))
	for _, key := range schemas.keys {
		if schema, ok := asJSONObject(schemas.values[key]); ok {
			result = append(result, schema)
		}
	}

	return result
}
