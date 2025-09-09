package renderer

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/giantswarm/konfigure/v2/pkg/model"
)

func MergeValueFileReferences(schema *model.Schema, layer model.Layer, valueType model.ValueMergeReferenceType, valueFiles ValueFiles) (string, error) {
	valueTypeLower := strings.ToLower(string(valueType))
	configMapTypeLower := strings.ToLower(string(model.ValueMergeReferenceTypeConfigMap))
	secretTypeLower := strings.ToLower(string(model.ValueMergeReferenceTypeSecret))

	var valueFileOptions model.ValueFileOptions
	switch valueTypeLower {
	case configMapTypeLower:
		valueFileOptions = layer.Templates.ConfigMap.Values
	case secretTypeLower:
		valueFileOptions = layer.Templates.Secret.Values
	default:
		return "", errors.Errorf("unknown value reference type %s", valueType)
	}

	var valuesToMerge []string
	var err error

	switch strings.ToLower(valueFileOptions.Merge.Strategy) {
	case strings.ToLower(model.ValueFileMergeStrategyCustomOrder):
		valuesToMerge, err = CustomOrderFilter(valueFileOptions.Merge.Options, valueFiles)
	case strings.ToLower(model.ValueFileMergeStrategySameTypeInLayerOrder):
		if valueTypeLower == configMapTypeLower {
			valuesToMerge, err = ConfigMapsInLayerOrderFilter(schema, valueFiles)
		} else {
			valuesToMerge, err = SecretsInLayerOrderFilter(schema, valueFiles)
		}
	case strings.ToLower(model.ValueFileMergeStrategyConfigMapsInLayerOrder):
		valuesToMerge, err = ConfigMapsInLayerOrderFilter(schema, valueFiles)
	case strings.ToLower(model.ValueFileMergeStrategySecretsInLayerOrder):
		valuesToMerge, err = SecretsInLayerOrderFilter(schema, valueFiles)
	case strings.ToLower(model.ValueFileMergeStrategyConfigMapsAndSecretsInLayerOrder):
		valuesToMerge, err = ConfigMapsAndSecretsInLayerOrderFilter(schema, valueFiles)
	case "":
		fallthrough
	case strings.ToLower(model.ValueFileMergeStrategySameTypeFromCurrentLayer):
		if valueTypeLower == configMapTypeLower {
			valuesToMerge = append(valuesToMerge, valueFiles.ConfigMaps[layer.Id])
		} else {
			valuesToMerge = append(valuesToMerge, valueFiles.Secrets[layer.Id])
		}
	case strings.ToLower(model.ValueFileMergeStrategyConfigMapAndSecretFromCurrentLayer):
		valuesToMerge = append(valuesToMerge, valueFiles.ConfigMaps[layer.Id], valueFiles.Secrets[layer.Id])
	default:
		return "", errors.Errorf("unknown value merge strategy %q", valueFileOptions.Merge.Strategy)
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
		switch strings.ToLower(string(valueMergeReference.Type)) {
		case strings.ToLower(string(model.ValueMergeReferenceTypeConfigMap)):
			valuesToMerge = append(valuesToMerge, valueFiles.ConfigMaps[valueMergeReference.LayerId])
		case strings.ToLower(string(model.ValueMergeReferenceTypeSecret)):
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
