package renderer

import "github.com/giantswarm/konfigure/v2/pkg/model"

func GetLayerOrder(schema *model.Schema) []string {
	var result []string

	for _, layer := range schema.Layers {
		result = append(result, layer.Id)
	}

	return result
}
