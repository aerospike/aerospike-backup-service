package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlattenEnumRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		prop string
		want string
	}{
		{
			name: "ref only",
			prop: `{"default":"1.2","allOf":[{"$ref":"#/components/schemas/dto.Version"}]}`,
			want: `{"default":"1.2","type":"string","enum":["1.2","1.3"]}`,
		},
		{
			name: "ref and property enum",
			prop: `{"default":"1.2","enum":["1.2"],"allOf":[{"$ref":"#/components/schemas/dto.Version"}]}`,
			want: `{"default":"1.2","type":"string","enum":["1.2","1.3"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc, err := parseJSONObject([]byte(`{"components":{"schemas":{` +
				`"dto.Version":{"type":"string","enum":["1.2","1.3"]},` +
				`"dto.Server":{"properties":{"version":` + tt.prop + `}}}}}`))
			require.NoError(t, err)

			FlattenEnumRefs(doc)

			got, err := marshalJSONValue(doc)
			require.NoError(t, err)
			assert.Equal(t, `{"components":{"schemas":{"dto.Server":{"properties":{"version":`+tt.want+`}}}}}`,
				string(got))
		})
	}
}
