package renderer

import (
	"github.com/pkg/errors"

	"github.com/giantswarm/konfigure/pkg/model"
)

func MergeValueFileReferences(schema *model.Schema, valueMergeOptions model.ValueFileOptions, valueFiles ValueFiles) (string, error) {
	var valuesToMerge []string
	var err error

	switch valueMergeOptions.Merge.Strategy {
	case model.ValueFileMergeStrategyCustomOrder:
		valuesToMerge, err = CustomOrderFilter(valueMergeOptions.Merge.Options, valueFiles)
	case model.ValueFileMergeStrategyConfigMapsInLayerOrder:
		valuesToMerge, err = ConfigMapsInLayerOrderFilter(schema, valueFiles)
	case model.ValueFileMergeStrategySecretsInLayerOrder:
		valuesToMerge, err = SecretsInLayerOrderFilter(schema, valueFiles)
	case model.ValueFileMergeStrategyConfigMapsAndSecretsInLayerOrder:
		valuesToMerge, err = ConfigMapsAndSecretsInLayerOrderFilter(schema, valueFiles)
	default:
		return "", errors.Errorf("unknown value merge strategy %q", valueMergeOptions.Merge.Strategy)
	}

	if err != nil {
		return "", err
	}

	return MergeYamlDocuments(valuesToMerge)
}

func CustomOrderFilter(rawOptions model.RawMessage, valueFiles ValueFiles) ([]string, error) {
	options := model.CustomOrderValueMergeStrategyOptions{}
	err := rawOptions.Unmarshal(&options)
	if err != nil {
		return nil, err
	}

	valuesToMerge := make([]string, 0)

	for _, valueMergeReference := range options.Order {
		switch valueMergeReference.Type {
		case model.ValueMergeReferenceTypeConfigMap:
			valuesToMerge = append(valuesToMerge, valueFiles.ConfigMaps[valueMergeReference.LayerId])
		case model.ValueMergeReferenceTypeSecret:
			valuesToMerge = append(valuesToMerge, valueFiles.Secrets[valueMergeReference.LayerId])
		default:
			return []string{}, errors.Errorf("unknown value merge reference type %s", valueMergeReference.Type)
		}
	}

	return valuesToMerge, nil
}

func ConfigMapsInLayerOrderFilter(schema *model.Schema, valueFiles ValueFiles) ([]string, error) {
	order := GetLayerOrder(schema)

	valuesToMerge := make([]string, 0)

	for _, layer := range order {
		valuesToMerge = append(valuesToMerge, valueFiles.ConfigMaps[layer])
	}

	return valuesToMerge, nil
}

func SecretsInLayerOrderFilter(schema *model.Schema, valueFiles ValueFiles) ([]string, error) {
	order := GetLayerOrder(schema)

	valuesToMerge := make([]string, 0)

	for _, layer := range order {
		valuesToMerge = append(valuesToMerge, valueFiles.Secrets[layer])
	}

	return valuesToMerge, nil
}

func ConfigMapsAndSecretsInLayerOrderFilter(schema *model.Schema, valueFiles ValueFiles) ([]string, error) {
	configMapsInOrder, err := ConfigMapsInLayerOrderFilter(schema, valueFiles)
	if err != nil {
		return []string{}, nil
	}

	secretsInOrder, err := SecretsInLayerOrderFilter(schema, valueFiles)
	if err != nil {
		return []string{}, nil
	}

	return append(configMapsInOrder, secretsInOrder...), nil
}
