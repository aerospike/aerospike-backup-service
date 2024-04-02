package schema

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
)

// this file is based on copy from asconfig
// https://github.com/aerospike/asconfig/blob/main/schema/schemamap.go

type Schemas map[string]string

func init() {
	schemaMap, err := NewSchemaMap()
	if err != nil {
		panic(fmt.Errorf("error initialising schema: %v", err))
	}
	asconfig.InitFromMap(logr.Discard(), schemaMap)
}

func NewSchemaMap() (Schemas, error) {
	schema := make(Schemas)

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
