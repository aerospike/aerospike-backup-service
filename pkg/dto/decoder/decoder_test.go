package decoder

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type TestStruct struct {
	ID       int    `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	NewField string `json:"new-field" yaml:"new-field"`
}

func setupTest(t *testing.T) {
	t.Helper()

	// Save original values
	origDeprecatedFields := deprecatedFields
	origFieldsByStruct := fieldsByStruct
	origAllFields := allFields

	// Set test values
	deprecatedFields = map[string]map[string]string{
		"decoder.TestStruct": {
			"old-field": "is deprecated. Use new-field instead",
		},
	}

	fieldsByStruct = map[string][]string{
		"decoder.TestStruct": {"id", "name", "new-field"},
	}

	allFields = []string{"id", "name", "new-field"}

	// Cleanup function to restore original values after test
	t.Cleanup(func() {
		deprecatedFields = origDeprecatedFields
		fieldsByStruct = origFieldsByStruct
		allFields = origAllFields
	})
}

func TestDeserialize_JSONSuccess(t *testing.T) {
	setupTest(t)

	jsonStr := `{"id": 1, "name": "test", "new-field": "value"}`

	var result TestStruct
	err := Deserialize(&result, strings.NewReader(jsonStr), JSON)

	require.NoError(t, err)
	assert.Equal(t, 1, result.ID)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, "value", result.NewField)
}

func TestDeserialize_YAMLSuccess(t *testing.T) {
	setupTest(t)

	yamlStr := `
id: 1
name: test
new-field: value
`
	var result TestStruct
	err := Deserialize(&result, strings.NewReader(yamlStr), YAML)

	require.NoError(t, err)
	assert.Equal(t, 1, result.ID)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, "value", result.NewField)
}

func TestDeserialize_JSONDeprecatedField(t *testing.T) {
	setupTest(t)

	jsonStr := `{"id": 1, "name": "test", "old-field": "value"}`

	var result TestStruct
	err := Deserialize(&result, strings.NewReader(jsonStr), JSON)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "field \"old-field\" is deprecated")
}

func TestDeserialize_YAMLDeprecatedField(t *testing.T) {
	setupTest(t)

	yamlStr := `
id: 1
name: test
old-field: value
`

	var result TestStruct
	err := Deserialize(&result, strings.NewReader(yamlStr), YAML)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "line 4: field \"old-field\" is deprecated")
}

func TestDeserialize_JSONSimilarField(t *testing.T) {
	setupTest(t)

	jsonStr := `{"id": 1, "name": "test", "new-feild": "value"}`

	var result TestStruct
	err := Deserialize(&result, strings.NewReader(jsonStr), JSON)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "new-field")
}

func TestDeserialize_YAMLSimilarField(t *testing.T) {
	setupTest(t)

	yamlStr := `
id: 1
name: test
new-feild: value
`

	var result TestStruct
	err := Deserialize(&result, strings.NewReader(yamlStr), YAML)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "new-field")
}

func TestDeserialize_JSONExtraFields(t *testing.T) {
	setupTest(t)

	jsonStr := `{"id": 1, "name": "test", "new-field": "value", "extra": "field"}`

	var result TestStruct
	err := Deserialize(&result, strings.NewReader(jsonStr), JSON)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field \"extra\"")
}

func TestDeserialize_YAMLExtraFields(t *testing.T) {
	setupTest(t)

	yamlStr := `
id: 1
name: test
new-field: value
extra: field
`
	var result TestStruct
	err := Deserialize(&result, strings.NewReader(yamlStr), YAML)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "field extra not found")
}

func TestEnhanceJSONError_UnparseableError(t *testing.T) {
	setupTest(t)

	// Mock a situation where the JSON error can't be parsed
	mockError := errors.New("some unexpected json error format")

	err := enhanceJSONError(mockError)

	// Should return the original error when it can't be parsed
	assert.Equal(t, mockError, err)
}

func TestEnhanceYamlErrors_UnparseableError(t *testing.T) {
	setupTest(t)

	// Mock a situation where the YAML error can't be parsed
	mockYamlError :=
		&yaml.TypeError{
			Errors: []string{
				"unexpected format that doesn't match the regex",
			},
		}

	err := enhanceYamlErrors(mockYamlError)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse error message")
}

func TestDeserialize_InvalidJSONSyntax(t *testing.T) {
	setupTest(t)

	jsonStr := `{"id": 1, "name": "test", "new-field": `

	var result TestStruct
	err := Deserialize(&result, strings.NewReader(jsonStr), JSON)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected EOF")
}

func TestDeserialize_InvalidYAMLSyntax(t *testing.T) {
	setupTest(t)

	yamlStr := `
id: 1
name: test
new-field: 'unclosed quote
`
	var result TestStruct
	err := Deserialize(&result, strings.NewReader(yamlStr), YAML)

	require.Error(t, err)
}

func TestDeserialize_UnsupportedFormat(t *testing.T) {
	setupTest(t)

	var result TestStruct

	err := Deserialize(&result, strings.NewReader(""), SerializationFormat(999))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestDeserialize_NilReader(t *testing.T) {
	setupTest(t)

	var result TestStruct

	err := Deserialize(&result, nil, JSON)

	require.Error(t, err)
}

func TestDeserialize_NilTarget(t *testing.T) {
	setupTest(t)

	jsonStr := `{"id": 1, "name": "test"}`

	err := Deserialize(nil, strings.NewReader(jsonStr), YAML)

	require.Error(t, err)
}
