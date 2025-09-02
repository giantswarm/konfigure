package renderer

import (
	"reflect"
	"testing"

	"github.com/giantswarm/konfigure/v2/pkg/model"
)

func TestGetLayerOrder(t *testing.T) {
	testCases := []struct {
		name string

		schema *model.Schema

		expected []string
	}{
		{
			name:     "case 0 - empty schema",
			schema:   &model.Schema{},
			expected: nil,
		},
		{
			name: "case 1 - simple schema",
			schema: &model.Schema{
				Layers: []model.Layer{
					{
						Id: "layer-1",
					},
					{
						Id: "layer-2",
					},
					{
						Id: "layer-3",
					},
				},
			},
			expected: []string{"layer-1", "layer-2", "layer-3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetLayerOrder(tc.schema)

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}
