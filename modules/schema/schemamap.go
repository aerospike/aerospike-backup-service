package schema

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// this file is based on copy from asconfig
// https://github.com/aerospike/asconfig/blob/main/schema/schemamap.go

type SchemaMap map[string]string

var schemaMap SchemaMap

func init() {
	var err error
	schemaMap, err = NewSchemaMap()
	if err != nil {
		panic(fmt.Errorf("error initialising schema: %v", err))
	}
}

func GetSchemas() SchemaMap {
	return schemaMap
}

func NewSchemaMap() (SchemaMap, error) {
	schema := make(SchemaMap)

	if err := fs.WalkDir(
		schemas, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() {
				content, err := fs.ReadFile(schemas, path)
				if err != nil {
					return err
				}

				baseName := filepath.Base(path)
				key := strings.TrimSuffix(baseName, filepath.Ext(baseName))
				schema[key] = string(content)
			}

			return nil
		},
	); err != nil {
		return nil, err
	}

	return schema, nil
}
