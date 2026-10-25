package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pointerEscaper escapes a key for a JSON Pointer (RFC 6901) segment.
var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

// TestJSONKeysUnique verifies that no published JSON document repeats a key
// within one object. encoding/json silently keeps the last value, so every other
// check here accepts such a document, but strict parsers such as Jackson reject
// it whole, and OpenAPI client generators built on them fail on the spec.
func TestJSONKeysUnique(t *testing.T) {
	var files []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".json" {
			files = append(files, path)
		}

		return nil
	})
	require.NoError(t, err)
	require.Contains(t, files, filePath)

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			require.NoError(t, err)

			duplicates, err := duplicateKeys(data)
			require.NoError(t, err)
			assert.Empty(t, duplicates, "keys repeated within one object, as JSON Pointers")
		})
	}
}

// duplicateKeys returns the JSON Pointer of every key that repeats within its object.
func duplicateKeys(data []byte) ([]string, error) {
	var duplicates []string
	err := walkJSONValue(json.NewDecoder(bytes.NewReader(data)), "", &duplicates)

	return duplicates, err
}

func walkJSONValue(dec *json.Decoder, pointer string, duplicates *[]string) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}

	switch token {
	case json.Delim('{'):
		seen := make(map[string]bool)
		for dec.More() {
			token, err := dec.Token()
			if err != nil {
				return err
			}

			key, _ := token.(string) // the decoder yields only strings in key position
			keyPointer := pointer + "/" + pointerEscaper.Replace(key)
			if seen[key] {
				*duplicates = append(*duplicates, keyPointer)
			}
			seen[key] = true

			if err := walkJSONValue(dec, keyPointer, duplicates); err != nil {
				return err
			}
		}
	case json.Delim('['):
		for i := 0; dec.More(); i++ {
			if err := walkJSONValue(dec, fmt.Sprintf("%s/%d", pointer, i), duplicates); err != nil {
				return err
			}
		}
	default:
		return nil
	}

	_, err = dec.Token() // the closing delimiter

	return err
}
